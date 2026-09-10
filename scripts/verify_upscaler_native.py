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
