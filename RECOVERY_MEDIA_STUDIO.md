# Open Studio recovery checkpoint

Base: main 70ef76d580329d2ff6f40025d5b42b7d326e4f08; PR #5 and v1.7.1 remain intact. Working branch: origin0-media-studio. Previous dirty work remains on recovery/pre-common-vision-20260910.

Implemented in this work: pinned Open-Higgsfield-AI image/reference/cinema components; same-origin native/ComfyUI adapter; seven complete local model packs; checksum/resume downloads; official ComfyUI Intel/AMD/NVIDIA portable installer; full workflow editor access and API workflow submission; durable outputs/parameters; name, colour and logo branding; Common Vision retained.

Actual current tests: Linux compile passed. First full Go race run reported one failure: TestHTTPState expected the old 1.7.1 version string. The assertion was updated for 1.8.0; the subsequent full race suite passed in 31.982 seconds. Browser launch in this local environment is blocked by socket permissions; the same browser checks are moving to the existing GitHub runner. No Windows/ComfyUI generation test has yet been run on this new media integration. Earlier v1.7.1 and native-generation evidence remains historical, not evidence for this update.

Remaining steps: finish frontend browser validation and correct any controls/errors; add real ComfyUI adapter fixtures and Windows portable/native generation acceptance; run full retained regression suite; record screenshots/logs and exact results; build a new versioned Windows package; publish only after required tests pass; merge the reviewed branch without overwriting earlier releases.

Known limits: local model licences differ; not a free proprietary Higgsfield/MuAPI service. Full video and multi-reference editing use installed ComfyUI workflows. Hardware estimates and configuration are not universal speed or quality guarantees.

GitHub PR: https://github.com/HUGELU/HASL/pull/6. First saved integration commit 3aeab5ad1d6dd3167b3097ff0f5f7c61947fe625; acceptance candidate de4e0c6d9ac7b7396647b65d7bbbe6a45d3f36e0. Push run 34525513096; PR run 34525530004. Further validation fixes may follow these candidates; use branch head for continuation.

First GitHub source acceptance identified a script-order regression: media_studio.js ran before the deferred host scripts, so renderPage was undefined. Fixed by deferring the integration script in the same order. Existing Concept Studio workflows themselves completed; validation correctly rejected the console errors.
