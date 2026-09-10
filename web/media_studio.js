'use strict';
window.originStudioHost=(target,asset)=>{setPage(target);if(asset&&target==='production'){window.useForFinishing?.(asset,'Open Studio output')}};
window.originStudioBrand=c=>{const brand=document.querySelector('.brand');if(brand){brand.replaceChildren();const label=document.createElement('span');label.textContent=c.brand;const small=document.createElement('small');small.textContent=c.tagline;label.append(small);brand.append(label)}if(!window.privacyLocked)document.title=c.brand+' · Creative studio'};
function ensureOpenStudio(){if(window.privacyLocked)return;const frame=$('open-studio-frame');if(!frame.getAttribute('src'))frame.src='/open-studio/';}
const oldRenderPage=renderPage;renderPage=function(){oldRenderPage();if(page==='media')ensureOpenStudio()};
setPage('media');
setInterval(()=>{if(page==='media')ensureOpenStudio()},1000);
