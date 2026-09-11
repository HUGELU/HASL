# Published Connected Studio v1.9.0 — 11 September 2026

**Owner acceptance remains unresolved.** After this release, the owner reported that the application still does not work and requested local desktop-assisted repair and a stronger coding/self-improvement foundation. No local error report or desktop connection has yet been provided. The passing CI results below remain valid for that test environment and do not establish a successful run on the owner's machine. Resume with LOCAL_PC_REPAIR.md; reproduce the actual failure before changing another executable.

Release: https://github.com/HUGELU/HASL/releases/tag/v1.9.0. Exact tested application commit: 013e48806a2bb1fbf461fb3e6475c0960ab817f2, branch origin0-media-studio, PR https://github.com/HUGELU/HASL/pull/6. Passing source, Windows and publication run: https://github.com/HUGELU/HASL/actions/runs/34641641245. PR #6 remains open; main has not been merged.

The complete source gate passed, including real Hugging Face and Civitai search/version/file metadata, race tests, vet, restricted container, neural finishing, exported walkthrough video, all retained browser flows, PIN recovery and Common Vision. Source evidence: artifact 10280466734, SHA-256 96fd7e26ff11d79dbb1a3bb371d2cc166b6c689314337d439f778544daa3d9f8.

Windows passed build/tests/vet, single-action ComfyUI/SD 1.5 setup, checksum-verified official Stability Matrix installation and real executable CLI startup, native Z-Image generation, ComfyUI generation and reference editing. Native image: 174.64 s; ComfyUI image: 95.44 s; reference edit: 22.08 s. All three were 256 × 256 at two steps on a four-thread CPU with about 16 GB RAM. There were zero connection-recovery retries and no recorded application panic. These are functional acceptance results, not evidence of superior image quality or Intel GPU performance. Full Matrix desktop interaction was not part of this acceptance. The Windows image-generation test is complete for this release.

The published ZIP contains nine passing report groups, actual images, video recordings, generation parameters and logs. Executable SHA-256: c46316e3090115fd4cbee80da5cb3219dd3598a6a009a0c4463a2e555512e06b. ZIP SHA-256: b6261ca15f4b11cc0ce7cc5af9f44c15c174871c120f685e404174624266ab47. Windows artifact: 10280603340, SHA-256 066087a495c20e30605b2f5e2226b34444a87ca1e5bd5f171929b50ab4c2c26b. Final release artifact: 10281055136. Older releases remain unchanged.

Base preserved: origin0-media-studio at 1a065524d4969223400d6f7003701a4b633ff68e, with published v1.8.0 unchanged. PR #6 remains the working integration PR. A previous main-branch merge was blocked by automatic approval review and has not been retried or bypassed.

Implemented: live Hugging Face/Civitai discovery and immutable/hash-verified model imports; local provider tokens excluded from diagnostics; dynamic saved model registry; actual Stability Matrix installer/CLI/package launch and read-only shared-library integration; combined engine/model setup with readiness checks; installed checkpoint selection; shared ComfyUI paths; first-run progress, diagnostics and download inactivity handling. Existing functionality is retained.

Local checks: Go build passed; full Go race suite passed in 32.479 s. Expanded browser checks passed, including model-source selection, version/file UI with labelled provider fixtures, Matrix controls, diagnostics, branding, Common Vision access and five responsive widths. Direct read-only upstream metadata verified HF model-revision/LFS fields and Matrix v2.16.3 CLI flags. The local live-provider acceptance was blocked with HTTP 400 and the execution tool subsequently reported a cancelled network-approval request; it is not recorded as a passing live-provider test.

First CI candidate f6642cc5cb7d65f4da00c085a392f562c8f6fe29: https://github.com/HUGELU/HASL/actions/runs/34640444449. Source/race/vet, restricted container, production exports, all retained browser/API tests and live Hugging Face metadata passed. Civitai text search returned HTTP 400 because the request combined `page` with `query`; upstream explicitly rejects that combination. Corrected to omit page for text search and added a regression test. Provider errors now preserve the useful provider message. Provider tabs were moved to the top of the model library, and the inherited hosted LoRA-ID field now lists real local ComfyUI filenames. Publication stayed blocked for the failed candidate.

