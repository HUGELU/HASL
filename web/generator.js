'use strict';
let nativeState=null,nativeLoaded=false,nativeRefreshBusy=false,nativeJobSignature='',poolState=null;
const nativeImages=new Map();
function nativeDuration(j){const end=j.finished||Math.floor(Date.now()/1000);const secs=Math.max(0,end-(j.started||j.created));return secs<60?secs+'s':Math.floor(secs/60)+'m '+secs%60+'s'}
window.renderNativeImages=()=>{};
async function refreshNative(){
 if(nativeRefreshBusy)return;nativeRefreshBusy=true;
 try{
  const [n,p]=await Promise.all([api('/api/images/state',{}),api('/api/pool/state',{})]);nativeState=n;poolState=p;
  if(!nativeLoaded){$('native-backend').value=n.config.backend;$('native-threads').value=n.config.threads;$('native-minutes').value=n.config.max_minutes;nativeLoaded=true;$('native-model-sources').innerHTML=n.catalog.files.map(f=>`<p><a href="${esc(f.source)}" target="_blank" rel="noopener noreferrer">${esc(f.name)}</a> · ${size(f.size)} · ${esc(f.license)}</p>`).join('')}
  $('image-ready').textContent=n.ready?'Ready · local generation':n.setup.status==='running'?'Preparing engine':'Setup required';$('native-platform').textContent=n.platform+' · '+n.config.threads+' CPU threads';
  $('image-setup-status').textContent=n.setup.message;$('native-device-report').textContent=n.setup.devices||'Device availability is measured by the native runtime during setup.';
  const downloading=n.setup.status==='running';$('native-progress').hidden=!downloading;if(n.setup.total>0){$('native-progress').value=100*n.setup.done/n.setup.total}else{$('native-progress').removeAttribute('value')}
  $('native-progress-label').textContent=downloading&&n.setup.total>0?`${size(n.setup.done)} / ${size(n.setup.total)} · file ${Math.max(1,n.setup.file)} of ${n.setup.files}`:'';
  $('image-setup').disabled=n.busy;$('image-setup').textContent=n.ready?'Verify / update device settings':'Set up image engine';$('cancel-image-setup').hidden=!downloading;
  $('native-generate').disabled=!(n.ready||$('native-shared').checked&&p.hosting);$('native-shared').disabled=!p.hosting;
  const jobs=n.jobs||[],active=jobs.filter(j=>['running','queued'].includes(j.status));$('native-queue-summary').textContent=active.length+' active · '+jobs.filter(j=>j.status==='completed').length+' images completed';
  const shown=jobs.slice(-16).reverse();const sig=shown.map(j=>[j.id,j.status,j.asset?.id,j.message]).join('|');
  if(sig!==nativeJobSignature&&!$('native-jobs').contains(document.activeElement)){
   nativeJobSignature=sig;$('native-jobs').innerHTML=shown.map(j=>`<article class="native-job" data-status="${esc(j.status)}"><div id="preview-${j.id}"></div><div class="native-job-body"><div class="row spread"><b>${esc(j.status)}</b><span class="native-job-time hint" data-time-job="${j.id}">${nativeDuration(j)}</span></div><p>${esc(j.request.prompt)}</p><p class="hint">${j.request.width} × ${j.request.height} · ${j.request.steps} steps · seed ${j.request.seed} · ${esc(j.worker||j.backend||'local')}</p><p class="hint">${esc(j.message)}</p><div class="actions">${j.asset?`<button data-native-download="${j.asset.id}" data-native-name="${j.asset.name}">Download PNG</button>`:''}<button data-reuse-image="${j.id}">Use prompt again</button>${['running','queued'].includes(j.status)?`<button data-cancel-image="${j.id}">Cancel job</button>`:''}</div>${j.log?`<details><summary>Generation log</summary><pre id="log-${j.id}"></pre></details>`:''}</div></article>`).join('')||'<p class="empty-studio">Your generated images will appear here. Nothing is pre-rendered.</p>';
   for(const j of shown){if(j.log&&$('log-'+j.id))$('log-'+j.id).textContent=j.log;if(j.asset){let u=nativeImages.get(j.asset.id);if(!u){const b=await assetBlob(j.asset.id);u=URL.createObjectURL(b);nativeImages.set(j.asset.id,u)}const box=$('preview-'+j.id);if(box){const img=document.createElement('img');img.src=u;img.alt=j.request.prompt;img.loading='lazy';box.replaceChildren(img)}}}
   const visible=new Set(shown.map(j=>j.asset?.id));for(const [id,u]of nativeImages){if(!visible.has(id)){URL.revokeObjectURL(u);nativeImages.delete(id)}}
  }
  for(const el of document.querySelectorAll('[data-time-job]')){const j=jobs.find(x=>x.id===el.dataset.timeJob);if(j)el.textContent=nativeDuration(j)}
  $('pool-address').textContent=p.relay_status||p.address||'Group is stopped';$('pool-host').disabled=p.hosting;$('pool-stop').disabled=!p.hosting;$('pool-invite').disabled=!p.hosting;$('pool-join').disabled=p.joined;$('pool-leave').disabled=!p.joined;
  $('pool-worker-status').textContent=p.worker_status+` · ${p.completed}/${p.budget} completed`;$('pool-peers').innerHTML=(p.peers||[]).map(x=>`<div class="item"><b>${esc(x.name)}</b><p class="hint">${x.completed} images returned · ${x.seen?'last seen '+stamp(x.seen):'waiting for invitation to be used'}</p></div>`).join('');
 }catch(e){$('image-setup-status').textContent='Cannot reach ORIGIN-0: '+e.message}finally{nativeRefreshBusy=false}
}
$('image-setup-form').addEventListener('submit',run(async e=>{e.preventDefault();await api('/api/images/setup',{backend:$('native-backend').value,threads:Number($('native-threads').value),max_minutes:Number($('native-minutes').value)});toast('Setup started. Progress and any errors appear here.');await refreshNative()}));
$('cancel-image-setup').addEventListener('click',run(async()=>{await api('/api/images/cancel',{id:'setup'});await refreshNative()}));
$('native-shared').addEventListener('change',()=>{if(nativeState)$('native-generate').disabled=!(nativeState.ready||$('native-shared').checked&&poolState?.hosting)});
$('native-image-form').addEventListener('submit',run(async e=>{e.preventDefault();const [width,height]=$('native-size').value.split('x').map(Number);const job=await api('/api/images/generate',{prompt:$('native-prompt').value,width,height,steps:Number($('native-steps').value),seed:Number($('native-seed').value),shared:$('native-shared').checked});toast('Image job queued: '+job.id);await refreshNative()}));
$('native-jobs').addEventListener('click',run(async e=>{const b=e.target.closest('button');if(!b)return;if(b.dataset.cancelImage){await api('/api/images/cancel',{id:b.dataset.cancelImage});await refreshNative()}if(b.dataset.nativeDownload)await downloadAsset(b.dataset.nativeDownload,b.dataset.nativeName);if(b.dataset.reuseImage){const j=nativeState.jobs.find(j=>j.id===b.dataset.reuseImage);if(j){$('native-prompt').value=j.request.prompt;$('native-steps').value=j.request.steps;$('native-seed').value=j.request.seed;$('native-size').value=j.request.width+'x'+j.request.height;$('native-prompt').focus()}}}));
$('image-diagnostics').addEventListener('click',run(async()=>{const r=await api('/api/images/diagnostics',{}, {raw:true});await downloadBlob(await r.blob(),'ORIGIN0_DIAGNOSTICS.json')}));
$('pool-host-form').addEventListener('submit',run(async e=>{e.preventDefault();await api('/api/pool/action',{action:'host',listen:$('pool-listen').value,advertise:$('pool-advertise').value});await refreshNative()}));
$('pool-invite').addEventListener('click',run(async()=>{const r=await api('/api/pool/action',{action:'invite'});$('pool-invite-output').value=r.invite;toast('Private invitation created. Copy it to the worker PC.')}));
$('pool-copy-invite').addEventListener('click',run(async()=>{if(!$('pool-invite-output').value)throw new Error('Create an invitation first.');await navigator.clipboard.writeText($('pool-invite-output').value);toast('Invitation copied.')}));
$('pool-stop').addEventListener('click',run(async()=>{await api('/api/pool/action',{action:'stop'});$('pool-invite-output').value='';$('native-shared').checked=false;await refreshNative()}));
$('pool-join-form').addEventListener('submit',run(async e=>{e.preventDefault();await api('/api/pool/action',{action:'join',invite:$('pool-invite-input').value,name:$('pool-worker-name').value,budget:Number($('pool-budget').value)});$('pool-invite-input').value='';await refreshNative()}));
$('pool-leave').addEventListener('click',run(async()=>{await api('/api/pool/action',{action:'leave'});await refreshNative()}));
refreshNative();setInterval(refreshNative,1500);

$('pool-relay-form').addEventListener('submit',run(async e=>{e.preventDefault();try{await api('/api/pool/action',{action:'host-internet',relay_url:$('pool-relay-url').value,relay_key:$('pool-relay-key').value});toast('Internet group started. Create a private invitation for each worker.');await refreshNative()}finally{$('pool-relay-key').value=''}}));
