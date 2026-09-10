"""Reuse only the pinned native build matching this source, or rebuild it.

Reused binaries are tested again through the actual release application.
No downloaded source is executed as part of the fallback; build_upscaler.py
fetches checksum-pinned NCNN source and model weights and compiles this host.
"""
import hashlib
import io
import json
import platform
from pathlib import Path
import subprocess
import sys
import zipfile

ROOT = Path(__file__).resolve().parents[1]


def reuse():
    specs = json.loads((ROOT / 'native/upscale/builds.json').read_text())
    source = b''.join((ROOT / n).read_text(encoding='utf-8').encode('utf-8') for n in specs['source_files'])
    if hashlib.sha256(source).hexdigest() != specs['source_sha256']:
        raise RuntimeError('Native source changed; compile the current source')
    target = ('windows' if platform.system() == 'Windows' else 'linux') + '-amd64'
    spec = specs['artifacts'][target]
    result = subprocess.run(['gh', 'api', 'repos/HUGELU/HASL/actions/artifacts/' + str(spec['id']) + '/zip'], capture_output=True, check=True)
    if hashlib.sha256(result.stdout).hexdigest() != spec['sha256']:
        raise RuntimeError('Native artifact checksum mismatch')
    with zipfile.ZipFile(io.BytesIO(result.stdout)) as z:
        manifest = json.loads(z.read('upscale_manifest.json'))
        runtime = manifest['runtimes'][target]
        data = z.read('bundled/' + runtime['name'])
    if hashlib.sha256(data).hexdigest() != runtime['sha256'] or len(data) != runtime['size']:
        raise RuntimeError('Runtime checksum mismatch')
    dest = ROOT / 'bundled' / runtime['name']
    dest.parent.mkdir(exist_ok=True)
    dest.write_bytes(data)
    (ROOT / 'upscale_manifest.json').write_text(json.dumps(manifest, indent=2))
    with zipfile.ZipFile(io.BytesIO(data)) as z:
        dest = ROOT / 'upscale-build/package'
        for name in z.namelist():
            if not (dest / name).resolve().is_relative_to(dest.resolve()):
                raise RuntimeError('Invalid native archive path')
        z.extractall(dest)
    if platform.system() != 'Windows':
        (dest / 'origin0-upscale').chmod(0o755)
    print('Recovered hash-pinned neural runtime for', target)


if __name__ == '__main__':
    try:
        reuse()
    except Exception as error:
        print('Rebuilding neural runtime:', type(error).__name__, flush=True)
        subprocess.run([sys.executable, str(ROOT / 'scripts/build_upscaler.py')], check=True)
