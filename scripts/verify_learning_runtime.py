"""Real local HTTP learning, worker, cancellation and rebuild acceptance checks."""
import io
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request
import zipfile

ROOT=Path(__file__).resolve().parents[1]
BASE=Path(tempfile.mkdtemp(prefix='origin0_v14_learning_'))
EXE=ROOT/'release'/'origin0-linux'
GO=ROOT.parent/'toolchain'/'go'/'bin'/'go'
report={'platform':sys.platform,'checks':[],'limitations':['No actual GPU, diffusion model or video model is installed.','Windows targets were compiled but not executed on Windows.']}
processes=[]


def boot(exe,base):
    base.mkdir(parents=True,exist_ok=True)
    log=open(base/'console.log','w')
    env=os.environ.copy();env.update(ORIGIN0_HOME=str(base),ORIGIN0_NO_BROWSER='1',GOMAXPROCS='2')
    p=subprocess.Popen([str(exe)],stdout=log,stderr=subprocess.STDOUT,env=env)
    processes.append((p,log))
    deadline=time.monotonic()+15
    while time.monotonic()<deadline:
        data=(base/'console.log').read_text(errors='replace')
        match=re.search(r'http://(127\.0\.0\.1:\d+)/#([a-f0-9]+)',data)
        if match:return p,'http://'+match[1],match[2]
        if p.poll() is not None:raise RuntimeError(data)
        time.sleep(.1)
    raise TimeoutError('App did not start')


def call(path,value=None,raw=None,address=None):
    origin,session=address or (url,key)
    headers={'X-Origin-Key':session}
    data=raw
    if value is not None:
        data=json.dumps(value).encode();headers['Content-Type']='application/json'
    request=urllib.request.Request(origin+path,data=data,headers=headers)
    with urllib.request.urlopen(request,timeout=30) as r:
        b=r.read()
        return json.loads(b) if b and r.headers.get('Content-Type','').startswith('application/json') else b


def job_done(id,timeout=240):
    deadline=time.monotonic()+timeout
    while time.monotonic()<deadline:
        ev=call('/api/evolution/state',{})
        j=next(x for x in ev['jobs'] if x['id']==id)
        if j['status'] not in ['running','queued']:
            if j['status']!='completed':
                raise AssertionError(json.dumps(j)+'\n'+call('/api/evolution/log',{'id':id})['log'])
            return j
        time.sleep(.25)
    raise TimeoutError(call('/api/evolution/log',{'id':id})['log'])

try:
    proc,url,key=boot(EXE,BASE)
    call('/api/compute',{'target':15})
    call('/api/learning/settings',{'auto':False})
    assets=[]
    for category in range(2):
        for i in range(5):
            content=bytes([32+category*192])*(800+i*20)+bytes([100])*(i+1)
            a=call('/api/upload?name=synthetic-%s-%s.dat'%(category,i),raw=content)
            call('/api/learning/label',{'asset':a['id'],'label':'category-'+str(category)})
            assets.append(a)
    out=call('/api/learning/train',{})
    assert out['promoted'] and len(out['candidates'])==6 and out['audit']['total']==2
    assert out['validation']['accuracy']==1 and out['audit']['accuracy']==1
    s=call('/api/learning/state',{});model=s['active']['id']
    assert len(s['examples'])==10 and all(x['features'] is None for x in s['examples'])
    prediction=call('/api/learning/predict',{'asset':assets[-1]['id']})
    assert prediction['label']=='category-1'
    report['checks'].append('HTTP upload, teaching, six measured algorithm candidates, separate audit and prediction')
    report['recognition']={'validation':out['validation'],'audit':out['audit'],'active_metric':s['active']['metric']}
    cp=call('/api/snapshot',{'action':'save','name':'Trained state'})
    call('/api/snapshot',{'action':'branch','id':cp['id'],'name':'learning-branch'})
    assert call('/api/learning/state',{})['active']['id']==model
    call('/api/snapshot',{'action':'restore','id':cp['id']})
    report['checks'].append('Trained model follows snapshot branching and restore')
    call('/api/evolution/config',{'python':sys.executable,'go_compiler':str(GO),'max_minutes':4})
    j=call('/api/evolution/start',{'kind':'diagnostics','steps':1})
    diag=job_done(j['id'])
    assert diag['result']['packages']['torch']=='missing' and diag['result']['device']=='unavailable'
    assert 'diagnostics' in call('/api/evolution/log',{'id':j['id']})['log']
    report['checks'].append('Bundled worker actually ran; missing PyTorch/GPU was reported without fabrication')
    report['diagnostics']=diag['result']
    for target in ['linux','windows']:
        j=call('/api/evolution/start',{'kind':'rebuild','target':target})
        built=job_done(j['id'])
        manifest=built['result']
        assert manifest['metric']=='l1' and manifest['tests']=='passed' and manifest['vet']=='passed'
        archive=call('/api/asset?id='+built['artifacts'][0]['id'])
        output=BASE/('rebuilt-'+target)
        with zipfile.ZipFile(io.BytesIO(archive)) as z:
            assert z.testzip() is None
            z.extractall(output)
        report['checks'].append('Actual '+target+' rebuild from embedded source passed host tests/vet and produced downloadable binary/source/checksums')
        report['rebuild_'+target]={k:v for k,v in manifest.items() if k not in ['old_kernel','new_kernel']}
        if target=='linux':
            new_exe=output/'origin0-next';new_exe.chmod(0o700)
            child,cu,ck=boot(new_exe,BASE/'rebuilt-instance')
            ev=call('/api/evolution/state',{},address=(cu,ck))
            assert ev['compiled_metric']=='l1'
            call('/api/stop',{},address=(cu,ck));child.wait(timeout=15)
            report['checks'].append('Executed the newly rebuilt Linux binary and verified its changed compiled numeric kernel')
        else:
            assert (output/'ORIGIN0-next.exe').read_bytes()[:2]==b'MZ'
    call('/api/stop',{});proc.wait(timeout=15)
    proc,url,key=boot(EXE,BASE)
    s=call('/api/learning/state',{})
    assert s['active']['id']==model and len(s['examples'])==10
    ev=call('/api/evolution/state',{})
    assert not ev['active'] and not ev['config']['auto_train']
    assert len(ev['jobs'])==3
    report['checks'].append('Teaching state, model and completed job history survived a real process restart')
    call('/api/stop',{});proc.wait(timeout=15)
    report['status']='PASS'
finally:
    for p,log in processes:
        if p.poll() is None:
            p.terminate()
            try:p.wait(timeout=5)
            except subprocess.TimeoutExpired:p.kill();p.wait()
        log.close()
    report['runtime_directory']=str(BASE)
    (ROOT/'release'/'learning-runtime-validation.json').write_text(json.dumps(report,indent=2))
print(json.dumps(report,indent=2))
