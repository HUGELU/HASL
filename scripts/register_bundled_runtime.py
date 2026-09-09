"""Register the locally built, inspected Windows CPU backend in the manifest."""
import hashlib
import json
from pathlib import Path

root = Path(__file__).resolve().parents[1]
p = root / 'bundled' / 'origin0-sd-d04e895-windows-amd64-cpu.zip'
catalog = json.loads((root / 'model_catalog.json').read_text())
catalog['runtimes']['windows-amd64-cpu'] = {
    'name': p.name, 'size': p.stat().st_size,
    'sha256': hashlib.sha256(p.read_bytes()).hexdigest(),
    'url': 'https://github.com/HUGELU/HASL/releases/download/v1.5.0/' + p.name,
    'license': 'MIT',
    'source': 'https://github.com/HUGELU/HASL/blob/main/scripts/build_native_windows.ps1',
}
(root / 'model_catalog.json').write_text(json.dumps(catalog, indent=2) + '\n')
