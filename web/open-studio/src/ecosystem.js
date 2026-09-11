import {localAPI,escapeHTML as esc} from './lib/muapi.js';

const bytes=n=>n>=1e9?(n/1e9).toFixed(2)+' GB':(n/1e6).toFixed(1)+' MB';
const action=fn=>async e=>{const b=e.currentTarget;b.disabled=true;try{await fn(e)}catch(err){window.originNotice(err.message,true)}finally{if(b.isConnected)b.disabled=false}};
function download(name,text,type='application/json'){const a=document.createElement('a');a.href=URL.createObjectURL(new Blob([text],{type}));a.download=name;a.click();setTimeout(()=>URL.revokeObjectURL(a.href),3000)}
export async function prepare(id){await localAPI('/api/studio/prepare',{id});window.originSetupTarget=id;window.originNotice('Setup started. Engine installation, model download and readiness checks run in order.');}

export function mountEcosystem(el,state){
 const nav=document.createElement('nav');nav.className='origin-source-tabs';nav.setAttribute('aria-label','Model sources');
 const panel=document.createElement('section');panel.id='ecosystem-panel';panel.className='origin-page';panel.hidden=true;
 const catalogue=el.querySelector('#model-cards'),search=el.querySelector('#catalog-search');
 catalogue.before(nav,panel);
 const views=[['packs','Starter packs'],['huggingface','Hugging Face'],['civitai','Civitai'],['installed','Installed models'],['matrix','Stability Matrix']];
 let current=0;
 for(const [id,label]of views){const b=document.createElement('button');b.textContent=label;b.type='button';b.dataset.source=id;b.setAttribute('aria-pressed',String(id==='packs'));nav.append(b);b.onclick=async()=>{
  const version=++current;for(const n of nav.children)n.setAttribute('aria-pressed',String(n===b));catalogue.hidden=id!=='packs';search.hidden=id!=='packs';panel.hidden=id==='packs';panel.replaceChildren();
  if(id==='packs')return;
  if(id==='huggingface'||id==='civitai'){sourceBrowser(panel,id);return}
  panel.textContent='Reading the connected library…';try{const view=id==='matrix'?await matrixPage():await installedPage();if(current===version)panel.replaceChildren(view)}catch(e){if(current===version)panel.textContent=e.message}
 }}
 for(const row of state.catalog){const card=el.querySelector('[data-model="'+CSS.escape(row.model.id)+'"]');if(!card)continue;
  const button=card.querySelector('[data-use]');button.textContent=row.model.recipe==='workflow'?'Use in workflows':'Set up & use';button.onclick=action(async()=>{if(row.model.recipe==='workflow'){window.originNavigate('workflows');return}await prepare(row.model.id)});
 }
}

