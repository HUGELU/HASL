"""Real app -> verified model setup -> native diffusion -> PNG acceptance test.

No mock model is used. --models reuses an already downloaded, checksum-verified
pack to avoid unnecessary multi-GB transfers. Output contains no session tokens.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import struct
import subprocess
import tempfile
import time
import urllib.request

ROOT = Path(__file__).resolve().parents[1]

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--binary', required=True)
    parser.add_argument('--models')
    parser.add_argument('--runtime-cache')
    parser.add_argument('--output', default='release/image-acceptance')
    parser.add_argument('--timeout', type=int, default=2400)
    args = parser.parse_args()
    output = Path(args.output).resolve()
    output.mkdir(parents=True, exist_ok=True)
    home = Path(tempfile.mkdtemp(prefix='origin0-image-acceptance-'))
    catalog = json.loads((ROOT / 'model_catalog.json').read_text())
    cache = home / 'origin0_data' / 'generator'
    if args.models:
        (cache / 'models').mkdir(parents=True)
        for f in catalog['files']:
            source = Path(args.models).resolve() / f['name']
            dest = cache / 'models' / f['name']
            try:
                os.link(source, dest)
            except OSError:
                shutil.copy2(source, dest)
    if args.runtime_cache:
        (cache / 'downloads').mkdir(parents=True, exist_ok=True)
        for source in Path(args.runtime_cache).glob('*.zip'):
            shutil.copy2(source, cache / 'downloads' / source.name)
    env = os.environ.copy()
    env.update(ORIGIN0_HOME=str(home), ORIGIN0_NO_BROWSER='1')
    logpath = home / 'launch.log'
    log = logpath.open('w', encoding='utf-8')
    proc = subprocess.Popen([str(Path(args.binary).resolve())], env=env, stdout=log, stderr=log)
    deadline = time.monotonic() + args.timeout
    base = key = None

    def api(path, data=None):
        payload = json.dumps(data).encode() if data is not None else None
        req = urllib.request.Request(base + path, data=payload, headers={'X-Origin-Key': key, 'Content-Type': 'application/json'})
        with urllib.request.urlopen(req, timeout=40) as response:
            raw = response.read()
            return json.loads(raw) if raw and 'json' in response.headers.get('Content-Type', '') else raw

    try:
        while time.monotonic() < deadline:
            match = re.search(r'(http://127\.0\.0\.1:\d+)/#([a-f0-9]+)', logpath.read_text(errors='replace'))
            if match:
                base, key = match.groups()
                break
            if proc.poll() is not None:
                raise RuntimeError('Application exited before exposing its interface: ' + logpath.read_text()[-2000:])
            time.sleep(.2)
        if not base:
            raise RuntimeError('No usable local interface address was printed')
        state = api('/api/state')
        assert state['version'].startswith('1.6')
        api('/api/compute', {'paused': True})
        api('/api/images/setup', {'backend': 'cpu', 'threads': min(4, os.cpu_count() or 1), 'max_minutes': 20})
        last = None
        while time.monotonic() < deadline:
            current = api('/api/images/state', {})
            text = current['setup']['message']
            if text != last:
                print(text, flush=True)
                last = text
            if current['ready']:
                break
            if current['setup']['status'] in ('error', 'cancelled'):
                raise RuntimeError(text)
            time.sleep(1)
        else:
            raise RuntimeError('Setup did not finish within the validation deadline')
        start = time.monotonic()
        job = api('/api/images/generate', {'prompt': 'A photograph of an orange tabby cat beside a sunlit window, blue chair, green plants.', 'width': 256, 'height': 256, 'steps': 1, 'seed': 42, 'shared': False})
        while time.monotonic() < deadline:
            current = api('/api/images/state', {})
            found = next(x for x in current['jobs'] if x['id'] == job['id'])
            if found['status'] == 'completed':
                break
            if found['status'] in ('failed', 'cancelled'):
                raise RuntimeError(found['message'] + '\n' + found['log'][-4000:])
            time.sleep(1)
        else:
            raise RuntimeError('Image job did not complete within validation deadline')
        image = api('/api/asset?id=' + found['asset']['id'])
        assert image[:8] == b'\x89PNG\r\n\x1a\n' and struct.unpack('>II', image[16:24]) == (256, 256)
        (output / 'image.png').write_bytes(image)
        (output / 'generation.log').write_text(found['log'], encoding='utf-8')
        report = {'result': 'passed', 'real_diffusion': True, 'version': state['version'], 'platform': current['platform'], 'model_pack': catalog['id'], 'image': {'width': 256, 'height': 256, 'steps': 1, 'seed': 42, 'sha256': hashlib.sha256(image).hexdigest()}, 'elapsed_seconds': round(time.monotonic() - start, 2), 'devices': current['setup']['devices'], 'checks': ['native app startup', 'authenticated API', 'pinned model setup', 'native process execution', 'job completed', 'PNG persisted and downloaded']}
        reference = found['asset']['id']
        start_edit = time.monotonic()
        edit = api('/api/images/generate', {'prompt': 'A photograph of an orange cat on a blue chair in soft warm light.', 'width': 256, 'height': 256, 'steps': 4, 'seed': 43, 'init_asset': reference, 'strength': .5, 'preview': True})
        preview_seen = False
        while time.monotonic() < deadline:
            current = api('/api/images/state', {})
            edited = next(x for x in current['jobs'] if x['id'] == edit['id'])
            if not preview_seen:
                try:
                    preview = api('/api/images/preview', {'id': edit['id']})
                    preview_seen = preview[:8] == b'\x89PNG\r\n\x1a\n'
                except urllib.error.HTTPError as error:
                    if error.code != 404:
                        raise
            if edited['status'] == 'completed':
                break
            if edited['status'] in ('failed', 'cancelled'):
                raise RuntimeError(edited['message'] + '\n' + edited['log'][-4000:])
            time.sleep(1)
        else:
            raise RuntimeError('Reference-image revision exceeded the release deadline')
        revised = api('/api/asset?id=' + edited['asset']['id'])
        assert revised[:8] == b'\x89PNG\r\n\x1a\n' and struct.unpack('>II', revised[16:24]) == (256, 256)
        assert revised != image
        (output / 'revision.png').write_bytes(revised)
        (output / 'revision.log').write_text(edited['log'], encoding='utf-8')
        report['revision'] = {'sha256': hashlib.sha256(revised).hexdigest(), 'elapsed_seconds': round(time.monotonic() - start_edit, 2), 'steps': 4, 'strength': .5, 'preview_observed': preview_seen}
        metadata = api('/api/images/metadata', {'id': edit['id']})
        assert metadata['request']['init_asset'] == reference and metadata['schema'] == 'origin0.image.v1'
        report['checks'] += ['native image-to-image revision', 'changed PNG persisted', 'reference metadata round-trip']
        (output / 'report.json').write_text(json.dumps(report, indent=2), encoding='utf-8')
        print(json.dumps(report), flush=True)
    except Exception:
        print('Application process exit status:', proc.poll(), flush=True)
        launch_text = logpath.read_text(errors='replace')
        launch_text = re.sub(r'(http://127\.0\.0\.1:\d+)/#[a-f0-9]+', r'\1/#[redacted]', launch_text)
        if key:
            launch_text = launch_text.replace(key, '[redacted]')
        (output / 'launch.log').write_text(launch_text, encoding='utf-8')
        print(launch_text[-16000:], flush=True)
        raise
    finally:
        if base and proc.poll() is None:
            try:
                api('/api/stop', {})
            except Exception:
                pass
        try:
            proc.wait(timeout=15)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait()
        log.close()
        shutil.rmtree(home, ignore_errors=True)

if __name__ == '__main__':
    main()
