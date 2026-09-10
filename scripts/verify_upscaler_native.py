"""Run real x4plus weights on CPU; no image-quality ranking is implied."""
import hashlib,json,platform,subprocess,time
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
work=ROOT/'upscale-build';dest=work/'acceptance';dest.mkdir(exist_ok=True)
package=work/'package';name='origin0-upscale'+('.exe' if platform.system()=='Windows' else '')
w,h=24,20
pixels=bytes(c for y in range(h) for x in range(w) for c in (x*255//(w-1),y*255//(h-1),60+int(x>w//2)*130))
(dest/'input.rgb').write_bytes(pixels)
start=time.monotonic()
cmd=[str(package/name),str(dest/'input.rgb'),str(dest/'output.rgb'),str(w),str(h),str(package/'realesrgan-x4plus.param'),str(package/'realesrgan-x4plus.bin'),'32','2','cpu']
r=subprocess.run(cmd,check=True,capture_output=True,text=True,timeout=600)
out=(dest/'output.rgb').read_bytes();assert len(out)==w*h*3*16;assert len(set(out))>32
assert 'backend=cpu' in r.stdout and 'tile=1/1' in r.stdout
report={'result':'passed','real_neural_inference':True,'model':'Real-ESRGAN-x4plus','backend':'cpu','width':w*4,'height':h*4,'seconds':round(time.monotonic()-start,3),'sha256':hashlib.sha256(out).hexdigest()}
(dest/'report.json').write_text(json.dumps(report,indent=2));(dest/'runtime.log').write_text(r.stdout+'\n'+r.stderr)
print(json.dumps(report),flush=True)

# Multiple independent CPU tiles must produce the exact sequential output.
w,h=96,80
pixels=bytes(c for y in range(h) for x in range(w) for c in (x*255//(w-1),y*255//(h-1),60+int(x>w//2)*130))
(dest/'parallel-input.rgb').write_bytes(pixels)
outputs=[];timings=[]
for threads in [1,3]:
    target=dest/('parallel-'+str(threads)+'.rgb')
    start=time.monotonic()
    result=subprocess.run([str(package/name),str(dest/'parallel-input.rgb'),str(target),str(w),str(h),str(package/'realesrgan-x4plus.param'),str(package/'realesrgan-x4plus.bin'),'64',str(threads),'cpu'],check=True,capture_output=True,text=True,timeout=600)
    outputs.append(target.read_bytes());timings.append(round(time.monotonic()-start,3))
    assert 'workers='+str(threads) in result.stdout and 'tile=4/4' in result.stdout
    (dest/('parallel-'+str(threads)+'.log')).write_text(result.stdout+'\n'+result.stderr)
assert outputs[0]==outputs[1] and len(outputs[0])==w*h*3*16
report['parallel']={'sequential_seconds':timings[0],'three_workers_seconds':timings[1],'identical_pixels':True,'width':w*4,'height':h*4}
(dest/'report.json').write_text(json.dumps(report,indent=2));print(json.dumps(report),flush=True)
