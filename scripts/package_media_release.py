"""Package the exact passed Windows executable and this run's evidence. Never rebuild or replace old releases."""
import hashlib,io,json,os,subprocess,zipfile
from pathlib import Path

root=Path(__file__).resolve().parents[1];out=root/'release';out.mkdir(exist_ok=True)
repo=os.environ['GITHUB_REPOSITORY'];run=os.environ['GITHUB_RUN_ID'];sha=os.environ['GITHUB_SHA']
artifacts=json.loads(subprocess.check_output(['gh','api',f'repos/{repo}/actions/runs/{run}/artifacts']))['artifacts']
evidence=out/'media-release-evidence';evidence.mkdir(exist_ok=True)
inventory=[]
for name in ['origin0-production-source-validation','origin0-open-studio-windows']:
 a=next(a for a in artifacts if a['name']==name);b=subprocess.check_output(['gh','api',f'repos/{repo}/actions/artifacts/{a["id"]}/zip']);assert a['digest']=='sha256:'+hashlib.sha256(b).hexdigest()
 dest=evidence/name;dest.mkdir(exist_ok=True)
 with zipfile.ZipFile(io.BytesIO(b)) as z:
  for n in z.namelist():
   if not (dest/n).resolve().is_relative_to(dest.resolve()):raise RuntimeError('Unsafe artifact path')
  z.extractall(dest)
 inventory.append({'artifact':name,'id':a['id'],'sha256':a['digest']})
reports={}
for folder in ['browser-acceptance','production-acceptance','production-browser','common-vision-api','common-vision-browser','media-browser','media-windows','common-vision-windows']:
 matches=list(evidence.rglob(folder+'/report.json'));assert len(matches)==1,(folder,matches);r=json.loads(matches[0].read_text());assert r['result']=='passed',(folder,r);reports[folder]=r
exe=next((evidence/'origin0-open-studio-windows').rglob('ORIGIN0.exe'));(out/'ORIGIN0.exe').write_bytes(exe.read_bytes())
manifest={'repository':repo,'branch':'origin0-media-studio','source_commit':sha,'run':int(run),'artifacts':inventory,'reports':reports,'exe_sha256':hashlib.sha256(exe.read_bytes()).hexdigest()}
(out/'MEDIA_VALIDATION.json').write_text(json.dumps(manifest,indent=2))
instructions='''ORIGIN-0 OPEN CREATIVE STUDIO v1.8.0 — WINDOWS PREVIEW

1. Extract the ZIP to a normal writable folder and double-click ORIGIN0.exe.
2. Keep its console open. Your browser opens the local workspace.
3. In Creative studio, open Models & engines.
4. For the quickest setup path on a 32 GB laptop, download the native
   Z-Image-Turbo pack. For other catalogue models, install portable ComfyUI
   and download that model's complete pack. Downloads show actual progress.
5. Select Use model and create an image. Use Gallery & queue for saved outputs,
   parameters and finishing. White label changes your studio's branding.

No separate Python installation or generation subscription is needed for these
local paths. ComfyUI includes its own runtime. Model downloads require several
gigabytes; larger models need substantially more RAM/VRAM. First downloads and
CPU generation take time. The package does not contain model weights or claim
instant inference on every laptop. Model and engine licences remain applicable.

Existing Common Vision, production, architecture, learning, worker-PC and PIN
features remain in the workspace tabs. Keep the existing origin0_data folder.
Do not replace it with an empty or older data folder. Stop the earlier ORIGIN
process before launching this new version on the same data.

Read MEDIA_STUDIO.md and MEDIA_VALIDATION.json for tested capabilities and limits.
ComfyUI startup errors are logged in origin0_data/media-studio/comfy-runtime.log.
'''
(out/'README_FIRST_v1.8.0.txt').write_text(instructions)
recovery=f'''# Open Studio release checkpoint — v1.8.0

PR: https://github.com/{repo}/pull/6
Branch: origin0-media-studio
Tested source commit: {sha}
Validation: https://github.com/{repo}/actions/runs/{run}
Executable SHA-256: {manifest['exe_sha256']}

Completed: embedded adapted Open-Higgsfield-AI image/reference/cinema components;
seven pinned downloadable model packs; native and ComfyUI generation adapters;
official portable Windows ComfyUI setup; complete workflow editor access and API
workflow submission; saved gallery/parameters; configurable white-label branding;
existing Common Vision, privacy, learning and production tools retained.

Actual gates: full source race suite/vet, restricted-container candidate test,
retained production/Concept/Common Vision API and browser checks, new Open Studio
browser checks, Windows Go suite/vet, actual native image, portable ComfyUI setup,
ComfyUI text-to-image and reference-image generation. Detailed outputs, timings,
logs, screenshots and browser recordings are included in validation/. These are
functional acceptance tests, not evidence of better image quality than any
commercial model. The Windows test is no longer pending for this tested commit.

Remaining: benchmark Intel GPU execution on the owner's Galaxy Book; validate
the other large packs with appropriate hardware; add curated video/multi-reference
recipes to the simple UI while retaining the full workflow editor; review outputs
and model licences for each intended use. Original 12 + 1 values remain pending
the owner's source material. No values or consequential actions are auto-adopted.

Restore: preserve the full origin0_data directory. Media studio configuration and
job metadata are in media-studio/studio.json; source assets remain in the object
store. The tagged source and this ZIP identify the exact tested version. Earlier
v1.7.1 releases and the pre-Common-Vision recovery branch remain unchanged.
'''
(out/'MEDIA_RECOVERY.md').write_text(recovery)
files={'ORIGIN0.exe':out/'ORIGIN0.exe','README_FIRST.txt':out/'README_FIRST_v1.8.0.txt','MEDIA_VALIDATION.json':out/'MEDIA_VALIDATION.json','MEDIA_RECOVERY.md':out/'MEDIA_RECOVERY.md'}
for name in ['MEDIA_STUDIO.md','COMMON_VISION.md','PRODUCTION_STUDIO.md','MODEL_GUIDE.md','LICENSE','studio_catalog.json','model_catalog.json','comfy_runtimes.json','upscale_manifest.json']:
 files[name]=root/name
for p in (root/'third_party').rglob('*'):
 if p.is_file():files[str(p.relative_to(root))]=p
for p in evidence.rglob('*'):
 if p.is_file() and not p.name.endswith(('.zip','.exe')):files['validation/'+str(p.relative_to(evidence))]=p
package=out/'ORIGIN0_OPEN_STUDIO_WINDOWS_v1.8.0.zip'
with zipfile.ZipFile(package,'w',zipfile.ZIP_DEFLATED) as z:
 for name,p in files.items():z.write(p,name)
(out/'OPEN_STUDIO_SHA256SUMS.txt').write_text(''.join(hashlib.sha256(p.read_bytes()).hexdigest()+'  '+p.name+'\n' for p in [out/'ORIGIN0.exe',package,out/'MEDIA_VALIDATION.json',out/'MEDIA_RECOVERY.md']))
print(json.dumps({'package':str(package),'source_commit':sha,'exe_sha256':manifest['exe_sha256'],'reports':list(reports)}))