The correction was saved at 80df6b160de280956a9f7fb2c77cdd9fbb9a1c07; local Civitai/provider regression tests and the revised five-width browser check passed. Final visual inspection also corrected the inherited v1.8.0 footer. Before supersession, the first Windows candidate completed combined ComfyUI/SD 1.5 setup and actual Matrix executable CLI startup, then was cancelled during the progressing native-model download (1,422,506,327 / 3,683,370,944 bytes). It was not stalled and did not complete image generation. Its partial evidence is preserved in run 34640444449, artifact 10280032719 (SHA-256 15dd17ee89da9e746458eba7818124d24170e2f956e2de89b59904237307288a). The follow-up must complete all generation gates before publication.

Remaining steps: run the published build on the owner's Galaxy Book5 Pro 360 and measure actual Intel GPU throughput; validate the larger FLUX/Qwen packs and specialised video/multi-reference workflows on suitable hardware; obtain the owner's original 12 + 1 source values. Only after explicit approval for the particular main-branch merge, verify the current PR head and merge PR #6. Do not rerun the completed Windows gate for documentation-only changes. Do not replace this release's tested executable; future application changes require a new version and validation.

Remaining product limits: full Matrix desktop UI runs separately; no claim of optimal Intel GPU performance or universal model compatibility; advanced video/other architectures require matching workflows/components. Original 12 + 1 values remain awaiting source material. Preserve origin0_data on upgrade.

---

# Open Studio recovery checkpoint

## Published and verified — 10 September 2026

Release: https://github.com/HUGELU/HASL/releases/tag/v1.8.0

Repository delivery: PR #6 is ready for review on `origin0-media-studio`. The final download/documentation checkpoint was saved at `9b2954a53d21c8adc69dfefe49e734745b636407`. Automatic approval review rejected the requested merge into `main`, stating that the user must explicitly approve this particular main-branch merge. No alternate merge route was attempted. The published Windows release remains available and the integration remains preserved on its branch.

Next repository action: obtain explicit approval to merge PR #6, verify its then-current head, and merge with that expected SHA. Do not rerun the completed Windows acceptance unless application code changes or a required gate demands it. The change recording this blocker is documentation only; the tested application commit and release checksums below remain unchanged.

Tested application commit: **106e8cd057e457b26a794d8e60326bd118ab9b06**, branch **origin0-media-studio**, PR **https://github.com/HUGELU/HASL/pull/6**.

Passing validation and publication run: https://github.com/HUGELU/HASL/actions/runs/34532281015. Source/race/vet, restricted-container, retained Concept/Production/Common Vision API and browser checks, new Open Studio browser checks, Windows tests/vet and the complete Windows generation path passed. Windows produced a native image in 117.30 s, a ComfyUI image in 70.27 s and a ComfyUI reference edit in 26.08 s. All three were 256 × 256 at two steps on a four-thread CPU. There were **zero polling recoveries** and no recorded application panic. ComfyUI reported v0.35.0, embedded Python 3.13.14 and PyTorch 2.13.0+xpu running on CPU.

The Windows image-generation test is **complete for this release**. Its results are functional acceptance evidence, not a quality or performance comparison with commercial models.

- Executable SHA-256: `708e4fba24ecf29e0040d48954e482cc32d38943d7cf4bd3bec902b92ae91498`
- ZIP SHA-256: `be86783b97b34c6cbf93648f56b866464b1b815161b1088c169ff66fd247fe98`
- Published files: `ORIGIN0.exe`, `ORIGIN0_OPEN_STUDIO_WINDOWS_v1.8.0.zip`, `OPEN_STUDIO_SHA256SUMS.txt`, `MEDIA_VALIDATION.json`, `MEDIA_RECOVERY.md`.
- The ZIP retains actual generated images, workflow/parameter metadata, logs, screenshots and browser/video validation outputs. Release artifact 10174635743 contains the same deliverables. Failed-run evidence is recorded below and separately preserved in the diagnostic recovery bundle.

Remaining steps: test automatic Intel GPU execution on the owner's Galaxy Book5 Pro 360; validate larger FLUX/Qwen packs on suitable hardware; add curated video and multi-reference recipes to the simple studio while retaining full ComfyUI workflow access; obtain the owner's original 12 + 1 values. No foundational values or consequential actions are automatically adopted.

Recovery: stop ORIGIN before copying its complete `origin0_data` directory. Keep that directory when replacing only the executable. Media settings and jobs are in `media-studio/studio.json`; original assets stay in the object store. Prior releases and the pre-Common-Vision recovery branch remain preserved. The historical entries below explain earlier failed candidates; the published result above supersedes their pending-test status.

