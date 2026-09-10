"""Reuse one immutable, hash-pinned native build; never reuse the prior app EXE.

This backend already passed its startup and import gates in run 34326838516.
The later application test setup failed there. If the artifact expires, the
caller falls back to a fresh build from the pinned C++ source.
"""
import hashlib
import io
from pathlib import Path
import subprocess
import zipfile

ROOT = Path(__file__).resolve().parents[1]
ARCHIVE_SHA = 'f5476902616173f278f6399aa8a41e37f345b285ffced944ca494a7a76a3f0b3'
result = subprocess.run(['gh', 'api', 'repos/HUGELU/HASL/actions/artifacts/10094290480/zip'], capture_output=True, check=True)
if hashlib.sha256(result.stdout).hexdigest() != ARCHIVE_SHA:
    raise SystemExit('Cached artifact hash differs; rebuilding the backend')
name = 'origin0-sd-d04e895-windows-amd64-cpu.zip'
with zipfile.ZipFile(io.BytesIO(result.stdout)) as bundle:
    runtime = bundle.read('bundled/' + name)
dest = ROOT / 'bundled' / name
dest.parent.mkdir(exist_ok=True)
dest.write_bytes(runtime)
with zipfile.ZipFile(io.BytesIO(runtime)) as bundle:
    # This zip is transitively covered by ARCHIVE_SHA above.
    bundle.extractall(ROOT / 'native-package')
print('Recovered the exact checked CPU backend; the application will be rebuilt.')