function sourceBrowser(panel,provider){
 panel.innerHTML=`<h2>${provider==='civitai'?'Civitai models & LoRAs':'Hugging Face model repositories'}</h2><p>Search the live provider, inspect versions and sizes, then add a file to your shared library. Downloads retain their source and SHA-256.</p><form id="provider-search" class="origin-actions"><label>Search ${provider==='civitai'?'Civitai':'Hugging Face'}<input id="provider-query" placeholder="Model name, author, architectural photography…" maxlength="200"></label><button>Search provider</button></form><div id="provider-results" aria-live="polite"></div><div id="provider-detail"></div><details class="origin-card"><summary>Provider access tokens</summary><p>Public models generally need no account. For gated models, use your own token and accept the creator’s terms on the source site. Tokens are saved locally and are not included in diagnostic exports.</p><form id="provider-keys"><label>Hugging Face token<input id="hf-token" type="password" autocomplete="off"></label><label>Civitai token<input id="civitai-token" type="password" autocomplete="off"></label><button>Save tokens</button><small>Blank fields remove saved tokens. Stored in your local data directory; this is not an encrypted credential vault.</small></form></details>`;
 panel.querySelector('#provider-keys').onsubmit=action(async e=>{e.preventDefault();await localAPI('/api/studio/source/keys',{huggingface:panel.querySelector('#hf-token').value,civitai:panel.querySelector('#civitai-token').value});panel.querySelector('#provider-keys').reset();window.originNotice('Provider tokens saved locally.')});
 panel.querySelector('#provider-search').onsubmit=action(async e=>{
  e.preventDefault();const results=panel.querySelector('#provider-results');results.textContent='Searching '+provider+'…';panel.querySelector('#provider-detail').replaceChildren();
  try{const rows=await localAPI('/api/studio/source/search',{provider,query:panel.querySelector('#provider-query').value});results.replaceChildren();results.className='origin-catalog';
   if(!rows.length)results.textContent='No results. Try a model name or author.';
   for(const row of rows){const card=document.createElement('article');card.className='origin-card';card.innerHTML=`<p class="origin-kicker">${esc(row.type||provider)}</p><h3>${esc(row.name)}</h3><p>${esc(row.license)} · ${Number(row.downloads||0).toLocaleString()} downloads</p><div class="origin-actions"><button data-inspect>Versions & files</button><a target="_blank" rel="noopener" href="${esc(row.source)}">Model card ↗</a></div>`;
    card.querySelector('[data-inspect]').onclick=action(()=>showDetail(panel,provider,row.id));results.append(card)}
  }catch(err){results.className='origin-error';results.textContent=err.message;throw err}
 });
}
async function showDetail(panel,provider,id){
 const target=panel.querySelector('#provider-detail');target.textContent='Loading exact model versions and file metadata…';
 const detail=await localAPI('/api/studio/source/inspect',{provider,id});target.replaceChildren();
 const box=document.createElement('section');box.className='origin-card';box.innerHTML=`<h2>${esc(detail.model.name)}</h2><p>${esc(detail.note)}</p><form id="source-import"><label>Model file and version<select id="source-file"></select></label><div id="source-file-info"></div><label>Store in shared folder<select id="source-role">${['checkpoints','loras','diffusion_models','text_encoders','vae','controlnet','clip_vision','upscale_models','embeddings'].map(r=>'<option>'+r+'</option>').join('')}</select></label><label>Generation recipe<select id="source-profile"><option value="workflow">Workflow component · use matching workflow</option><option value="sd15">Complete SD 1.5 checkpoint</option><option value="sdxl">Complete SDXL checkpoint</option></select></label><div class="origin-actions"><button ${detail.files.length?'':'disabled'}>Add to library</button><button type="button" id="source-download" ${detail.files.length?'':'disabled'}>Add & download</button></div></form>`;
 target.append(box);const select=box.querySelector('#source-file');for(const [i,f]of detail.files.entries()){const o=document.createElement('option');o.value=i;o.textContent=(f.version_name?f.version_name+' · ':'')+f.name+' · '+bytes(f.spec.size);select.append(o)}
 const change=()=>{const f=detail.files[Number(select.value)];if(!f)return;box.querySelector('#source-role').value=f.role;box.querySelector('#source-profile').value=f.profile;box.querySelector('#source-file-info').textContent=bytes(f.spec.size)+' · '+(f.base_model||'Architecture must match your selected recipe')+' · SHA-256 '+f.spec.sha256};select.onchange=change;change();
 if(!detail.files.length)box.querySelector('#source-file-info').textContent='No downloadable Safetensors/GGUF file with a full checksum was returned. This may be a gated repository or a model requiring another runtime.';
 const add=async doDownload=>{const f=detail.files[Number(select.value)];if(!f)return;const m=await localAPI('/api/studio/source/import',{provider,id,revision:f.version,file:f.name,role:box.querySelector('#source-role').value,profile:box.querySelector('#source-profile').value});if(doDownload)await localAPI('/api/studio/install',{id:m.id});window.originNotice(doDownload?'Verified download started. The model is also saved in Starter packs.':'Model added to your library. Open Starter packs to download and activate it.')};
 box.querySelector('#source-import').onsubmit=action(async e=>{e.preventDefault();await add(false)});box.querySelector('#source-download').onclick=action(()=>add(true));box.scrollIntoView({block:'start',behavior:'smooth'});
}

async function installedPage(){
 const el=document.createElement('section');el.innerHTML='<h2>Your connected engine’s models</h2><p>These filenames come directly from ComfyUI, including its Stability Matrix shared library.</p><div id="installed-list"></div>';
 const data=await localAPI('/api/studio/comfy',{});const list=el.querySelector('#installed-list');
 const names=data.loaders?.CheckpointLoaderSimple?.input?.required?.ckpt_name?.[0]||[];
 if(!names.length)list.textContent='No checkpoints were reported. Install a pack or link Stability Matrix, then restart the engine with the shared paths.';
 for(const name of names){const row=document.createElement('div');row.className='origin-card';row.innerHTML=`<h3>${esc(name)}</h3><label>Checkpoint family<select><option value="sdxl">SDXL</option><option value="sd15">SD 1.5</option></select></label><button>Use checkpoint</button>`;row.querySelector('button').onclick=action(async()=>{await localAPI('/api/studio/checkpoint',{name,profile:row.querySelector('select').value});window.originNavigate('image')});list.append(row)}
 const loras=data.loaders?.LoraLoader?.input?.required?.lora_name?.[0]||[];const note=document.createElement('p');note.textContent=loras.length+' available LoRAs. Choose a matching LoRA in the image studio’s advanced settings.';list.append(note);return el;
}