## Development history

Base: main 70ef76d580329d2ff6f40025d5b42b7d326e4f08; PR #5 and v1.7.1 remain intact. Working branch: origin0-media-studio. Previous dirty work remains on recovery/pre-common-vision-20260910.

Implemented in this work: pinned Open-Higgsfield-AI image/reference/cinema components; same-origin native/ComfyUI adapter; seven complete local model packs; checksum/resume downloads; official ComfyUI Intel/AMD/NVIDIA portable installer; full workflow editor access and API workflow submission; durable outputs/parameters; name, colour and logo branding; Common Vision retained.

Actual current tests: Linux compile passed. First full Go race run reported one failure: TestHTTPState expected the old 1.7.1 version string. The assertion was updated for 1.8.0; the subsequent full race suite passed in 31.982 seconds. Browser launch in this local environment is blocked by socket permissions; the same browser checks are moving to the existing GitHub runner. No Windows/ComfyUI generation test has yet been run on this new media integration. Earlier v1.7.1 and native-generation evidence remains historical, not evidence for this update.

Remaining steps: finish frontend browser validation and correct any controls/errors; add real ComfyUI adapter fixtures and Windows portable/native generation acceptance; run full retained regression suite; record screenshots/logs and exact results; build a new versioned Windows package; publish only after required tests pass; merge the reviewed branch without overwriting earlier releases.

Known limits: local model licences differ; not a free proprietary Higgsfield/MuAPI service. Full video and multi-reference editing use installed ComfyUI workflows. Hardware estimates and configuration are not universal speed or quality guarantees.

GitHub PR: https://github.com/HUGELU/HASL/pull/6. First saved integration commit 3aeab5ad1d6dd3167b3097ff0f5f7c61947fe625; acceptance candidate de4e0c6d9ac7b7396647b65d7bbbe6a45d3f36e0. Push run 34525513096; PR run 34525530004. Further validation fixes may follow these candidates; use branch head for continuation.

First GitHub source acceptance identified a script-order regression: media_studio.js ran before the deferred host scripts, so renderPage was undefined. Fixed by deferring the integration script in the same order. Existing Concept Studio workflows themselves completed; validation correctly rejected the console errors.

Local standard headless Chromium is now available. The new visible Open Studio test passed: mounted image component, real first-run setup error, seven model cards/filtering, saved/restored white-label identity/colour, cinema component, rejected invalid workflow, five responsive widths (320–2560), and the preserved Common Vision tab. No page errors. The upstream missing createInlineInstructions helper was repaired. Local evidence: release/media-browser-local/. Windows portable and neural acceptance remain pending until the next successful CI run.

Source candidate a506bb3e3a5feee157b8c271b8e5b149ac123be7 passed every source/browser gate in run 34527840091. Its Windows native studio generated a real 256 × 256 PNG in 106.06 seconds (SHA-256 0523e1de2938151ce43628b994d3878f4ec16f59f2f7438f361c5084b507de92). The subsequent ComfyUI setup poll failed with Windows connection reset 10054. Publication was blocked. Evidence is artifact 10172625925; source evidence is artifact 10172380847. The acceptance collector used the wrong log root, so this follow-up corrects it to origin0_data, retains launch/runtime/ComfyUI logs and prints process exit status on failure. Do not classify ComfyUI setup or generation as passed until the full follow-up gate succeeds.

Diagnostic candidate e124197 (run 34529163374, artifact 10173297094) showed the app remained alive, without a logged panic, while the ComfyUI download advanced beyond 1.1 GB. The reset truncated the local status response. Unlike the existing native status handler, the new studio's empty-argument POST handlers did not consume their request bodies. The follow-up validates/consumes these bodies before responding, preventing unread-body connection resets and rejecting malformed control requests before side effects. Read-only acceptance polls also retry at most three consecutive transport failures, record every recovery, and reject any application panic in the log. Generation/setup mutations are never retried implicitly.

Candidate b3c2dce (run 34531294415, artifact 10173695794) completed the ComfyUI download with no reset, then failed because the Windows tar build could not inspect the verified 7z archive. The follow-up replaces the system tar dependency with official 7zr.exe 26.03, pinned by length and SHA-256 before execution; archive entries are checked before extraction. The console version label is corrected to v1.8.0. Source and Windows validation now run independently; release publication still requires both to pass.
