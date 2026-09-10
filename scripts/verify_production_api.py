"""Actual executable, learned weights, 8K finish, recipes and property exports."""
import argparse
import hashlib
import json
from pathlib import Path
import struct
import urllib.error
from acceptance_app import App, pattern_png


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--binary', required=True)
    p.add_argument('--output', default='release/production-acceptance')
    args = p.parse_args()
    out = Path(args.output).resolve()
    out.mkdir(parents=True, exist_ok=True)
    report = {'result': 'running', 'checks': []}
    try:
        with App(args.binary) as app:
            report['version'] = app.api('/api/state')['version']
            original = pattern_png()
            source = app.api('/api/upload?name=pattern.png', original, 'image/png')
            (out / 'original.png').write_bytes(original)
            for mode, size in [('fast', 7680), ('neural', 1024)]:
                request = {'asset': source['id'], 'mode': mode, 'long_edge': size,
                           'sharpness': .1, 'detail': .8, 'backend': 'cpu'}
                job = app.finish(request)
                image = app.api('/api/asset?id=' + job['asset']['id'])
                width, height = struct.unpack('>II', image[16:24])
                assert width == size and height == round(size * 64 / 96)
                assert image[:8] == b'\x89PNG\r\n\x1a\n'
                recipe = app.api('/api/asset?id=' + job['recipe']['id'])
                if isinstance(recipe, bytes):
                    recipe = json.loads(recipe)
                assert recipe['source'] == source['id'] and recipe['result'] == hashlib.sha256(image).hexdigest()
                if mode == 'neural':
                    assert job['backend'] == 'realesrgan-x4plus-cpu'
                    assert 'tile=' in job['log']
                (out / (mode + '.png')).write_bytes(image)
                (out / (mode + '-recipe.json')).write_text(json.dumps(recipe, indent=2))
                (out / (mode + '.log')).write_text(job['log'])
                report[mode] = {'width': width, 'height': height, 'seconds': job['seconds'],
                                'backend': job['backend'], 'sha256': hashlib.sha256(image).hexdigest()}
                if mode == 'fast':
                    cached = app.finish(request)
                    assert cached['cached'] and cached['asset']['id'] == job['asset']['id']
                    report['checks'].append('Identical finish reused its checksummed output')
            assert app.api('/api/asset?id=' + source['id']) == original
            report['checks'].extend(['8K-long-edge fast finish', 'Actual NCNN CPU neural inference',
                                     'Exact finishing recipes downloaded', 'Original bytes unchanged'])
            project = {'schema': 'origin0.architecture.v1', 'name': 'Acceptance property',
                       'rooms': [{'name': 'Living', 'x': 0, 'y': 0, 'w': 5, 'h': 4,
                                  'door': 'bottom', 'door_offset': 1, 'door_width': .9},
                                 {'name': 'Kitchen', 'x': 5, 'y': 0, 'w': 3, 'h': 4}],
                       'views': [{'asset': source['id'], 'label': label, 'seconds': 2} for label in ['Arrival', 'Interior']]}
            plan = app.api('/api/architecture/plan', project)
            assert plan['room_area'] == 32
            svg = app.api('/api/asset?id=' + plan['svg']['id'])
            assert b'5.00' in svg and b'Kitchen' in svg
            (out / 'plan.svg').write_bytes(svg)
            walkthrough = app.api('/api/architecture/walkthrough', project)
            html = app.api('/api/asset?id=' + walkthrough['html']['id'])
            assert b'data:image/png;base64,' in html and b'MediaRecorder' in html
            (out / 'walkthrough.html').write_bytes(html)
            assert app.api('/api/architecture/project', {})['views'] == project['views']
            report['checks'].extend(['Dimensioned SVG with 32 m² room area', 'Self-contained photo walkthrough', 'Property project persisted'])
            report['result'] = 'passed'
    finally:
        (out / 'report.json').write_text(json.dumps(report, indent=2))
    print(json.dumps(report))


if __name__ == '__main__':
    main()
