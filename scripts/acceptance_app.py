"""Launch this checkout's app with disposable state for release acceptance."""
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import time
import urllib.request


class App:
    def __init__(self, binary):
        self.binary = str(Path(binary).resolve())

    def __enter__(self):
        self.temp = tempfile.TemporaryDirectory(prefix='origin0-production-check-')
        self.home = Path(self.temp.name)
        self.logpath = self.home / 'launch.log'
        self.log = self.logpath.open('w', encoding='utf-8')
        env = dict(os.environ, ORIGIN0_HOME=str(self.home), ORIGIN0_NO_BROWSER='1', GOMAXPROCS='4')
        self.proc = subprocess.Popen([self.binary], env=env, stdout=self.log, stderr=self.log)
        try:
            for _ in range(250):
                match = re.search(r'(http://127\.0\.0\.1:\d+)/#([a-f0-9]+)', self.logpath.read_text(errors='replace'))
                if match:
                    self.root, self.key = match.groups()
                    self.api('/api/compute', {'paused': True})
                    return self
                if self.proc.poll() is not None:
                    raise RuntimeError('Application exited before startup')
                time.sleep(.1)
            raise RuntimeError('Application did not print its local address')
        except BaseException:
            self.__exit__(None, None, None)
            raise

    def api(self, path, body=None, mime=None):
        data = body if isinstance(body, bytes) else json.dumps(body).encode() if body is not None else None
        req = urllib.request.Request(self.root + path, data=data, headers={
            'X-Origin-Key': self.key, 'Content-Type': mime or 'application/json'})
        with urllib.request.urlopen(req, timeout=60) as response:
            data = response.read()
            return json.loads(data) if data and 'json' in response.headers.get('Content-Type', '') else data

    def finish(self, request, timeout=600):
        job = self.api('/api/finish/start', request)
        end = time.monotonic() + timeout
        while time.monotonic() < end:
            state = self.api('/api/finish/state', {})
            current = next(j for j in state['jobs'] if j['id'] == job['id'])
            if current['status'] == 'completed':
                return current
            if current['status'] not in ('queued', 'running'):
                raise RuntimeError(current['message'] + '\n' + current['log'])
            time.sleep(.2)
        raise RuntimeError('Finishing exceeded its validation deadline')

    def __exit__(self, *args):
        if hasattr(self, 'root') and self.proc.poll() is None:
            try:
                self.api('/api/stop', {})
            except Exception:
                pass
        try:
            self.proc.wait(timeout=15)
        except subprocess.TimeoutExpired:
            self.proc.kill()
            self.proc.wait()
        self.log.close()
        self.temp.cleanup()


def pattern_png(width=96, height=64):
    """A deterministic RGB fixture; no external/private photograph or model."""
    import struct
    import zlib
    def chunk(kind, data):
        return struct.pack('>I', len(data)) + kind + data + struct.pack('>I', zlib.crc32(kind + data) & 0xffffffff)
    rows = b''.join(b'\0' + bytes(c for x in range(width) for c in (
        x * 255 // (width - 1), y * 255 // (height - 1), 70 + 100 * (x > width // 2))) for y in range(height))
    return b'\x89PNG\r\n\x1a\n' + chunk(b'IHDR', struct.pack('>IIBBBBB', width, height, 8, 2, 0, 0, 0)) + chunk(b'IDAT', zlib.compress(rows)) + chunk(b'IEND', b'')
