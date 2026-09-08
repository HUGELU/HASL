'use strict';
let evolutionData=null,learningData=null,evolutionBusy=false,evolutionLast=0,evolutionLoaded=false,evolutionLog='',evolutionPreviewURLs=[];
window.renderEvolution=async function(force=false){
 if(evolutionBusy||(!force&&Date.now()-evolutionLast<2500))return;
 evolutionBusy=true;evolutionLast=Date.now();
 try{
  [learningData,evolutionData]=await Promise.all([api('/api/learning/state',{}),api('/api/evolution/state',{})]);
  refreshAssetSelect('teach-asset',()=>true);
  const l=learningData,rows=l.examples||[],groups={};
  for(const x of rows){groups[x.label]??={train:new Set(),validation:new Set(),audit:new Set()};groups[x.label][x.split].add(x.group)}
  $('learning-note').textContent=l.note;
  $('recognition-metrics').innerHTML=metric(rows.length,'Taught examples')+metric(l.active.id?l.active.method+' / '+l.active.metric:'Awaiting evidence','Active classifier')+metric(l.runs?.length||0,'Evaluations')+metric(evolutionData.active?'Running':'Ready','Local job');
  $('teaching-summary').innerHTML=Object.entries(groups).map(([label,g])=>'<div class="item"><strong>'+esc(label)+'</strong><small> · '+g.train.size+' training / '+g.validation.size+' validation / '+g.audit.size+' audit groups</small></div>').join('')||'<p class="hint">Choose an uploaded file and tell ORIGIN-0 what category it belongs to. Captions describe image content for optional image training.</p>';
  if(!evolutionLoaded){$('auto-learn').checked=l.auto;evolutionLoaded=true}
  if(!$('learning-runs').contains(document.activeElement))$('learning-runs').innerHTML=(l.runs||[]).slice(-8).reverse().map(r=>'<div class="saved"><div><strong>'+ (r.promoted?'Promoted':'Kept incumbent')+' · revision '+r.revision+'</strong><small>'+stamp(r.created)+' · '+r.candidates.length+' candidate algorithms</small><p>Validation '+Math.round(100*r.validation.balanced_accuracy)+'% · baseline '+Math.round(100*r.baseline.balanced_accuracy)+'% · audit '+Math.round(100*r.audit.balanced_accuracy)+'% ('+r.audit.total+' examples)</p><small>'+esc(r.reason)+'</small></div></div>').join('')||'<p class="hint">Results appear after enough independent examples are available.</p>';
  const jobs=evolutionData.jobs||[];
  if(!$('evolution-jobs').contains(document.activeElement))$('evolution-jobs').innerHTML=jobs.slice().reverse().map(j=>'<article class="saved"><div><strong>'+esc(j.kind)+' · '+esc(j.status)+'</strong><small>'+stamp(j.created)+'</small><p>'+esc(j.message)+'</p>'+(j.result?.device?'<small>Actual worker device: '+esc(j.result.device)+'</small>':'')+'</div><div class="actions"><button data-job-log="'+j.id+'">Show log</button>'+((j.artifacts||[]).some(a=>a.mime.startsWith('image/')||a.mime.startsWith('video/'))?'<button data-job-preview="'+j.id+'">View results</button>':'')+(j.artifacts||[]).map(a=>'<button data-download="'+a.id+'" data-name="'+esc(a.name)+'">Download '+esc(a.name)+'</button>').join('')+(j.kind==='train_lora'&&j.status==='completed'?'<button data-review-adapter="'+j.id+'">Review adapter</button>':'')+'</div></article>').join('')||'<p class="hint">Diagnostics, image training, generation and rebuild jobs appear here with logs and outputs.</p>';
  $('local-job-status').textContent=evolutionData.active?'A local job is running. Its time limit and Cancel control remain active.':'Ready for a configured local job.';
  $('active-adapter').textContent=evolutionData.active_adapter?'Active adapter: '+evolutionData.active_adapter:'Base image model active. New adapters need a recorded visual review.';
  $('auto-training-left').textContent='Automatic training runs remaining this session: '+(evolutionData.config.training_runs||0);
  if(evolutionLog){const out=await api('/api/evolution/log',{id:evolutionLog});$('job-log').textContent=out.log||'Waiting for worker output…'}
 }catch(e){toast(e.message,true)}finally{evolutionBusy=false}
};
$('teach-form').addEventListener('submit',run(async e=>{
 e.preventDefault();await api('/api/learning/label',{asset:$('teach-asset').value,label:$('teach-label').value,caption:$('teach-caption').value,group:$('teach-group').value});
 toast('Teaching example saved. Automatic learning evaluates new evidence every 15 seconds.');
 await window.renderEvolution(true);
}));
$('predict-example').addEventListener('click',run(async()=>{
 const r=await api('/api/learning/predict',{asset:$('teach-asset').value});
 $('prediction-result').textContent='Prediction: '+r.label+'. '+r.note;$('teach-label').value=r.label;
}));
$('train-recognition').addEventListener('click',run(async()=>{await api('/api/learning/train',{});await window.renderEvolution(true)}));
$('rollback-recognition').addEventListener('click',run(async()=>{await api('/api/learning/rollback',{});$('auto-learn').checked=false;await window.renderEvolution(true)}));
$('auto-learn').addEventListener('change',run(async()=>{await api('/api/learning/settings',{auto:$('auto-learn').checked})}));
$('local-runtime-form').addEventListener('submit',run(async e=>{
 e.preventDefault();
 await api('/api/evolution/config',{python:$('python-path').value,go_compiler:$('go-path').value,image_model:$('sdxl-path').value,video_model:$('video-path').value,max_minutes:Number($('job-minutes').value),auto_build:$('auto-build').checked,auto_train:$('auto-train-images').checked,training_runs:Number($('training-allowance').value),train_steps:Number($('training-steps').value),validation_prompt:$('validation-prompt').value});
 toast('Local runtime configured for this session.');await window.renderEvolution(true);
}));
$('cancel-local-job').addEventListener('click',run(async()=>{await api('/api/evolution/cancel',{});await window.renderEvolution(true)}));
async function previewEvolutionJob(id){
 const j=evolutionData.jobs.find(x=>x.id===id);if(!j)return;
 for(const u of evolutionPreviewURLs)URL.revokeObjectURL(u);evolutionPreviewURLs=[];
 const root=$('job-preview');root.replaceChildren();
 for(const a of (j.artifacts||[]).filter(x=>x.mime.startsWith('image/')||x.mime.startsWith('video/'))){
  const bytes=await assetBlob(a.id),u=URL.createObjectURL(new Blob([bytes],{type:a.mime}));evolutionPreviewURLs.push(u);
  const fig=document.createElement('figure'),caption=document.createElement('figcaption'),media=document.createElement(a.mime.startsWith('video/')?'video':'img');
  caption.textContent=a.name;media.src=u;if(media.tagName==='VIDEO')media.controls=true;else media.alt=a.name+' from local model job';
  fig.append(caption,media);root.appendChild(fig);
 }
 root.scrollIntoView({behavior:'smooth',block:'start'});
}
document.addEventListener('click',run(async e=>{
 const b=e.target.closest('button');if(!b)return;
 if(b.dataset.localJob){
  const kind=b.dataset.localJob,prompt=kind==='train_lora'?$('validation-prompt').value:$('local-prompt').value;
  const job=await api('/api/evolution/start',{kind,prompt,steps:kind==='train_lora'?Number($('training-steps').value):Number($('generation-steps').value),seed:Number($('generation-seed').value),target:'windows',adapter_job:kind==='image'&&$('use-adapter').checked?(evolutionData?.active_adapter||''):''});
  evolutionLog=job.id;toast('Local '+kind+' job started.');await window.renderEvolution(true);
 }
 if(b.dataset.jobLog){evolutionLog=b.dataset.jobLog;await window.renderEvolution(true);$('job-log').scrollIntoView({block:'center',behavior:'smooth'})}
 if(b.dataset.jobPreview)await previewEvolutionJob(b.dataset.jobPreview);
 if(b.dataset.reviewAdapter){$('review-adapter-id').value=b.dataset.reviewAdapter;$('adapter-review').hidden=false;await previewEvolutionJob(b.dataset.reviewAdapter)}
}));
$('adapter-review').addEventListener('submit',run(async e=>{
 e.preventDefault();await api('/api/evolution/adapter',{id:$('review-adapter-id').value,preferred:$('adapter-preferred').checked,reason:$('adapter-reason').value});
 $('adapter-review').hidden=true;toast('Visual review saved.');await window.renderEvolution(true);
}));
