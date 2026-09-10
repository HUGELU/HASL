"""Package only after the actual Windows application produced a model-generated PNG."""
import hashlib
import json
from pathlib import Path
import zipfile

ROOT = Path(__file__).resolve().parents[1]
release = ROOT / 'release'
report = json.loads((release / 'windows-acceptance/report.json').read_text())
if report.get('result') != 'passed' or report.get('real_diffusion') is not True:
    raise SystemExit('A real successful image job is required before packaging')
revision = release / 'windows-acceptance/revision.png'
if hashlib.sha256(revision.read_bytes()).hexdigest() != report.get('revision', {}).get('sha256'):
    raise SystemExit('A verified real image-to-image revision is required before packaging')
image = release / 'windows-acceptance/image.png'
if hashlib.sha256(image.read_bytes()).hexdigest() != report['image']['sha256']:
    raise SystemExit('Validation image checksum mismatch')
finished = release / 'windows-acceptance/finished.png'
if not report.get('finishing', {}).get('automatic') or hashlib.sha256(finished.read_bytes()).hexdigest() != report['finishing']['sha256']:
    raise SystemExit('Automatic neural finishing must pass before packaging')
production = json.loads((release / 'windows-production/report.json').read_text())
if production.get('result') != 'passed' or production.get('neural', {}).get('backend') != 'realesrgan-x4plus-cpu' or production.get('fast', {}).get('width') != 7680:
    raise SystemExit('Standalone production tools must pass before packaging')
archive = release / 'ORIGIN0_WINDOWS_v1.7.0.zip' 
files = {
    'ORIGIN0.exe': release / 'ORIGIN0.exe',
    'PRODUCTION_STUDIO.md': ROOT / 'PRODUCTION_STUDIO.md',
    'INTEGRATION_REVIEW.md': ROOT / 'INTEGRATION_REVIEW.md',
    'upscale_manifest.json': ROOT / 'upscale_manifest.json',
    'validation/finished.png': finished,
    'validation/finish-recipe.json': release / 'windows-acceptance/finish-recipe.json',
    'validation/finish.log': release / 'windows-acceptance/finish.log',
    'validation/upscaler-imports.txt': ROOT / 'upscale-build/imports.txt',
    'README_FIRST.txt': ROOT / 'README_FIRST.txt',
    'LICENSE': ROOT / 'LICENSE',
    'RELEASE_NOTES.md': ROOT / 'RELEASE_NOTES.md',
    'CONCEPT_STUDIO.md': ROOT / 'CONCEPT_STUDIO.md',
    'INTERNET_RELAY.md': ROOT / 'INTERNET_RELAY.md',
    'TECHNOLOGY_REPORT.md': ROOT / 'TECHNOLOGY_REPORT.md',
    'MODEL_GUIDE.md': ROOT / 'MODEL_GUIDE.md',
    'LOCAL_MODELS.md': ROOT / 'LOCAL_MODELS.md',
    'worker/requirements.txt': ROOT / 'worker/requirements.txt',
    'worker/local_models.py': ROOT / 'worker/local_models.py',
    'model_catalog.json': ROOT / 'model_catalog.json',
    'validation/report.json': release / 'windows-acceptance/report.json',
    'validation/image.png': image,
    'validation/revision.png': revision,
    'validation/revision.log': release / 'windows-acceptance/revision.log',
    'validation/generation.log': release / 'windows-acceptance/generation.log',
    'validation/runtime-imports.txt': ROOT / 'native-package/imports.txt',
}
for path in (release / 'source-validation/release').rglob('*'):
    if path.is_file():
        files['validation/source/' + path.relative_to(release / 'source-validation/release').as_posix()] = path
for path in (release / 'windows-production').rglob('*'):
    if path.is_file():
        files['validation/production/' + path.relative_to(release / 'windows-production').as_posix()] = path
for path in (ROOT / 'third_party').rglob('*'):
    if path.is_file():
        files[path.relative_to(ROOT).as_posix()] = path
with zipfile.ZipFile(archive, 'w', zipfile.ZIP_DEFLATED, compresslevel=6) as bundle:
    for name, path in sorted(files.items()):
        bundle.write(path, 'ORIGIN0/' + name)
assets = [release / 'ORIGIN0.exe', archive,
          ROOT / 'bundled/origin0-sd-d04e895-windows-amd64-cpu.zip',
          ROOT / 'model_catalog.json', release / 'upscale_manifest.json',
          ROOT / 'bundled/origin0-upscale-windows-amd64.zip',
          ROOT / 'bundled/origin0-upscale-linux-amd64.zip']
(release / 'SHA256SUMS.txt').write_text(''.join(
    hashlib.sha256(p.read_bytes()).hexdigest() + '  ' + p.name + '\n'
    for p in assets), encoding='utf-8')
print(archive)
