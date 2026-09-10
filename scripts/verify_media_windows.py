"""Real Windows portable installation and neural generation through the new studio API."""
import argparse,json,time,urllib.error,shutil,hashlib,struct
from pathlib import Path
from acceptance_app import App

def main():
 p=argparse.ArgumentParser();p.add_argument('--binary',required=True);p.add_argument('--output',default='release/media-windows');a=p.parse_args();out=Path(a.output);out.mkdir(parents=True,exist_ok=True)
 report={'result':'failed','checks':[],'measurements':[],'scope':'Real Windows CPU execution. Small acceptance images test functionality, not image-quality superiority or all-device performance.'}
 def wait(label,read,done,failed,seconds):
  start=time.monotonic();last=''
  while time.monotonic()-start<seconds:
   state=read();message=state.get('message',state.get('status',''));message+=(' · '+str(state.get('done',0))+'/'+str(state['total'])+' bytes') if state.get('total') else ''
   if message!=last:print(label+': '+message,flush=True);last=message
   if failed(state):raise RuntimeError(label+': '+message)
   if done(state):return state,time.monotonic()-start
   time.sleep(2)
  raise RuntimeError(label+' exceeded its deadline. Latest output: '+last)
 def image(app,job,name):
  route='/api/images/state' if job['id'].startswith('img-') else '/api/studio/state'
  result,elapsed=wait(name,lambda:next(j for j in app.api(route,{})['jobs'] if j['id']==job['id']),lambda j:j['status']=='completed',lambda j:j['status'] not in ['queued','running','finishing','completed'],600)
  if job['id'].startswith('comfy-'):result=app.api('/api/studio/job',{'id':job['id']})
  asset=result.get('asset') or result['assets'][0];b=app.api('/api/asset?id='+asset['id']);(out/(name+'.png')).write_bytes(b);assert b.startswith(b'\x89PNG');width,height=struct.unpack('>II',b[16:24]);assert width==256 and height==256
  (out/(name+'-parameters.json')).write_text(json.dumps(result,indent=2));report['measurements'].append({'task':name,'seconds':round(elapsed,2),'width':width,'height':height,'sha256':hashlib.sha256(b).hexdigest()});return result
 try:
  with App(a.binary) as app:
   try:
    report['app_version']=app.api('/api/state')['version']
    state=app.api('/api/studio/state',{});report['hardware']=state['hardware'];assert len(state['catalog'])==7
    app.api('/api/images/setup',{'backend':'cpu','threads':4,'max_minutes':15})
    wait('native setup',lambda:app.api('/api/images/state',{})['setup'],lambda s:s['status']=='ready',lambda s:s['status'] in ['failed','error','cancelled'],1200)
    image(app,app.api('/api/studio/generate',{'model':'native-z-image','prompt':'Architectural photograph of a timber pavilion beside a lake in daylight','width':256,'height':256,'steps':2,'seed':0}),'native-studio')
    report['checks'].append('New studio endpoint generated and saved a real native Z-Image-Turbo image')
    app.api('/api/studio/config',{**state['config'],'device':'cpu'})
    app.api('/api/studio/comfy-setup',{})
    wait('ComfyUI portable',lambda:app.api('/api/studio/state',{})['install'],lambda s:s['status']=='ready',lambda s:s['status'] in ['failed','error','cancelled'],1200)
    deadline=time.monotonic()+180;error=''
    while time.monotonic()<deadline:
     try:
      probe=app.api('/api/studio/comfy',{});break
     except (urllib.error.HTTPError,urllib.error.URLError) as e:error=str(e);time.sleep(3)
    else:raise RuntimeError('ComfyUI startup failed: '+error)
    report['comfy_system']=probe['system'];report['checks'].append('Official pinned Windows portable extracted with bundled Windows tar; embedded Python started ComfyUI in CPU mode')
    app.api('/api/studio/install',{'id':'sd15'})
    wait('SD 1.5 model',lambda:app.api('/api/studio/state',{})['install'],lambda s:s['status']=='ready',lambda s:s['status'] in ['failed','error','cancelled'],1200)
    # ComfyUI refreshes its filename lists when object_info is requested.
    probe=app.api('/api/studio/comfy',{});loader=probe['loaders']['CheckpointLoaderSimple']['input']['required']['ckpt_name'][0];assert 'v1-5-pruned-emaonly.safetensors' in loader
    report['checks'].append('Pinned model downloaded to shared model folders and discovered by the running ComfyUI checkpoint loader')
    result=image(app,app.api('/api/studio/generate',{'model':'sd15','prompt':'A sunlit timber pavilion in a garden, architectural photograph','width':256,'height':256,'steps':2,'guidance_scale':7,'seed':0}),'comfy-studio')
    image(app,app.api('/api/studio/generate',{'model':'sd15','prompt':'A sunlit timber pavilion in a garden with red flowers','width':256,'height':256,'steps':2,'guidance_scale':7,'seed':0,'init_asset':result['assets'][0]['id'],'strength':.6}),'comfy-reference')
    report['checks'].append('Real ComfyUI text-to-image and reference-image workflows executed through ORIGIN and saved their outputs and graphs')
    report['result']='passed'
   except BaseException:
    report['process_exit_code']=app.proc.poll()
    diagnostic=app.logpath.read_text(errors='replace').replace(app.key,'[session-redacted]')
    print('ORIGIN process exit code: '+str(report['process_exit_code'])+'\nLatest ORIGIN output:\n'+diagnostic[-14000:],flush=True)
    raise
   finally:
    data=app.home/'origin0_data'
    for source,dest in [(app.logpath,'launch.log'),(data/'logs/runtime.log','runtime.log'),(data/'media-studio/comfy-runtime.log','comfy-runtime.log'),(data/'media-studio/studio.json','studio-state.json')]:
     if source.exists():(out/dest).write_text(source.read_text(errors='replace').replace(app.key,'[session-redacted]'),encoding='utf-8')
 except BaseException as e:report['blocker']=str(e);raise
 finally:(out/'report.json').write_text(json.dumps(report,indent=2))
 print(json.dumps(report))
if __name__=='__main__':main()
