# ORIGIN-0 Connected Studio v1.9.0

The studio now combines the adapted Autom8AI/Open-Higgsfield-AI image/cinema interface, ComfyUI execution, Stability Matrix interoperability and live Hugging Face/Civitai model discovery.

## What changed

- One **Set up & use** action prepares the local engine, downloads the model, checks the engine's actual loader list and selects the model. Progress stays visible; stopping setup retains partial downloads.
- Search **Hugging Face** or **Civitai**, inspect exact versions and file sizes, retain creator/model-card links and download checksum-verified files. Imported SD 1.5 / SDXL complete checkpoints can use the simple generator. Components for other architectures remain available to matching workflows.
- Install the official **Stability Matrix 2.16.3** Windows package from a pinned release, or link an existing Data directory. ORIGIN can open Matrix and ask it to launch one of its installed packages using the documented command-line interface. Its settings and existing files are preserved.
- Managed ComfyUI reads both ORIGIN's model folders and the linked Matrix library. The installed-model view uses actual ComfyUI checkpoint and LoRA lists. Existing external engines can import the generated shared-path YAML.
- **Engine status** shows actual connection state, current setup, hardware inventory and startup logs. Its diagnostic export excludes provider tokens. Downloads that receive no data for 90 seconds are interrupted and can resume.
- White-label name, logo, colour and tagline remain configurable. Existing Common Vision, research, property, finishing, learning and privacy tools remain available.

## Practical limits

Stability Matrix opens as its own desktop application; this is integration through its supported CLI and shared model folders, not an embedded web replacement of its entire desktop UI. ORIGIN's white label does not remove third-party branding or licence obligations. Matrix source is AGPL-3.0; official binaries have their own EULA. ComfyUI and each model retain their own terms.

Open-Higgsfield-AI's upstream interface uses MuAPI for hosted generation. ORIGIN's local adapters do not supply proprietary hosted weights or paid services. No hosted subscription is required for the included local generation paths. Some downloadable models require provider accounts or accepted model terms. Provider tokens are stored locally, not in an encrypted vault, and are never returned in diagnostics.

A file download alone does not make every architecture runnable. Complete SD 1.5/SDXL checkpoints have direct recipes; other models may require companion components and installed ComfyUI nodes. Advanced video/multi-reference/masking uses matching ComfyUI workflows. Matrix can manage additional generation/training packages in its own interface.

This Windows build targets x64. macOS/Linux source builds can connect their installed ComfyUI. Automatic setup does not establish optimal performance. Intel GPU throughput on the owner's Samsung Galaxy Book5 Pro 360 remains unbenchmarked, as do the larger FLUX/Qwen packs. CPU generation takes time. No superiority to commercial models is claimed.

## Validation and recovery

The release is publishable only after source, retained workflows and actual Windows installation/generation gates pass. The ZIP includes the exact tested executable, outputs, parameters, logs, screenshots and reports. Consult MEDIA_VALIDATION.json for actual timings and scope; a small acceptance image is functional evidence, not a quality benchmark.

Keep the complete origin0_data directory when updating. Stop the previous process first. Source branch: origin0-media-studio; PR: https://github.com/HUGELU/HASL/pull/6. The older v1.8.0 release is preserved.

## Upstream references

- https://github.com/Autom8AI/Open-Higgsfield-AI
- https://github.com/LykosAI/StabilityMatrix/tree/v2.16.3
- https://github.com/LykosAI/StabilityMatrix/blob/v2.16.3/StabilityMatrix.Avalonia/Models/AppArgs.cs
- https://github.com/Comfy-Org/ComfyUI/tree/v0.35.0
- https://huggingface.co/docs/huggingface_hub/en/package_reference/hf_api
- https://github.com/civitai/civitai/wiki/REST-API-Reference
