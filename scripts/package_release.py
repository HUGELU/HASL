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
image = release / 'windows-acceptance/image.png'
if hashlib.sha256(image.read_bytes()).hexdigest() != report['image']['sha256']:
    raise SystemExit('Validation image checksum mismatch')
archive = release / 'ORIGIN0_WINDOWS_v1.6.zip'
files = {
    'ORIGIN0.exe': release / 'ORIGIN0.exe',
    'README_FIRST.txt': ROOT / 'README_FIRST.txt',
    'LICENSE': ROOT / 'LICENSE',
    'RELEASE_NOTES.md': ROOT / 'RELEASE_NOTES.md',
    'CONCEPT_STUDIO.md': ROOT / 'CONCEPT_STUDIO.md',
    'INTERNET_RELAY.md': ROOT / 'INTERNET_RELAY.md',
    'TECHNOLOGY_REPORT.md': ROOT / 'TECHNOLOGY_REPORT.md',
    'model_catalog.json': ROOT / 'model_catalog.json',
    'validation/report.json': release / 'windows-acceptance/report.json',
    'validation/image.png': image,
    'validation/generation.log': release / 'windows-acceptance/generation.log',
    'validation/runtime-imports.txt': ROOT / 'native-package/imports.txt',
}
for path in (ROOT / 'third_party').rglob('*'):
    if path.is_file():
        files[path.relative_to(ROOT).as_posix()] = path
with zipfile.ZipFile(archive, 'w', zipfile.ZIP_DEFLATED, compresslevel=6) as bundle:
    for name, path in sorted(files.items()):
        bundle.write(path, 'ORIGIN0/' + name)
assets = [release / 'ORIGIN0.exe', archive,
          ROOT / 'bundled/origin0-sd-d04e895-windows-amd64-cpu.zip',
          ROOT / 'model_catalog.json']
(release / 'SHA256SUMS.txt').write_text(''.join(
    hashlib.sha256(p.read_bytes()).hexdigest() + '  ' + p.name + '\n'
    for p in assets), encoding='utf-8')
print(archive)
