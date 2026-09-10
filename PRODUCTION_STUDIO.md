# Production Studio — ORIGIN-0 v1.7

Production connects local image generation, reversible finishing, measured property
projects and source-development experiments. The existing Image studio, Concept
Studio, worker groups, interface trials and saved branches remain available.

## Finish an image

Open **Production**, choose a PNG/JPEG, choose a long edge and press **Finish image**.
You can also select **Finish / upscale** on a generated image. Enable automatic
finishing to attach those settings to subsequently queued image jobs.

| Mode | Operation | Use |
|---|---|---|
| Fast | Separable Lanczos-3 resampling and bounded sharpening on CPU | Resize artwork or photos while retaining transparency; no model download |
| Neural | Original Real-ESRGAN x4plus weights through NCNN, followed by final resizing and optional sharpening | Estimate additional texture/detail; compare against the preserved original |

The Windows release embeds the neural runtime and weights. No Python, account,
API key or separate model setup is required for finishing. The diffusion image
generator still needs its separate 6.52 GB first-run model download.

Neural mode uses opaque PNG/JPEG inputs. Its learned pass accepts at most 1024 pixels
on the long edge; larger originals are reduced for that pass. The model creates
4× pixels, then Lanczos produces the requested final dimensions, up to a 7680-pixel
long edge and 60 megapixels. Consequently an 8K export can include interpolation.
It is not an 8K-native model or proof that missing details were recovered correctly.
Use **Detail blend** to reduce the contribution of inferred detail.

Input limit: 64 MiB and 60 megapixels. Output memory is estimated before processing.
One heavy local generation, finishing or development task runs at a time. The
finishing queue supports 32 pending jobs and 200 recent records. Identical recipes
reuse a completed output after verifying its checksum. Cancellation retains the
original; an interrupted run is marked explicitly when the app reopens.

CPU inference schedules up to four independent tiles without requiring an external
OpenMP runtime. Vulkan selection is available when the packaged NCNN runtime finds
a compatible device; CPU execution is the release acceptance path. Select CPU if
your driver fails. Hardware-specific throughput must be measured on that computer.

Each result includes its original asset ID, output hash, requested settings,
dimensions, backend and exact runtime/model manifest in a downloadable JSON recipe.
Both source and result remain available for comparison. Neural reconstruction can
alter lettering, faces, materials and edges; it does not guarantee exact logos or
survey geometry. [Real-ESRGAN](https://github.com/xinntao/Real-ESRGAN) supplies the
learned model; [NCNN](https://github.com/Tencent/ncnn) supplies inference.

## A property project

1. Name the project and add rooms. Enter measured X/Y positions, width and depth in
   metres. Room names identify reference views and therefore must be distinct.
2. Add an optional door wall, offset and width. Create and download an SVG plan.
   Overlapping rooms, invalid dimensions and doors outside their wall are rejected.
3. Upload up to 12 reference photos. Associate them with rooms, reorder the shots
   and set each duration from 1 to 10 seconds.
4. **Render this reference** opens Image studio with that image, low change strength
   and editable architecture/camera/lighting guidance. Review the output against the
   supplied photo. **Add to property project** returns a generated result to the list.
5. Save the project JSON. It records dimensions, shot order and asset IDs in this
   workspace. Copy the complete `origin0_data` folder when transferring its original
   assets to another computer. The JSON alone does not contain the photographs.
6. **Create photo walkthrough** produces one self-contained HTML file with its images
   embedded. It plays offline. In a browser supporting VP9 MediaRecorder, **Export
   WebM video** records the sequence at 1920×1080/24 fps in real time. Keep the tab
   visible while recording and inspect the downloaded clip before sharing it.

The plan is a schematic from supplied dimensions. It does not estimate dimensions
from photographs, account for wall thickness or implement statutory floor-area
rules. Room areas are rectangular width × depth sums. The walkthrough is an ordered
photo sequence with gentle motion; it does not reconstruct a navigable 3D building.
Lens and lighting selections are prompt guidance, not a calibrated renderer.

## Local development lab

Connect an installed coding model in **Settings** using its numeric loopback URL,
for example `http://127.0.0.1:11434/v1` for Ollama or
`http://127.0.0.1:8080/v1` for a compatible llama.cpp server. Supply its actual model
name, enable the connection and set the finite request allowance. The lab uses the
local Chat Completions endpoint. No paid provider is required or used by this lab.
[Ollama compatibility](https://docs.ollama.com/api/openai-compatibility) ·
[llama.cpp server](https://github.com/ggml-org/llama.cpp/tree/master/tools/server).

Choose the recognition distance kernel or image resampling source, describe a
specific improvement goal and select 1–3 iterations. ORIGIN sends that source file
and prior validation feedback to the local model. Each response must identify the
same file and contain valid Go syntax. The interface records the proposal, risks,
feedback and a downloadable review archive. This is iterative coding assistance;
it does not train the coding model or establish an improvement merely by producing
different source.

**Executable tests require the optional Docker sandbox.** Install Docker yourself
and pull an official Go image compatible with this project's `go.mod`. Resolve its
immutable digest with `docker image inspect` and configure the absolute Docker CLI
path and full image name such as `golang@sha256:<actual digest>`. The app never pulls
an image automatically. Windows users need Docker's Linux-container mode.

Tests run on a copied source tree with the selected candidate file, fixed tests,
no network, no writable root filesystem, no additional capabilities, a non-root UID,
2 CPU/2 GiB memory limits, and a five-minute host deadline. Temporary storage is
bounded. Without this configured container, the interface reports **syntax only**.
A passing functional suite is not a speed benchmark or a security proof.

Model-written source is archived for review and does not overwrite the running
program. The previous validated numeric-kernel rebuild workflow remains separately
available in **Learning & upgrades**. The coding lab does not access proprietary
model internals, crawl/install arbitrary repositories or invent new verified model
architectures automatically.

## Validation scope

The release pipeline runs Go race tests and vet, actual CPU neural inference on
Windows/Linux, identical-pixel sequential/parallel tile checks, real app finishing,
browser operation and WebM download, six responsive widths and touch interaction.
The Windows release also requires real native diffusion and image-revision output.
The container gate uses a baseline candidate; model transport uses a deterministic
local response fixture. It does not benchmark the quality of an installed coding LLM.
Exact reports and generated test files accompany the release. No quality lead over
Higgsfield, Midjourney or other services is established by these acceptance tests.
