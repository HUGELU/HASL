"""Local HTTP acceptance check against the compiled host binary; no external model calls."""
import json, os, re, subprocess, tempfile, time, urllib.request, zipfile, io
from pathlib import Path

root = Path(__file__).resolve().parents[1]
base = Path(tempfile.mkdtemp(prefix='origin0_release_validation_'))
exe = root / 'release' / ('ORIGIN0.exe' if os.name == 'nt' else 'origin0-linux')
env = os.environ.copy()
env.update(ORIGIN0_HOME=str(base), ORIGIN0_NO_BROWSER='1', GOMAXPROCS='2')


def start():
    p = subprocess.Popen([str(exe)], env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    for _ in range(20):
        line = p.stdout.readline()
        match = re.search(r'http://(127\.0\.0\.1:\d+)/#([a-f0-9]+)', line)
        if match:
            return p, 'http://' + match[1], match[2]
        if p.poll() is not None:
            raise RuntimeError('Application failed to start: ' + line)
    raise RuntimeError('No startup address')


def call(path, value=None, raw=None):
    headers = {'X-Origin-Key': key}
    data = raw
    if value is not None:
        data = json.dumps(value).encode()
        headers['Content-Type'] = 'application/json'
    req = urllib.request.Request(url + path, data=data, headers=headers)
    with urllib.request.urlopen(req, timeout=10) as r:
        data = r.read()
        return json.loads(data) if r.headers.get('Content-Type', '').startswith('application/json') and data else data


def state_when(pred):
    deadline = time.monotonic() + 12
    while time.monotonic() < deadline:
        state = call('/api/state')
        if pred(state):
            return state
        time.sleep(.15)
    raise AssertionError('Runtime condition did not become true')


def stop():
    call('/api/stop', {})
    proc.wait(timeout=12)
    assert proc.returncode == 0


report = {'version': '1.4-learning', 'platform': os.name, 'checks': []}
proc = None
try:
    proc, url, key = start()
    state_when(lambda s: s['telemetry']['cycles'] >= 10 and len(s['concepts']) > 0)
    report['checks'].append('fresh standalone boot and active cognition')
    html = urllib.request.urlopen(url, timeout=10).read()
    assert b'Voice &amp;' in html or b'Voice & creation' in html
    assert all(urllib.request.urlopen(url + '/' + x, timeout=10).status == 200 for x in ['app.js', 'style.css'])
    report['checks'].append('embedded HTML, JavaScript and CSS served')
    call('/api/observe', {'text': '{"address":"Release test","asking_price":345000,"vacant":true}'})
    s = state_when(lambda s: any(x['source'] == 'observation' for x in s['lab']['discovered']))
    report['checks'].append('explicit observation archived and JSON fields discovered')
    a = call('/api/upload?name=release-note.txt', raw=b'Release acceptance sample. No personal data.')
    assert call('/api/asset?id=' + a['id']) == b'Release acceptance sample. No personal data.'
    report['checks'].append('raw upload and exact download')
    cp = call('/api/snapshot', {'action':'save','name':'Release checkpoint'})
    call('/api/snapshot', {'action':'branch','id':cp['id'],'name':'test-branch'})
    assert call('/api/state')['lab']['branch'] == 'test-branch'
    call('/api/snapshot', {'action':'restore','id':cp['id']})
    assert call('/api/state')['lab']['branch'] == 'main'
    report['checks'].append('snapshot, branch and restore')
    call('/api/settings', {'goal':'Release acceptance: resume this exact priority','track':False})
    call('/api/compute', {'target':15,'paused':True})
    time.sleep(.4)
    first = call('/api/state')['telemetry']['cycles']
    time.sleep(.2)
    assert call('/api/state')['telemetry']['cycles'] == first
    report['checks'].append('pause stops background cognitive cycles')
    with zipfile.ZipFile(io.BytesIO(call('/api/export'))) as z:
        assert z.testzip() is None
        names = z.namelist()
        assert any(x.endswith('.exe') or x == 'origin0-linux' for x in names)
        assert 'origin0_data/state.json' in names and not any('instance.lock' in n for n in names)
    report['checks'].append('runnable branch export')
    stop()
    proc, url, key = start()
    s = state_when(lambda s: s['telemetry']['cycles'] >= first)
    assert s['lab']['goal'] == 'Release acceptance: resume this exact priority'
    assert s['telemetry']['compute_target'] == 15
    assert any(x['id'] == cp['id'] for x in s['lab']['snapshots'])
    assert call('/api/asset?id=' + a['id']) == b'Release acceptance sample. No personal data.'
    report['checks'].append('saved state, preferences, snapshots and objects survive restart')
    duplicate = subprocess.run([str(exe)], env=env, capture_output=True, text=True, timeout=8)
    assert duplicate.returncode == 0 and 'already running' in duplicate.stdout
    report['checks'].append('second instance reuses the existing application')
    report['observed'] = {'version':s['version'],'concepts':s['health']['total_concepts'],'hypotheses':s['health']['total_hypotheses'],'swarms':len(s['swarms']),'ui_candidates':len(s['ui_candidates']),'managed_objects':s['lab']['object_count']}
    stop()
    report['status'] = 'PASS'
finally:
    if proc is not None and proc.poll() is None:
        proc.terminate()
        try: proc.wait(timeout=5)
        except subprocess.TimeoutExpired: proc.kill(); proc.wait()
(root/'release'/'runtime-validation.json').write_text(json.dumps(report,indent=2))
print(json.dumps(report,indent=2))
