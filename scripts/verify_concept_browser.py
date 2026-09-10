"""Release browser gate for the real local app, including keyboard and touch layouts.

Executed by project CI against its own Chromium and isolated application state.
No private user data, GitHub token, external model or network image is submitted.
"""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import time
import urllib.request

from playwright.sync_api import sync_playwright, expect


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--binary', required=True)
    parser.add_argument('--output', default='release/browser-acceptance')
    args = parser.parse_args()
    out = Path(args.output).resolve()
    out.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='origin0-concept-browser-') as base:
        base = Path(base)
        env = dict(os.environ, ORIGIN0_HOME=str(base), ORIGIN0_NO_BROWSER='1', GOMAXPROCS='2')
        log = (base / 'console.log').open('w', encoding='utf-8')
        proc = subprocess.Popen([str(Path(args.binary).resolve())], env=env, stdout=log, stderr=log)
        root = key = None
        report = {'result': 'running', 'checks': [], 'viewports': [], 'console_errors': []}

        def api(path, payload=None):
            data = json.dumps(payload).encode() if payload is not None else None
            request = urllib.request.Request(root + path, data=data, headers={'X-Origin-Key': key, 'Content-Type': 'application/json'})
            with urllib.request.urlopen(request, timeout=25) as response:
                b = response.read()
                return json.loads(b) if b and 'json' in response.headers.get('Content-Type', '') else b

        try:
            for _ in range(150):
                match = re.search(r'(http://127\.0\.0\.1:\d+)/#([a-f0-9]+)', (base / 'console.log').read_text(errors='replace'))
                if match:
                    root, key = match.groups()
                    break
                if proc.poll() is not None:
                    raise RuntimeError('App exited before browser setup')
                time.sleep(.1)
            if not root:
                raise RuntimeError('No application address printed')
            api('/api/compute', {'target': 15})
            with sync_playwright() as pw:
                browser = pw.chromium.launch()
                context = browser.new_context(viewport={'width': 1440, 'height': 1000}, accept_downloads=True)
                page = context.new_page()
                page.on('pageerror', lambda error: report['console_errors'].append(str(error)))
                page.goto(root + '/#' + key)
                page.get_by_role('button', name='Concept Studio', exact=True).click()
                page.get_by_label('Project name', exact=True).fill('Release test: positions')
                page.get_by_label('Subject / intended generation', exact=True).fill('A red square beside a blue circle')
                page.get_by_role('button', name='Save project', exact=True).click()
                expect(page.locator('#concept-select')).not_to_have_value('')
                project = page.locator('#concept-select').input_value()
                report['checks'].append('Created a project through the visible form')
                page.get_by_text('Start with an exact logical exercise', exact=True).click()
                page.get_by_role('button', name='Create examples & evaluate', exact=True).click()
                expect(page.locator('#concept-model-note')).to_contain_text('Active:', timeout=30000)
                expect(page.locator('#concept-examples .concept-example')).to_have_count(12)
                expect(page.locator('#concept-runs')).to_contain_text('Audit:')
                state = api('/api/concepts/state', {})
                model = next(p for p in state['projects'] if p['id'] == project)
                assert len(model['examples']) == 20 and model['active']['id']
                report['classifier'] = model['runs'][-1]
                report['checks'].append('Generated 20 diagrams, trained six candidates and displayed held-out evaluation')
                page.get_by_role('button', name='Next', exact=True).click()
                expect(page.locator('#concept-examples .concept-example')).to_have_count(8)
                page.get_by_role('button', name='Previous', exact=True).click()
                expect(page.locator('#concept-examples .concept-example')).to_have_count(12)
                card = page.locator('#concept-examples .concept-example').first
                card.get_by_label('What is actually visible?', exact=True).fill('A red square on the left side of a light background.')
                card.get_by_role('button', name='Use for training', exact=True).click()
                expect(page.locator('#concept-note')).not_to_be_empty()
                report['checks'].append('Paged the dataset and saved an edited caption')
                page.get_by_text('Position sliders', exact=True).click()
                page.get_by_label('Horizontal placement', exact=True).focus()
                page.get_by_label('Horizontal placement', exact=True).press('Home')
                expect(page.locator('#concept-prompt-preview')).to_contain_text('left side')
                page.get_by_role('button', name='Use prompt in Image studio ↗', exact=True).click()
                expect(page.locator('#native-prompt')).to_have_value(re.compile('left side'))
                page.get_by_role('button', name='Concept Studio', exact=True).click()
                report['checks'].append('Keyboard slider changed the composed image prompt')
                page.get_by_role('button', name='Preview recipe', exact=True).click()
                expect(page.locator('#concept-recipe-preview')).to_contain_text('origin0.concept.v1')
                with page.expect_download() as download:
                    page.get_by_role('button', name='Export training dataset ZIP', exact=True).click()
                dataset = out / 'concept-dataset.zip'
                download.value.save_as(dataset)
                import zipfile
                with zipfile.ZipFile(dataset) as z:
                    assert z.testzip() is None
                    assert 'audit/metadata.jsonl' in z.namelist()
                    assert len([p for p in z.namelist() if p.endswith('.png')]) == 20
                report['checks'].append('Previewed recipe and downloaded a valid split dataset ZIP')
                page.screenshot(path=str(out / 'concept-desktop.png'), full_page=True)
                for width in [280, 320, 390, 768, 1440, 2560]:
                    page.set_viewport_size({'width': width, 'height': 1000})
                    overflow = page.evaluate('document.documentElement.scrollWidth > window.innerWidth + 1')
                    report['viewports'].append({'width': width, 'horizontal_overflow': overflow})
                    assert not overflow, f'Concept Studio overflows at {width}px'
                touch = browser.new_context(viewport={'width': 390, 'height': 844}, has_touch=True, is_mobile=True)
                mobile = touch.new_page()
                mobile.goto(root + '/#' + key)
                mobile.get_by_role('button', name='Concept Studio', exact=True).tap()
                mobile.locator('#concept-select').select_option(project)
                expect(mobile.locator('#concept-examples .concept-example')).to_have_count(12)
                mobile.screenshot(path=str(out / 'concept-mobile.png'), full_page=True)
                report['checks'].append('Six viewport widths had no horizontal page overflow; touch navigation opened the saved dataset')
                assert not report['console_errors'], report['console_errors']
                browser.close()
            report['result'] = 'passed'
        finally:
            (out / 'report.json').write_text(json.dumps(report, indent=2), encoding='utf-8')
            if root and proc.poll() is None:
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
    print(json.dumps(report))


if __name__ == '__main__':
    main()
