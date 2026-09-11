// ORIGIN adaptation of the Open-Higgsfield-AI client contract. Same-origin local requests only.
import { getModelById } from './models.js';
export const escapeHTML=s=>String(s??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
export async function localAPI(path,body,raw=false){
 const headers={'X-Origin-Key':sessionStorage.getItem('origin-session')||''};
 const options={headers,method:body===undefined?'GET':'POST'};
 if(body!==undefined){if(body instanceof Blob){options.body=body;headers['Content-Type']=body.type||'application/octet-stream'}else{options.body=JSON.stringify(body);headers['Content-Type']='application/json'}}
 options.signal=AbortSignal.timeout(/\/(state|readiness)$/.test(path)?12000:60000);const r=await fetch(path,options);if(r.status===423){window.parent?.showPrivacy?.();throw new Error('Workspace locked')};if(!r.ok)throw new Error((await r.text()).slice(0,1200));if(raw)return r;if(r.status===204||r.status===202)return null;return r.json();
}
const assetURLs=new Map();
const sockets=new Map();
export function watchComfy(id){if(sockets.has(id))return;const ws=new WebSocket(location.origin.replace(/^http/,'ws')+'/studio/ws?clientId='+encodeURIComponent(id),['origin0',sessionStorage.getItem('origin-session')||'locked']);ws.binaryType='arraybuffer';sockets.set(id,ws);ws.onmessage=e=>{if(typeof e.data==='string'){try{const m=JSON.parse(e.data);if(['progress','executing','execution_start'].includes(m.type))window.dispatchEvent(new CustomEvent('origin-comfy-live',{detail:m}));if(m.type==='executing'&&m.data?.node===null){ws.close();sockets.delete(id)}}catch{}}else{const d=new DataView(e.data);if(d.byteLength>8&&d.getUint32(0)===1){const mime=d.getUint32(4)===2?'image/png':'image/jpeg';window.dispatchEvent(new CustomEvent('origin-comfy-preview',{detail:new Blob([e.data.slice(8)],{type:mime})}))}}};ws.onclose=()=>sockets.delete(id)}
window.closeMediaChannels=()=>{for(const ws of sockets.values())ws.close();sockets.clear()};
export async function assetURL(id,mime='image/png'){if(assetURLs.has(id))return assetURLs.get(id);const r=await localAPI('/api/asset?id='+encodeURIComponent(id),undefined,true),b=await r.blob(),u=URL.createObjectURL(new Blob([b],{type:mime}));assetURLs.set(id,u);return u}
export class MuapiClient{
 getKey(){return 'local-session'}
 async uploadFile(file){const a=await localAPI('/api/upload?name='+encodeURIComponent(file.name),file);return 'origin-asset:'+a.id}
 async generateImage(p){
  const m=getModelById(p.model);if(!m)throw new Error('Select a local model from Models & engines');
  const quantum=m.engine==='native'?64:16;const [a,b]=(p.aspect_ratio||'1:1').split(':').map(Number);let width=p.width||m.width||512,height=p.height||Math.max(256,Math.round(width*b/a/quantum)*quantum);
  if(!p.width&&!p.height&&height>width){height=width;width=Math.max(256,Math.round(height*a/b/quantum)*quantum)}
  const refs=p.images_list||[p.image_url].filter(Boolean);if(refs.length>1)throw new Error('This image recipe accepts one reference. Use a multi-reference ComfyUI workflow for several images.');
  let init='';if(refs[0]){if(!refs[0].startsWith('origin-asset:'))throw new Error('Upload this reference to the local workspace again');init=refs[0].slice(13)}
  const request={model:p.model,prompt:p.prompt||'',negative_prompt:p.negative_prompt||'',width,height,steps:p.steps??m.steps,guidance_scale:p.guidance_scale??m.guidance,seed:p.seed??-1,init_asset:init,strength:p.strength??.5,lora:p.lora||'',lora_weight:p.lora_weight??1};
  const count=Math.min(8,Math.max(1,p.batch_count||1));let first;
  for(let i=0;i<count;i++){const j=await localAPI('/api/studio/generate',{...request,seed:request.seed===-1?-1:request.seed+i});if(!first){first=j;if(p.onRequestId)p.onRequestId(j.id)}if(j.id.startsWith('comfy-'))watchComfy(j.id)}
  return this.pollForResult(first.id);
 }
 generateI2I(p){return this.generateImage(p)}
 async pollForResult(id){
  const end=Date.now()+2*60*60*1000;let failures=0;
  while(Date.now()<end){
   let j;try{const data=await localAPI(id.startsWith('img-')?'/api/images/state':'/api/studio/state',{});j=(data.jobs||[]).find(j=>j.id===id);failures=0}catch(e){if(++failures>=3)throw e;await new Promise(r=>setTimeout(r,1500));continue}
   if(!j)throw new Error('Generation not found in saved job history');
   window.dispatchEvent(new CustomEvent('origin-progress',{detail:j}));
   if(j.status==='completed'){const a=j.asset||j.assets?.[0];if(!a)throw new Error('Completed job has no saved output');return {...j,url:await assetURL(a.id,a.mime),outputs:[await assetURL(a.id,a.mime)]}}
   if(!['queued','running','finishing'].includes(j.status))throw new Error(j.message||j.status);
   await new Promise(r=>setTimeout(r,1200));
  }throw new Error('Monitoring reached two hours. Inspect the saved queue before resubmitting.');
 }
}
export const muapi=new MuapiClient();
