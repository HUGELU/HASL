"""Actual executable: persistence, human decisions and outcome-feedback cycle."""
import argparse, json, urllib.error
from pathlib import Path
from acceptance_app import App

def main():
 p=argparse.ArgumentParser();p.add_argument('--binary',required=True);p.add_argument('--output',default='release/common-vision-api');a=p.parse_args();out=Path(a.output).resolve();out.mkdir(parents=True,exist_ok=True)
 report={'result':'running','checks':[]}
 try:
  with App(a.binary) as app:
   report['app_version']=app.api('/api/state')['version']
   def state():return app.api('/api/vision/state',{})['state']
   def act(action,**kw):
    return app.api('/api/vision/action',dict(action=action,revision=state()['revision'],**kw))
   def review(action,target):return act(action,id=target,reviewer='Acceptance human',reason='Local demonstration reviewed; no external action authorised.',human=True,confirm=True)
   s=state();assert len(s['values'])==13 and s['values'][-1]['plus_one'] and not any(v['original'] for v in s['values'])
   report['checks'].append('Original values remain pending; distinct +1 retained')
   act('demo');s=state();draft=s['draft'];review('adopt-draft',draft);adopted=next(f for f in state()['frameworks'] if f['id']==draft)
   proposal=s['proposals'][0]['id']
   try:
    act('authorise-experiment',id=proposal,reviewer='test',reason='premature',human=True,confirm=True)
    raise AssertionError('Unreviewed proposal authorised')
   except urllib.error.HTTPError as error:assert error.code==400
   review('approve-proposal',proposal);review('authorise-experiment',proposal)
   act('outcome',outcome=dict(proposal=proposal,kind='simulation',result='Synthetic response share 4/10 to 6/10',measure='10 invented submissions',source='Acceptance fixture',limitations='No real participants or causal evidence',next='Test an offline route and a workload cap',demo=True))
   outcome=state()['outcomes'][0];act('follow-up',id=outcome['id']);s=state();assert len(s['proposals'])==2 and s['proposals'][1]['status']=='proposed'
   assert next(f for f in s['frameworks'] if f['id']==draft)==adopted
   review('reject-draft',s['draft']);assert state()['adopted']==draft
   report['checks']+=['Review and authorisation are separate','Simulation feedback created a reviewable follow-up','Adopted snapshot unchanged by outcomes or rejected drafts']
   for i in range(4):
    act('contribute',contribution=dict(actor='test-bot',source_type='automated',kind='opinion',topic='access',text='Bot analysis '+str(i),language='en',context='identified automated fixture',stance='support',importance=3,urgency=3,consequence=3,consent=True))
   topics=app.api('/api/vision/state',{})['topics'];assert all(t['human_records']==0 and t['support']==0 for t in topics)
   assert next(t for t in topics if t['topic']=='access')['automated_records']==1
   report['checks'].append('Bot repetition and demonstration inputs did not become human support')
   exported=app.api('/api/vision/export',{});assert exported['schema']=='origin0.common-vision.v1'
   (out/'demonstration-state.json').write_text(json.dumps(exported,indent=2),encoding='utf-8')
   on_disk=json.loads((app.home/'origin0_data/common-vision.json').read_text());assert on_disk==exported
   assert app.api('/api/vision/state',{'since':exported['revision']})=={'unchanged':True}
   report['checks'].append('Recovery export exactly matches durable state; unchanged polling is bounded')
   report['result']='passed'
 finally:(out/'report.json').write_text(json.dumps(report,indent=2))
 print(json.dumps(report))
if __name__=='__main__':main()
