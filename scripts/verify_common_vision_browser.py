"""Actual Chromium UI: contribution -> review -> outcome -> follow-up."""
import argparse,json
from pathlib import Path
from playwright.sync_api import sync_playwright,expect
from acceptance_app import App

def main():
 p=argparse.ArgumentParser();p.add_argument('--binary',required=True);p.add_argument('--output',default='release/common-vision-browser');a=p.parse_args();out=Path(a.output).resolve();out.mkdir(parents=True,exist_ok=True)
 report={'result':'running','checks':[],'viewports':[],'console_errors':[]}
 try:
  with App(a.binary) as app,sync_playwright() as pw:
   browser=pw.chromium.launch();context=browser.new_context(viewport={'width':1440,'height':1000},accept_downloads=True);page=context.new_page();page.on('pageerror',lambda e:report['console_errors'].append(str(e)))
   try:
    page.goto(app.root+'/#'+app.key);expect(page.locator('#cv-status')).to_contain_text('Local workspace ready')
    page.get_by_role('button',name='Load labelled demonstration',exact=True).click();expect(page.locator('#cv-proposals')).to_contain_text('Pilot a public decision response ledger')
    page.get_by_role('button',name='12 + 1 values',exact=True).click();expect(page.locator('.civic-value')).to_have_count(13);expect(page.locator('.plus-one')).to_contain_text('Awaiting original source')
    page.get_by_role('button',name='Living manuscript',exact=True).click();expect(page.locator('.civic-document').first).to_contain_text('digital-only system could exclude')
    page.get_by_role('button',name='Living manifesto',exact=True).click();expect(page.locator('.civic-document').first).not_to_be_empty()
    page.get_by_role('button',name='Implementation programme',exact=True).click();expect(page.locator('.civic-document').first).to_contain_text('Pilot a public decision response ledger')
    page.get_by_role('button',name='Common vision',exact=True).click()
    def decision(button):
     page.get_by_role('button',name=button,exact=True).click();page.locator('#cv-reviewer').fill('Local acceptance reviewer');page.locator('#cv-review-reason').fill('Reviewed this synthetic prototype scenario and its minority objection.');page.locator('#cv-review-confirm').check();page.get_by_role('button',name='Record decision',exact=True).click();expect(page.locator('#cv-review-dialog')).not_to_be_visible()
    decision('Review adoption');expect(page.locator('#cv-framework')).to_contain_text('ADOPTED LOCALLY')
    decision('Review proposal');decision('Record experiment authorisation')
    page.get_by_role('button',name='Record feedback',exact=True).click();expect(page.locator('#cv-outcome-demo')).to_be_checked();page.get_by_role('button',name='Save outcome and update working draft',exact=True).click();expect(page.locator('#cv-outcomes')).to_contain_text('Synthetic pilot')
    page.get_by_role('button',name='Draft follow-up proposal',exact=True).click();expect(page.locator('#cv-proposals article')).to_have_count(2)
    report['checks'].append('Visible demo -> five document views -> human adoption -> proposal review -> authorisation -> simulation -> follow-up')
    page.locator('#cv-actor').fill('browser-bot');page.locator('#cv-source-type').select_option('automated');page.locator('#cv-topic').fill('access');page.locator('#cv-text').fill('An identified automated contribution');page.locator('#cv-context').fill('Synthetic browser validation; not a human participant');page.locator('#cv-consent').check();page.get_by_role('button',name='Save contribution',exact=True).click();expect(page.locator('#cv-topics')).to_contain_text('1 automated')
    with page.expect_download() as dl:page.get_by_role('button',name='Export recovery JSON',exact=True).click()
    exported=Path(dl.value.path()).read_bytes();assert json.loads(exported)['adopted'];(out/'recovery-export.json').write_bytes(exported)
    page.reload();expect(page.locator('#cv-outcomes')).to_contain_text('Synthetic pilot');expect(page.locator('#cv-framework')).to_contain_text('ADOPTED LOCALLY')
    page.evaluate('scrollTo(0,0)');page.screenshot(path=str(out/'common-vision-desktop.png'),full_page=False)
    for width in [280,320,390,768,1440,2560]:
     page.set_viewport_size({'width':width,'height':1000});overflow=page.evaluate('document.documentElement.scrollWidth > innerWidth + 1');report['viewports'].append({'width':width,'horizontal_overflow':overflow});assert not overflow,f'Overflow at {width}'
    mobile_context=browser.new_context(viewport={'width':390,'height':844},has_touch=True,is_mobile=True);mobile=mobile_context.new_page();mobile.goto(app.root+'/#'+app.key);mobile.get_by_role('button',name='12 + 1 values',exact=True).tap();expect(mobile.locator('.plus-one')).to_be_visible();mobile.evaluate('scrollTo(0,0)');mobile.screenshot(path=str(out/'common-vision-mobile.png'),full_page=False)
    report['checks']+=['Automated contribution submitted without human support','Exported JSON and reload retained the adopted snapshot and outcome','Six widths fit; touch can open the distinct +1 framework']
    assert not report['console_errors'];report['result']='passed';browser.close()
   except BaseException:
    try:page.screenshot(path=str(out/'failure.png'),full_page=True);(out/'failure.html').write_text(page.content())
    except Exception:pass
    raise
 finally:(out/'report.json').write_text(json.dumps(report,indent=2))
 print(json.dumps(report))
if __name__=='__main__':main()
