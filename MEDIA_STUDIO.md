# ORIGIN Creative Studio

This release embeds adapted vanilla components from Autom8AI/Open-Higgsfield-AI rather than merely listing that project as a reference. Create image, reference input, advanced controls and cinema camera-prompt controls use the local ORIGIN/ComfyUI bridge. No MuAPI account or generation subscription is required for the local paths. The upstream hosted-model catalogue is not misrepresented as downloadable weights.

## First run
Open Creative studio → Models & engines. Download the native Z-Image-Turbo Q4 pack for the existing CPU/Vulkan/Metal engine, or install the official ComfyUI portable environment on Windows x64 and download a ComfyUI model pack. Select Use model. Progress shows actual downloaded bytes; incomplete files are resumed and checked against pinned SHA-256 values. Model downloads need disk space, network access and time.

On a Samsung Galaxy Book5 Pro 360 with 32 GB RAM, start with native Z-Image-Turbo or an SDXL family model. ComfyUI portable chooses the official Intel package when Intel is detected. Actual GPU availability is established by the runtime; failed GPU launch retries CPU. The 30 GB Qwen pack is aimed at a larger workstation. These are resource estimates, not claims of perfect tuning or superior image quality.

The full ComfyUI editor is accessible from Models & engines and Video & workflows. It retains its own workflows, nodes, LoRAs, ControlNet, reference editing and supported video/audio/3D tools. API-format workflows can be submitted through ORIGIN and their outputs saved in its gallery. Installing the engine does not install every model/custom node or grant free access to commercial partner APIs.

## White label
White label settings persist an organisation name, tagline, accent colour and optional uploaded logo. The main workspace and embedded studio use that branding. About and third-party notices remain available. ComfyUI/model licences continue to apply; the software is not the proprietary Higgsfield service.

## Provenance and boundaries
Open-Higgsfield-AI pin: b578108936e83a3b2a5e86644057a56f8aea73a1. ComfyUI runtime pin: v0.35.0. FLUX.2 / Qwen graph nodes were checked against Comfy-Org/workflow_templates tree 18abde72b2b778610d90dc0d17cfbc37f0413344. Model weights are referenced at immutable Hugging Face revisions with their LFS hashes.

Native and ComfyUI jobs share ORIGIN's bounded local heavy-work slot. A normal batch has up to eight images; native queue limit 256 and ComfyUI queue limit 64 avoid an unbounded memory backlog. Parameters and imported workflow graphs persist with outputs. Reopening ORIGIN does not automatically resubmit an interrupted external workflow.

Cinema camera/lens settings compile descriptive prompts; they do not simulate optics or guarantee 3D/character consistency. Large final exports use the existing separately tested tiled neural finishing pipeline. Common Vision, its human-review boundaries, the original-values placeholders, and all earlier tools remain available.

## Development
The generated Tailwind 3.4.17 CSS is committed and embedded: no npm or CDN is needed on the user PC. Regenerate it from web/open-studio with `../../.media-tools/node_modules/.bin/tailwindcss -i src/styles/global.css -o tailwind.css --minify` after installing tailwindcss@3.4.17 into .media-tools. Pinned camera thumbnails are embedded as data URLs in camera_assets.js, preserving source-only rebuilds.
