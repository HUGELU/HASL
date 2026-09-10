# Open Studio recovery checkpoint

Base: main 70ef76d580329d2ff6f40025d5b42b7d326e4f08; PR #5 and v1.7.1 remain intact. Working branch: origin0-media-studio. Previous dirty work remains on recovery/pre-common-vision-20260910.

Implemented in this work: pinned Open-Higgsfield-AI image/reference/cinema components; same-origin native/ComfyUI adapter; seven complete local model packs; checksum/resume downloads; official ComfyUI Intel/AMD/NVIDIA portable installer; full workflow editor access and API workflow submission; durable outputs/parameters; name, colour and logo branding; Common Vision retained.

Actual current tests: Linux compile passed. First full Go race run reported one failure: TestHTTPState expected the old 1.7.1 version string. The assertion was updated for 1.8.0; the suite still needs rerunning. No Windows/ComfyUI generation test has yet been run on this new media integration. Earlier v1.7.1 and native-generation evidence remains historical, not evidence for this update.

Remaining steps: finish frontend browser validation and correct any controls/errors; add real ComfyUI adapter fixtures and Windows portable/native generation acceptance; run full retained regression suite; record screenshots/logs and exact results; build a new versioned Windows package; publish only after required tests pass; merge the reviewed branch without overwriting earlier releases.

Known limits: local model licences differ; not a free proprietary Higgsfield/MuAPI service. Full video and multi-reference editing use installed ComfyUI workflows. Hardware estimates and configuration are not universal speed or quality guarantees.
