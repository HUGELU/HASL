// Local catalogue adaptation. Hosted model names are not advertised as free weights.
export let t2iModels=[];
export let i2iModels=[];
export function setModels(state){
 t2iModels=state.catalog.map(x=>({...x.model,installed:x.installed,family:x.model.engine,inputs:{aspect_ratio:{default:'1:1'}}}));
 const active=state.config.active_model;t2iModels.sort((a,b)=>(a.id===active?-1:b.id===active?1:0));
 i2iModels=t2iModels.filter(m=>m.recipe!=='flux2');
}
export const getModelById=id=>t2iModels.find(m=>m.id===id);
export const getI2IModelById=getModelById;
export const getAspectRatiosForModel=()=>['1:1','16:9','9:16','4:3','3:4','3:2'];
export const getAspectRatiosForI2IModel=getAspectRatiosForModel;
export const getResolutionsForModel=()=>[];
export const getResolutionsForI2IModel=getResolutionsForModel;
export const getQualityFieldForModel=()=>null;
export const getQualityFieldForI2IModel=getQualityFieldForModel;
export const getMaxImagesForI2IModel=()=>1;
