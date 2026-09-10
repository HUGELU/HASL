"""Package passed Windows + browser evidence without repeating old media claims."""
import hashlib,json,os,zipfile
from pathlib import Path
root=Path(__file__).resolve().parents[1];release=root/'release'
report=json.loads((release/'common-vision-windows/report.json').read_text());assert report['result']=='passed' and report['app_version']=='1.7.1-common-vision'
evidence=release/'source-validation'
source=json.loads((evidence/'evidence-source.json').read_text())
assert source['commit']==os.environ['GITHUB_SHA']
recovery=f'''# Common Vision recovery checkpoint — v1.7.1

Repository: https://github.com/{source['repository']}
Branch: origin0-common-vision
Saved source commit: {source['commit']}
Validation run: https://github.com/{source['repository']}/actions/runs/{source['run']}
Source evidence artifact: {source['artifact_id']} ({source['artifact_sha256']})

## Completed prototype

The local executable now offers contributions, distinct original-source-pending
12 + 1 values, all five linked document views, an immutable adopted snapshot,
proposal review, separate experiment authorisation, outcomes and a new reviewable
follow-up. The named human decision records remain separate from automated
technical proposals. Demonstration records, bot analysis, objections and stated
participation gaps remain visible. Existing media, property and development tools
are retained. See COMMON_VISION.md for the complete workflow and limits.

## Actual validation

This package is produced only after the source race suite, vet, restricted Docker
candidate test, existing finishing/property API tests, Concept Studio browser
checks, Production Studio browser/video checks, Common Vision API/browser cycle,
and Windows Go suite/vet/Common Vision executable cycle pass in the run above.
The detailed reports, actual screenshots and generated video are in validation/.
Common Vision browser checks cover six widths (280–2560 pixels) and touch input.
These results apply to this prototype and fixtures; they are not performance,
representativeness, real-world-policy-effectiveness or all-device guarantees.

The old Windows image-generation test was already completed in run 34488608267:
80.55 s for a 256-pixel image, 211.03 s for its reference revision and 29.31 s for
1024-pixel neural finishing. It was not stalled and was not repeated here.

## Preserved preceding work

RECOVERY_CHECKPOINT_20260910.md records the preceding merged PR #4, preserved
legacy changes, temporary evidence and verified v1.7 media release. Requested
PR #47 returned 404 in the accessible repository; its branch/commit remain
unverified. No claim that PR #47 was found is made.

## Exact remaining steps

1. Run ORIGIN0.exe on the target Samsung Galaxy Book5 Pro 360. No Python is needed.
   This increment was tested on Windows x64 CI, not on that physical laptop.
2. Open Common Vision, load the explicitly labelled demonstration, and follow
   the seven-step review/outcome walkthrough in COMMON_VISION.md.
3. Supply the authoritative original 12 + 1 values, the special +1 explanation
   and existing manuscript/manifesto. Confirm exact source wording before use.
4. Define the participating community, outreach gaps and accountable reviewers.
   Current actor names are declared, not verified identities. No public mandate
   or general agreement can be inferred from this local workspace.
5. Replace synthetic examples with consented contributions and cited evidence.
   The current synthesis is a transparent topic register, not semantic consensus.
6. Review proposals and obtain real authority outside this prototype before a
   real experiment. Record measured outcomes and limits separately from forecasts.
7. Prioritise the next capability from those outcomes. Use the existing development
   lab for a bounded proposal/test/review cycle; technical changes do not adopt
   foundational values or authorise consequential actions.

## Resume and recover

The tagged v1.7.1 source names the exact tested commit above. Keep the entire
origin0_data folder. Export recovery JSON from Common Vision before a move. Its
state is origin0_data/common-vision.json; stop the app and preserve a backup
before manually restoring that file. The older general Save State ZIP does not
include it yet. Never replace a newer working folder with a historical checkout.
'''
checkpoint=release/'COMMON_VISION_RECOVERY.md';checkpoint.write_text(recovery,encoding='utf-8')
files={'ORIGIN0.exe':release/'ORIGIN0.exe','COMMON_VISION_RECOVERY.md':checkpoint}
for name in ['README_FIRST.txt','COMMON_VISION.md','RECOVERY_CHECKPOINT_20260910.md','PRODUCTION_STUDIO.md','INTEGRATION_REVIEW.md','LOCAL_MODELS.md','MODEL_GUIDE.md','LICENSE']:
 files[name]=root/name
for p in (root/'third_party').rglob('*'):
 if p.is_file():files[p.relative_to(root).as_posix()]=p
for p in (release/'common-vision-windows').rglob('*'):
 if p.is_file():files['validation/windows/'+p.relative_to(release/'common-vision-windows').as_posix()]=p
for folder in ['common-vision-api','common-vision-browser','browser-acceptance','production-browser','production-acceptance','container-acceptance']:
 for p in (evidence/'release'/folder).rglob('*'):
  if p.is_file():files['validation/'+folder+'/'+p.relative_to(evidence/'release'/folder).as_posix()]=p
files['validation/evidence-source.json']=evidence/'evidence-source.json'
archive=release/'ORIGIN0_COMMON_VISION_WINDOWS_v1.7.1.zip'
with zipfile.ZipFile(archive,'w',zipfile.ZIP_DEFLATED,compresslevel=6) as z:
 for name,path in sorted(files.items()):z.write(path,'ORIGIN0/'+name)
(release/'COMMON_VISION_SHA256SUMS.txt').write_text(''.join(hashlib.sha256(p.read_bytes()).hexdigest()+'  '+p.name+'\n' for p in [release/'ORIGIN0.exe',archive,checkpoint]))
print(archive)
