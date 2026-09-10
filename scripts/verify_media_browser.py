"""Exercise the actual embedded upstream studio; no neural quality claim from UI checks."""
import argparse,json
from pathlib import Path
from acceptance_app import App
from playwright.sync_api import sync_playwright,expect

def main():
 p=argparse.ArgumentParser();p.add_argument('--binary',required=True);p.add_argument('--output',default='release/media-browser');a=p.parse_args();out=Path(a.output);out.mkdir(parents=True,exist_ok=True)
 report={'result':'failed','checks':[],'errors':[],'viewports':[]}
 try:
  with App(a.binary) as app,sync_playwright() as pw:
   browser=pw.chromium.launch();context=browser.new_context(viewport={'width':1440,'height':1050},record_video_dir=str(out/'video'));page=context.new_page();page.on('pageerror',lambda e:report['errors'].append(str(e)))
   page.goto(app.root+'/#'+app.key);frame=page.frame_locator('#open-studio-frame');expect(frame.get_by_role('button',name='Create image',exact=True)).to_be_visible();expect(frame.get_by_label('Image prompt',exact=True)).to_be_visible()
   frame.get_by_label('Image prompt',exact=True).fill('A timber pavilion beside a lake, daylight, architectural photograph')
   frame.get_by_role('button',name='Models & engines',exact=True).click();expect(frame.locator('[data-model]')).to_have_count(7);expect(frame.locator('[data-model="flux2-klein"]')).to_contain_text('16.13 GB');page.screenshot(path=str(out/'local-model-catalogue.png'))
   frame.get_by_label('Search local models').fill('SDXL');expect(frame.locator('[data-model]')).to_have_count(2);frame.get_by_label('Search local models').fill('')
   frame.get_by_role('button',name='White label',exact=True).click();frame.locator('#brand-name').fill('Pavilion Studio');frame.locator('#brand-tagline').fill('Architecture, locally');frame.locator('#brand-accent').fill('#55aacc');frame.get_by_role('button',name='Save branding',exact=True).click();expect(frame.locator('#origin-name')).to_have_text('Pavilion Studio');expect(page.locator('.brand')).to_contain_text('Pavilion Studio')
   page.reload();frame=page.frame_locator('#open-studio-frame');expect(frame.locator('#origin-name')).to_have_text('Pavilion Studio');expect(frame.get_by_label('Image prompt',exact=True)).to_be_visible()
   frame.get_by_role('button',name='Cinema studio',exact=True).click();expect(frame.locator('body')).to_contain_text('What would you shoot');page.screenshot(path=str(out/'cinema-studio.png'))
   frame.get_by_role('button',name='Video & workflows',exact=True).click();frame.locator('#workflow-json').fill('{"nodes":[]}');frame.get_by_role('button',name='Validate and queue workflow',exact=True).click();expect(frame.locator('#origin-notice')).to_contain_text('API')
   for width in [320,390,768,1440,2560]:
    page.set_viewport_size({'width':width,'height':1000});frame.get_by_role('button',name='Models & engines',exact=True).click();expect(frame.locator('[data-model]')).to_have_count(7);overflow=frame.locator('body').evaluate('(el)=>document.documentElement.scrollWidth>innerWidth+1');report['viewports'].append({'width':width,'horizontal_overflow':overflow});assert not overflow
   page.set_viewport_size({'width':390,'height':844});page.screenshot(path=str(out/'models-mobile.png'))
   page.get_by_role('button',name='Common Vision',exact=True).click();expect(page.locator('#cv-status')).to_contain_text('Local workspace ready')
   report['checks']=['Upstream image studio mounted with local session; no hosted API-key gate','Seven pinned model cards and live filtering','White-label name, tagline and colour saved and survived reload','Cinema component rendered','Invalid visual workflow rejected with an actionable API-format error','Five viewport widths fit; existing Common Vision still opens']
   assert not report['errors'],report['errors'];report['result']='passed';context.close();browser.close()
 except BaseException:
  try:page.screenshot(path=str(out/'failure.png'));(out/'failure.html').write_text(page.content(),encoding='utf-8')
  except Exception:pass
  raise
 finally:(out/'report.json').write_text(json.dumps(report,indent=2))
 print(json.dumps(report))
if __name__=='__main__':main()
