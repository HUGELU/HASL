"""Build the small NCNN CPU/Vulkan host and package the original x4plus weights.
Dependencies are fetched from their official repositories. The initial CI discovery
run records the historical model archive hash; a release requires it to be pinned.
"""
import argparse, hashlib, json, os, platform, shutil, subprocess, urllib.request, zipfile
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
NCNN_URL='https://github.com/Tencent/ncnn/releases/download/20260526/ncnn-20260526-full-source.zip'
NCNN_SHA='754659d6fe65545cf2ef4483ffb84526fea631f8764c44b150f1601d0fb4004b'
MODEL_URL='https://github.com/xinntao/Real-ESRGAN/releases/download/v0.2.5.0/realesrgan-ncnn-vulkan-20220424-windows.zip'
MODEL_SHA=''  # Filled from the first official archive inspection before release.
def fetch(url,path,digest):
    if not path.exists():
        with urllib.request.urlopen(url,timeout=90) as r, path.open('wb') as f:shutil.copyfileobj(r,f)
    actual=hashlib.sha256(path.read_bytes()).hexdigest()
    if digest and actual!=digest:raise RuntimeError('Dependency checksum mismatch: '+path.name)
    print(path.name,actual,flush=True)
    return actual
def extract(path,dest):
    dest=dest.resolve()
    with zipfile.ZipFile(path) as z:
        for n in z.namelist():
            if not (dest/n).resolve().is_relative_to(dest):raise RuntimeError('Unsafe archive entry')
        z.extractall(dest)
def main():
    p=argparse.ArgumentParser();p.add_argument('--discover',action='store_true');args=p.parse_args()
    if not MODEL_SHA and not args.discover:raise RuntimeError('Model archive must be checksum-pinned before release')
    work=ROOT/'upscale-build';work.mkdir(exist_ok=True)
    fetch(NCNN_URL,work/'ncnn.zip',NCNN_SHA);model_hash=fetch(MODEL_URL,work/'models.zip',MODEL_SHA)
    extract(work/'ncnn.zip',work/'ncnn-source');extract(work/'models.zip',work/'model-source')
    source=next(p.parent.parent for p in (work/'ncnn-source').rglob('src/net.h'))
    build=work/'build';cmd=['cmake','-S',str(ROOT/'native/upscale'),'-B',str(build),'-DNCNN_SOURCE='+str(source),'-DCMAKE_BUILD_TYPE=Release']
    if platform.system()=='Windows':cmd+=['-A','x64']
    subprocess.run(cmd,check=True);subprocess.run(['cmake','--build',str(build),'--config','Release','--parallel','3'],check=True)
    name='origin0-upscale'+('.exe' if platform.system()=='Windows' else '')
    exe=next(p for p in build.rglob(name) if p.is_file())
    dest=work/'package';dest.mkdir(exist_ok=True);shutil.copy2(exe,dest/name)
    for n in ['realesrgan-x4plus.param','realesrgan-x4plus.bin']:
        shutil.copy2(next((work/'model-source').rglob(n)),dest/n)
    for folder in [source,work/'model-source']:
        for n in ['LICENSE.txt','LICENSE','LICENSE.md']:
            paths=list(folder.rglob(n))
            for i,path in enumerate(paths):shutil.copy2(path,dest/(folder.name+'-'+str(i)+'-'+n))
    target=('windows' if platform.system()=='Windows' else 'linux')+'-amd64'
    archive=ROOT/'bundled'/('origin0-upscale-'+target+'.zip')
    with zipfile.ZipFile(archive,'w',zipfile.ZIP_DEFLATED) as z:
        for path in sorted(dest.iterdir()):z.write(path,path.name)
    spec={'name':archive.name,'size':archive.stat().st_size,'sha256':hashlib.sha256(archive.read_bytes()).hexdigest(),'url':'https://github.com/HUGELU/HASL/releases/download/v1.7.0/'+archive.name,'license':'NCNN BSD-3-Clause; Real-ESRGAN BSD-3-Clause','source':MODEL_URL}
    manifest={'schema':'origin0.upscale.v1','model':'Real-ESRGAN-x4plus','scale':4,'model_archive_sha256':model_hash,'runtimes':{target:spec}}
    (ROOT/'upscale_manifest.json').write_text(json.dumps(manifest,indent=2))
    print(json.dumps(manifest),flush=True)
if __name__=='__main__':main()
