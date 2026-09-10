# Open Higgsfield and the local production stack

Reviewed 10 September 2026. This records what the examined source actually provides
and how it relates to ORIGIN. A listed research candidate is not an installed feature.

## The Autom8AI project

The requested project exists:
[Autom8AI/Open-Higgsfield-AI](https://github.com/Autom8AI/Open-Higgsfield-AI), inspected
at commit `b578108936e83a3b2a5e86644057a56f8aea73a1`. Its README describes image,
video, cinema and lip-sync studios, multiple reference images, camera guidance,
model-dependent controls and generation history.

Its actual [MuAPI adapter](https://github.com/Autom8AI/Open-Higgsfield-AI/blob/b578108936e83a3b2a5e86644057a56f8aea73a1/src/lib/muapi.js)
sends authenticated requests to `https://api.muapi.ai`, uploads media and polls
remote predictions. An open interface does not bundle all those model weights or
provide unlimited free inference. MuAPI publishes a
[service pricing page](https://muapi.ai/pricing). The inspected README states MIT;
the inspected repository tree did not contain a top-level licence file. ORIGIN
does not wholesale copy that implementation or relabel its hosted models as local.

The separate [Higgsfield API](https://docs.higgsfield.ai/docs) is an authenticated,
asynchronous hosted service. Its account/credit requirements remain separate from
an open-source client. ORIGIN does not bundle private Higgsfield, OpenAI or Anthropic
model weights or claim knowledge of unpublished internal systems.

## Workflow mapping

| Requested capability | ORIGIN v1.7 implementation | Remaining distinction |
|---|---|---|
| Prompt → image, image → revision | Existing native Z-Image-Turbo path; queue, seeds, real sampling previews, metadata and four ratings | One initial reference; no guaranteed exact identity or local masked edit |
| Production/postprocessing | Bundled Real-ESRGAN x4plus + NCNN, fast Lanczos, sharpening, detail blend, cache and recipe | Inferred detail; final 8K can include interpolation |
| Multiple property views | Up to 12 ordered photos, room association, saved project and render handoff | These are shot references, not a multi-view-conditioned image model |
| Architecture plans | Measured rectangular rooms, doors, SVG and area sums | Schematic; no photogrammetric measurement or IFC/BIM export |
| Walkthrough output | Offline photo sequence with browser WebM recording | No recovered 3D scene or generated camera movement between viewpoints |
| Camera and lighting controls | Editable lens and light prompt guidance | No calibrated optical simulator |
| Video model generation | Existing separate optional CogVideoX worker | Requires its own compatible model environment/hardware; not bundled |
| Local coding loops | Local-model proposal → syntax/container tests → feedback → review archive | No autonomous installation of model-written host code; no LLM weight training |
| Volunteer compute | Existing authenticated worker groups and encrypted HTTPS relay protocol | Whole jobs; no cross-internet shared VRAM or supplied public relay |
| Private workspace | Existing PIN, recovery and server-enforced UI lock | No disk encryption or concealment from the operating-system owner |

## Further open building blocks, in implementation priority

| Tool | Concrete purpose | Integration decision |
|---|---|---|
| [ComfyUI](https://github.com/Comfy-Org/ComfyUI) | Local reusable image/video/audio graphs, model loading and partial graph execution | Strong next adapter for a separately installed runtime. Pin and validate workflow/node schemas and checkpoint licences; its cloud/partner nodes are separate services. Not wired into ORIGIN v1.7. |
| [COLMAP](https://github.com/colmap/colmap) | Recover camera poses and scene structure from overlapping photographs | Needed before a geometrically grounded photo-to-3D property workflow. Requires suitable coverage and scale constraints; not a floor-plan generator from one photo. Not bundled. |
| [gsplat](https://github.com/nerfstudio-project/gsplat) | Accelerated Gaussian-splat scene rendering/training | A candidate after calibrated multi-view input for navigable scene previews; CUDA/hardware needs differ from the portable CPU path. Not bundled. |
| [IfcOpenShell](https://github.com/IfcOpenShell/IfcOpenShell) | IFC parsing, geometry and BIM tooling | Useful for measured building-model interchange rather than asking an image model to invent dimensions. Not bundled. |
| [Wan2.2](https://github.com/Wan-Video/Wan2.2) | Open text/image-conditioned video models | Candidate for a verified worker adapter with explicit model size, hardware and licence metadata. Existing CogVideoX support does not imply Wan support. Not bundled. |
| [llama.cpp](https://github.com/ggml-org/llama.cpp/tree/master/tools/server) / [Ollama](https://docs.ollama.com/api/openai-compatibility) | Serve local coding models through a compatible API | Connected by the new local development lab. The operator installs the runtime and weights. |

The portable learned upscaler is implemented now because it improves both imported
photos and generated outputs without requiring a separate Python installation.
Measured project state connects those results to a practical property workflow.
The next major architecture stage should add calibrated reconstruction and
interchange, with reference photographs and dimensions retained as evidence.

For coding assistance, a manageable example is
[Qwen2.5-Coder-7B-Instruct](https://huggingface.co/Qwen/Qwen2.5-Coder-7B-Instruct),
served through a suitable quantized local runtime. Model memory plus context and
working buffers must fit the machine; an installed model's name is not a performance
guarantee. ORIGIN's loop follows public proposal/evaluation patterns such as
[Anthropic's workflow discussion](https://www.anthropic.com/engineering/building-effective-agents).
That does not reproduce the private internals of a commercial assistant.

Every further adapter needs an actual executable acceptance test, dependency/model
checksums, explicit input/output contracts, cancellation and usable errors before
being presented as integrated. An endless list of model names would not supply
those capabilities on a low-memory laptop.