async function matrixPage(){
 const m=await localAPI('/api/studio/matrix',{});const el=document.createElement('section');el.innerHTML=`<h2>Stability Matrix</h2><p>The full desktop package manager, connected through shared models and ComfyUI. Install/manage ComfyUI, Forge, SwarmUI, InvokeAI and training packages in Matrix; use the connected ComfyUI models in this studio.</p><div class="origin-actions"><button id="matrix-install" ${m.installed?'disabled':''}>Install Stability Matrix · ${bytes(m.download_bytes)}</button><button id="matrix-open" ${m.installed?'':'disabled'}>Open Stability Matrix</button><a href="${esc(m.source)}" target="_blank" rel="noopener">Open-source project ↗</a><a href="${esc(m.binary_terms)}" target="_blank" rel="noopener">Official binary terms ↗</a></div><p>Matrix opens as its own desktop application. Its original branding and licence stay intact; ORIGIN’s studio remains white-label configurable.</p><form id="matrix-link" class="origin-card"><h3>Reuse an existing installation</h3><label>Stability Matrix Data directory<input id="matrix-library" value="${esc(m.library||'')}" placeholder="C:\\AI\\StabilityMatrix\\Data" required></label><label>Stability Matrix executable (optional for sharing models)<input id="matrix-executable" value="${esc(m.installed?m.executable:'')}" placeholder="C:\\AI\\StabilityMatrix\\StabilityMatrix.exe"></label><button>Link existing library</button></form><p>Shared model root: <code>${esc(m.models||'Created when Matrix is installed or linked')}</code></p><button id="matrix-paths">Download shared-path configuration</button><p>ORIGIN-managed ComfyUI reads this library on its next start. For an already running external ComfyUI, load this YAML with its extra model paths configuration and restart it.</p><div id="matrix-packages"></div>`;
 el.querySelector('#matrix-install').onclick=action(async()=>{await localAPI('/api/studio/matrix/install',{});window.originNotice('Downloading the official Stability Matrix package. When finished, reopen this tab to launch it.')});
 el.querySelector('#matrix-open').onclick=action(async()=>{await localAPI('/api/studio/matrix/open',{});window.originNotice('Stability Matrix launched. Complete any first-run prompts in its window.')});
 el.querySelector('#matrix-link').onsubmit=action(async e=>{e.preventDefault();await localAPI('/api/studio/matrix/link',{library:el.querySelector('#matrix-library').value,executable:el.querySelector('#matrix-executable').value});window.originNotice('Library linked. Restart the engine to discover its models. Existing Matrix settings and files were preserved.')});
 el.querySelector('#matrix-paths').onclick=action(async()=>{const r=await localAPI('/api/studio/model-paths',{},true);download('origin0_shared_models.yaml',await r.text(),'text/yaml')});
 for(const p of m.packages){const row=document.createElement('article');row.className='origin-card';row.innerHTML=`<strong>${esc(p.display_name||p.package_name)}</strong><button>Launch package in Matrix</button>`;row.querySelector('button').onclick=action(async()=>{await localAPI('/api/studio/matrix/open',{package:p.id});window.originNotice('Package launch requested through Stability Matrix.')});el.querySelector('#matrix-packages').append(row)}
 return el;
}

export function DiagnosticsPage(){
 const el=document.createElement('section');el.className='origin-page';el.innerHTML='<h1>Engine status</h1><p>Actual connection state, hardware inventory and the latest startup output.</p><div class="origin-actions"><button id="engine-refresh">Refresh status</button><button id="engine-start">Start installed ComfyUI</button><button id="engine-export">Download diagnostics</button></div><pre id="engine-status">Checking…</pre><h2>ComfyUI startup log</h2><pre id="engine-log"></pre><h2>Stability Matrix startup log</h2><pre id="matrix-log"></pre>';
 let saved=null;const refresh=async()=>{const [d,r]=await Promise.all([localAPI('/api/studio/diagnostics',{}),localAPI('/api/studio/readiness',{})]);saved=d;el.querySelector('#engine-status').textContent=JSON.stringify({readiness:r,hardware:d.hardware,setup:d.install},null,2);el.querySelector('#engine-log').textContent=d.comfy_log||'No managed ComfyUI startup output yet.';el.querySelector('#matrix-log').textContent=d.matrix_log||'No Matrix startup output yet.'};
 el.querySelector('#engine-refresh').onclick=action(refresh);el.querySelector('#engine-start').onclick=action(async()=>{await localAPI('/api/studio/comfy-start',{});await refresh()});el.querySelector('#engine-export').onclick=action(async()=>{await refresh();download('ORIGIN0_STUDIO_DIAGNOSTICS.json',JSON.stringify(saved,null,2))});refresh().catch(e=>{el.querySelector('#engine-status').textContent=e.message});return el;
}
