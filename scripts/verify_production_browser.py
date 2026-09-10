"""Exercise the visible production workflow in the project's CI Chromium."""
import argparse
import json
from pathlib import Path
import re
import struct
from playwright.sync_api import sync_playwright, expect
from acceptance_app import App, pattern_png


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--binary', required=True)
    p.add_argument('--output', default='release/production-browser')
    args = p.parse_args()
    out = Path(args.output).resolve()
    out.mkdir(parents=True, exist_ok=True)
    report = {'result': 'running', 'checks': [], 'viewports': [], 'console_errors': []}
    try:
        with App(args.binary) as app, sync_playwright() as pw:
            browser = pw.chromium.launch()
            context = browser.new_context(viewport={'width': 1440, 'height': 1000}, accept_downloads=True)
            page = context.new_page()
            try:
                page.on('pageerror', lambda e: report['console_errors'].append(str(e)))
                page.goto(app.root + '/#' + app.key)
                page.get_by_role('button', name='Production', exact=True).click()
                expect(page.locator('#development-history')).to_contain_text('No coding runs yet')
                page.locator('#finish-file').set_input_files({'name': 'pattern.png', 'mimeType': 'image/png', 'buffer': pattern_png()})
                expect(page.locator('#finish-source-label')).to_have_text('pattern.png')
                page.locator('#finish-mode').select_option('fast')
                page.locator('#finish-size').select_option('1024')
                page.locator('#finish-form button[type=submit]').click()
                expect(page.locator('#finish-history')).to_contain_text('completed', timeout=60000)
                with page.expect_download() as dl:
                    page.locator('#finish-history').get_by_role('button', name='Download PNG', exact=True).first.click()
                image = Path(dl.value.path()).read_bytes()
                assert struct.unpack('>II', image[16:24]) == (1024, 683)
                (out / 'browser-finished.png').write_bytes(image)
                page.get_by_role('button', name='Compare original', exact=True).first.click()
                expect(page.locator('#finish-compare')).to_be_visible()
                report['checks'].append('Upload, queued finishing, real PNG download and original comparison')
                page.locator('#finish-automatic').check()
                values = page.evaluate('window.studioRequestValues()')
                assert values['finish']['mode'] == 'fast' and values['finish']['long_edge'] == 1024
                report['checks'].append('Automatic finishing controls attach the requested recipe to new image jobs')
                page.locator('#architecture-name').fill('Release test property')
                page.get_by_role('button', name='Add room', exact=True).click()
                page.get_by_role('button', name='Add room', exact=True).click()
                rooms = page.locator('[data-room]')
                rooms.nth(0).get_by_label('Room name', exact=True).fill('Living')
                rooms.nth(0).get_by_label('Door wall', exact=True).select_option('bottom')
                rooms.nth(1).get_by_label('Room name', exact=True).fill('Kitchen')
                page.get_by_role('button', name='Create plan', exact=True).click()
                expect(page.locator('#architecture-plan-output')).to_contain_text('24.00 m²')
                expect(page.locator('#architecture-plan-preview')).to_be_visible()
                with page.expect_download() as dl:
                    page.get_by_role('button', name='Download SVG plan', exact=True).click()
                svg = Path(dl.value.path()).read_text()
                assert 'Living' in svg and 'Kitchen' in svg and '4.00 × 3.00' in svg
                (out / 'browser-plan.svg').write_text(svg)
                report['checks'].append('Two measured rooms, door placement and downloaded dimensioned SVG')
                page.locator('#architecture-files').set_input_files([
                    {'name': 'arrival.png', 'mimeType': 'image/png', 'buffer': pattern_png()},
                    {'name': 'interior.png', 'mimeType': 'image/png', 'buffer': pattern_png(100, 72)}])
                expect(page.locator('[data-view]')).to_have_count(2)
                for index in [0, 1]:
                    page.locator('[data-view]').nth(index).get_by_label('Seconds', exact=True).fill('2')
                page.locator('[data-view]').nth(0).get_by_label('Room', exact=True).select_option('Living')
                page.locator('[data-view]').nth(1).get_by_role('button', name='Move earlier', exact=True).click()
                page.locator('[data-view]').nth(0).get_by_role('button', name='Render this reference', exact=True).click()
                expect(page.locator('#native-prompt')).to_have_value(re.compile('Architectural photograph'))
                expect(page.locator('#native-reference')).not_to_have_value('')
                expect(page.locator('#native-strength')).to_have_value('0.25')
                page.get_by_role('button', name='Production', exact=True).click()
                report['checks'].append('Reference upload, room association, shot ordering and native image-revision handoff')
                with page.expect_download() as dl:
                    page.get_by_role('button', name='Save property project', exact=True).click()
                saved = Path(dl.value.path()).read_bytes()
                assert json.loads(saved)['views'][1]['room'] == 'Living'
                page.locator('#architecture-import').set_input_files({'name': 'property.json', 'mimeType': 'application/json', 'buffer': saved})
                page.get_by_role('button', name='Create photo walkthrough', exact=True).click()
                expect(page.locator('#architecture-walkthrough-preview')).to_be_visible(timeout=30000)
                frame = page.frame_locator('#architecture-walkthrough-preview')
                expect(frame.locator('#status')).to_contain_text('Ready')
                with page.expect_download(timeout=45000) as dl:
                    frame.get_by_role('button', name='Export WebM video', exact=True).click()
                video = out / 'walkthrough.webm'
                dl.value.save_as(video)
                assert video.stat().st_size > 5000 and video.read_bytes()[:4] == b'\x1aE\xdf\xa3'
                report['video'] = {'bytes': video.stat().st_size, 'format': 'WebM VP9', 'duration_requested_seconds': 4, 'canvas': '1920x1080'}
                with page.expect_download() as dl:
                    page.get_by_role('button', name='Download standalone walkthrough', exact=True).click()
                page_file = out / 'walkthrough.html'
                dl.value.save_as(page_file)
                assert 'data:image/png;base64,' in page_file.read_text()
                report['checks'].append('Saved and restored project; rendered and downloaded an actual WebM video')
                page.locator('#development-goal').fill('Preserve numerical correctness')
                page.get_by_role('button', name='Start local coding run', exact=True).click()
                expect(page.locator('#toast')).to_contain_text('connect', ignore_case=True)
                report['checks'].append('Unconfigured local coding action gave a concrete connection error')
                page.screenshot(path=str(out / 'production-desktop.png'), full_page=True)
                for width in [280, 320, 390, 768, 1440, 2560]:
                    page.set_viewport_size({'width': width, 'height': 1000})
                    overflow = page.evaluate('document.documentElement.scrollWidth > innerWidth + 1')
                    report['viewports'].append({'width': width, 'horizontal_overflow': overflow})
                    assert not overflow, f'Production page overflows at {width}px'
                mobile_context = browser.new_context(viewport={'width': 390, 'height': 844}, has_touch=True, is_mobile=True)
                mobile = mobile_context.new_page()
                mobile.goto(app.root + '/#' + app.key)
                mobile.get_by_role('button', name='Production', exact=True).tap()
                expect(mobile.locator('[data-room]')).to_have_count(2)
                expect(mobile.locator('[data-view]')).to_have_count(2)
                mobile.get_by_role('button', name='Create plan', exact=True).tap()
                expect(mobile.locator('#architecture-plan-preview')).to_be_visible()
                mobile.screenshot(path=str(out / 'production-mobile.png'), full_page=True)
                report['checks'].append('Saved property opened on touch viewport; six widths had no page overflow')
                assert not report['console_errors'], report['console_errors']
                browser.close()
            except BaseException:
                try:
                    page.screenshot(path=str(out / 'failure.png'), full_page=True)
                    (out / 'failure.html').write_text(page.content())
                except Exception:
                    pass
                raise

        report['result'] = 'passed'
    finally:
        (out / 'report.json').write_text(json.dumps(report, indent=2))
    print(json.dumps(report))


if __name__ == '__main__':
    main()
