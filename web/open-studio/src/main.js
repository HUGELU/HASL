import {ImageStudio} from './components/ImageStudio.js';
import {CinemaStudio} from './components/CinemaStudio.js';
import {setModels} from './lib/models.js';
import {localAPI,assetURL} from './lib/muapi.js';
import {ModelsPage,BrandingPage,WorkflowsPage,GalleryPage,bytes} from './origin_pages.js';

const app=document.querySelector('#app');
app.innerHTML='<header class="origin-nav"><a id="origin-brand" href="#"><span id="origin-logo">◈</span><span><strong id="origin-name">ORIGIN-0</strong><small id="origin-tagline">Your local creative studio</small></span></a><nav aria-label="Creative studio"></nav></header><div id="origin-notice" role="status" hidden></div><div id="origin-work" role="status" hidden><progress></progress><span></span><button id="origin-stop-download">Stop download</button></div><main id="content-area"></main>';
const content=document.querySelector('#content-area'),notice=document.querySelector('#origin-notice');
let route='image',version=0;
window.originNotice=(text,error=false)=>{notice.hidden=false;notice.textContent=text;notice.classList.toggle('origin-error',error)};
const routes=[['image','Create image'],['cinema','Cinema studio'],['models','Models & engines'],['workflows','Video & workflows'],['gallery','Gallery & queue'],['branding','White label']];
for(const [id,label]of routes){const b=document.createElement('button');b.textContent=label;b.dataset.route=id;b.onclick=()=>navigate(id);document.querySelector('.origin-nav nav').append(b)}
async function brand(state){const c=state.config;document.querySelector('#origin-name').textContent=c.brand;document.querySelector('#origin-tagline').textContent=c.tagline;document.title=c.brand+' · Creative studio';document.documentElement.style.setProperty('--origin-accent',c.accent);window.originActiveModel=c.active_model;if(c.logo){const logo=document.createElement('img');logo.alt=c.brand+' logo';logo.src=await assetURL(c.logo);document.querySelector('#origin-logo').replaceChildren(logo)}else document.querySelector('#origin-logo').textContent='◈';window.parent.originStudioBrand?.(c)}
async function navigate(page){
 const current=++version;route=page;notice.hidden=true;document.querySelectorAll('[data-route]').forEach(b=>b.setAttribute('aria-current',String(b.dataset.route===page)));
 try{const s=await localAPI('/api/studio/state',{});if(current!==version)return;setModels(s);await brand(s);content.replaceChildren();let view;
 if(page==='models')view=ModelsPage(s);else if(page==='branding')view=BrandingPage(s);else if(page==='workflows')view=WorkflowsPage(s);else if(page==='gallery')view=await GalleryPage(s);else if(page==='cinema')view=CinemaStudio();else view=ImageStudio();if(current!==version)return;content.append(view);
 if(page==='image'&&!s.catalog.find(x=>x.model.id===s.config.active_model)?.installed){window.originNotice('First run: open Models & engines and download a model pack. Your existing ORIGIN tools are still available in the workspace tabs.')}
 }catch(e){window.originNotice(e.message,true)}
}
window.originNavigate=navigate;
document.querySelector('#origin-brand').onclick=e=>{e.preventDefault();navigate('image')};
window.addEventListener('navigate',e=>navigate(e.detail.page==='settings'?'branding':e.detail.page));
window.addEventListener('origin-progress',e=>{const j=e.detail;window.originNotice(j.status+' · '+j.message)});
document.querySelector('#origin-stop-download').onclick=async()=>{await localAPI('/api/studio/stop-download',{});await localAPI('/api/images/cancel',{id:'setup'}).catch(()=>{});};
let polling=false;
setInterval(async()=>{if(polling||window.parent.privacyLocked)return;polling=true;try{const [s,n]=await Promise.all([localAPI('/api/studio/state',{}),localAPI('/api/images/state',{})]);const i=['downloading','extracting'].includes(s.install.status)?s.install:n.busy&&!n.ready?n.setup:null;const bar=document.querySelector('#origin-work');bar.hidden=!i;if(i){bar.querySelector('span').textContent=i.message+' · '+bytes(i.done||0)+' / '+bytes(i.total||0);const p=bar.querySelector('progress');if(i.status==='extracting'||!i.total)p.removeAttribute('value');else{p.max=i.total;p.value=i.done}}if(s.install.status==='failed')window.originNotice(s.install.message,true)}catch(e){if(!String(e.message).includes('locked'))window.originNotice(e.message,true)}finally{polling=false}},2200);
navigate('image');
