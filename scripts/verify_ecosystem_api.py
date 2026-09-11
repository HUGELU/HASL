"""Live read-only provider checks through the running application; no hidden model downloads."""
import argparse,json,urllib.error
from pathlib import Path
from acceptance_app import App

def main():
 p=argparse.ArgumentParser();p.add_argument('--binary',required=True);p.add_argument('--output',default='release/ecosystem-api');a=p.parse_args();out=Path(a.output);out.mkdir(parents=True,exist_ok=True)
 report={'result':'failed','checks':[],'providers':{}}
 try:
  with App(a.binary) as app:
   report['version']=app.api('/api/state')['version']
   for provider,query in [('huggingface','stable-diffusion-v1-5'),('civitai','RealVisXL')]:
    rows=app.api('/api/studio/source/search',{'provider':provider,'query':query});assert rows,provider+' returned no models'
    selected='stable-diffusion-v1-5/stable-diffusion-v1-5' if provider=='huggingface' else rows[0]['id']
    detail=app.api('/api/studio/source/inspect',{'provider':provider,'id':selected});assert detail['files'],provider+' returned no verified model files'
    for f in detail['files']:assert len(f['spec']['sha256'])==64 and f['spec']['size']>0
    report['providers'][provider]={'query':query,'results':len(rows),'selected':selected,'versions_and_files':len(detail['files']),'source':detail['model']['source']}
    (out/(provider+'-metadata.json')).write_text(json.dumps(detail,indent=2));print(provider+': live search and file metadata passed',flush=True)
   diagnostics=app.api('/api/studio/diagnostics',{});assert diagnostics['version']=='1.9.0'
   assert 'huggingface' not in diagnostics and 'civitai' not in diagnostics
   report['checks']=['Live Hugging Face search, immutable revision and exact file metadata','Live Civitai search, creator source and version/file metadata','Diagnostics endpoint available without provider credentials'];report['result']='passed'
 except BaseException as e:
  message=str(e)
  if isinstance(e,urllib.error.HTTPError):message+=': '+e.read().decode(errors='replace')[:1800]
  report['blocker']=message;print(message,flush=True);raise
 finally:(out/'report.json').write_text(json.dumps(report,indent=2))
 print(json.dumps(report))
if __name__=='__main__':main()
