"""Collect this run's completed source/browser evidence for the release package."""
import hashlib
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import zipfile

ROOT = Path(__file__).resolve().parents[1]
repo, run = os.environ['GITHUB_REPOSITORY'], os.environ['GITHUB_RUN_ID']
artifacts = json.loads(subprocess.check_output(['gh', 'api', f'repos/{repo}/actions/runs/{run}/artifacts']))['artifacts']
artifact = next(a for a in artifacts if a['name'] == 'origin0-production-source-validation')
data = subprocess.check_output(['gh', 'api', f'repos/{repo}/actions/artifacts/{artifact["id"]}/zip'])
if 'sha256:' + hashlib.sha256(data).hexdigest() != artifact['digest']:
    raise SystemExit('Source evidence checksum mismatch')
dest = ROOT / 'release/source-validation'
with zipfile.ZipFile(io.BytesIO(data)) as z:
    for name in z.namelist():
        if not (dest / name).resolve().is_relative_to(dest.resolve()):
            raise SystemExit('Source artifact has an invalid path')
    z.extractall(dest)
for folder in ['browser-acceptance', 'production-browser', 'production-acceptance']:
    report = json.loads((dest / 'release' / folder / 'report.json').read_text())
    if report.get('result') != 'passed':
        raise SystemExit('Source acceptance report is incomplete: ' + folder)
if '--common-vision' in sys.argv:
    for folder in ['common-vision-api', 'common-vision-browser']:
        report = json.loads((dest / 'release' / folder / 'report.json').read_text())
        if report.get('result') != 'passed':
            raise SystemExit('Common Vision acceptance failed: ' + folder)
    (dest / 'evidence-source.json').write_text(json.dumps({
        'repository': repo, 'run': run, 'commit': os.environ['GITHUB_SHA'],
        'artifact_id': artifact['id'], 'artifact_sha256': artifact['digest']
    }, indent=2))
    print('Collected this run\'s passed Common Vision and existing browser evidence')
    raise SystemExit(0)
manifest = json.loads((ROOT / 'upscale_manifest.json').read_text())
linux = json.loads((dest / 'upscale_manifest.json').read_text())
spec = linux['runtimes']['linux-amd64']
runtime = dest / 'bundled' / spec['name']
if hashlib.sha256(runtime.read_bytes()).hexdigest() != spec['sha256']:
    raise SystemExit('Linux runtime does not match the tested manifest')
shutil.copy2(runtime, ROOT / 'bundled' / runtime.name)
manifest['runtimes'].update(linux['runtimes'])
(ROOT / 'release/upscale_manifest.json').write_text(json.dumps(manifest, indent=2))
shutil.copy2(ROOT / 'release/windows-production/report.json', ROOT / 'release/production-report.json')
print('Collected this workflow run’s passed browser/source reports and Linux neural runtime')
