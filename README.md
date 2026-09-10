# ORIGIN-0 — local image generation and a learning workbench

ORIGIN-0 runs an open image model on your own computer and makes generation,
reviewed teaching examples, interface experiments and volunteer worker PCs
available in one local application. Application code: **MIT**. The native
Z-Image-Turbo model pack uses **Apache-2.0** weights.

[Windows v1.6 ZIP](https://github.com/HUGELU/HASL/releases/download/v1.6.0/ORIGIN0_WINDOWS_v1.6.zip) ·
[ORIGIN0.exe](https://github.com/HUGELU/HASL/releases/download/v1.6.0/ORIGIN0.exe) ·
[Release evidence and checksums](https://github.com/HUGELU/HASL/releases/tag/v1.6.0)

The v1.6 release workflow publishes these downloads only after source, browser
and actual native Windows image-generation tests pass. See
[validation](VALIDATION_V16.md) for the exact scope.

## Start on Windows

1. Extract the ZIP into a normal writable folder and double-click **ORIGIN0.exe**.
2. Keep the console open. It prints the local address and opens your browser.
3. In **Image studio**, click **Set up image engine**. The first run downloads
   **6.52 GB** of pinned model files, with visible progress and resumable downloads.
4. Wait for **Ready**, enter a prompt and click **Generate image**.

The native CPU runtime is inside the release executable. This image path needs
**no Python, API key or paid subscription**. Keep at least 10 GB of disk space
free. 16 GB RAM is a practical starting point; 32 GB gives more room. The Windows
CPU package targets modern x64 processors with AVX2. A Galaxy Book5 Pro 360 is
in that processor class, but its specific GPU/driver has not been certified here.
CPU images can take minutes. Automatic Vulkan probing has a CPU fallback.

## Generation controls

- CPU count, RAM, CPU identity and available GPU inventory; actual runtime
  device probing during setup. Draft, balanced and detail recommendations.
- Prompt helpers for candid photography, architecture and fashion; editable
  wording, sizes, steps, fixed or random seeds and batches of up to 16 requests.
- **One-reference image revision**, adjustable change strength and rough
  intermediate sampling previews from the pinned native runtime.
- Queue and cancellation: one local image at a time, up to 256 pending jobs,
  and 1,000 recent job records with history paging.
- Four user ratings, notes and downloadable generation metadata. After three
  rated results for the **same prompt and reference**, an enabled preference
  selector can reuse the best-rated settings. Loading/reusing explicit saved
  settings disables that selector until you turn it on again.
- Voice-to-prompt through a configured local or remote transcription provider.
  Recording itself is local; automatic offline transcription is not bundled.
- A local PIN lock, a private recovery-code download, restart locking and
  inactivity locking. The server blocks workspace data APIs while locked.
  Files on disk and the process remain visible to the operating-system owner.

Recommendations are starting points. Ratings learn a preference over settings;
they do not silently train the native image model. Image-to-image revision does
not guarantee identity, exact logos, unchanged regions or 360-degree geometry.
Native output remains at most 1024 × 1024 in this release. See the
[model and media guide](MODEL_GUIDE.md) for additional models and remaining work.

## Teach concepts and inspect evidence

**Concept Studio** combines descriptive attributes and position sliders with
image collection, review, separate datasets and measured experiments.

- Search Openverse or a configured SearXNG instance. Download a bounded set of
  results with source/licence metadata, or import images you provide.
- Review each category and caption. Downloaded images begin as pending examples.
  Duplicates and conservative near-duplicate groups share a dataset split.
- Try exact synthetic exercises for left/right, above/below, in/out, on/off
  and colour. Compare six small spatial-feature classifier candidates.
- Select on validation evidence; report a separate audit result; retain the
  previous classifier for rollback. These are small recognizers, not a vision
  foundation model or a claim of general comprehension.
- Export split datasets, feed a project's approved examples to the optional
  SDXL LoRA worker, or preview and publish a recipe as a **draft GitHub PR**.
  The contribution flow sends no image bytes or model weights.

[Concept Studio guide](CONCEPT_STUDIO.md) · [Technology report](TECHNOLOGY_REPORT.md)

## Connect worker PCs over the internet

Each group has a coordinator. Workers explicitly join with their own model,
finite job allowance and local resource limits. They pull whole-image jobs,
renew leases and return validated PNGs. Leaving stops their current contribution.

Direct groups use TLS and an invitation that pins the coordinator certificate.
For laptops behind routers, deploy ORIGIN-0's **HTTPS relay** on a reachable
server, select **Connect through an internet relay**, and share invitations.
Both laptops make outbound HTTPS connections; the relay carries AES-GCM encrypted
job envelopes. No port forwarding is required on those laptops.

[Relay deployment and limits](INTERNET_RELAY.md). No public relay is supplied.
There is no internet VRAM pooling, public peer discovery, tensor-parallel model
inference or federated image training in this version.

## Existing research and source evolution

The previous graph, independent hypothesis memories, interface candidates,
measured usability trials, saved branches and restore operations are retained.
The optional local workbench supports SDXL image/LoRA and CogVideoX jobs in a
separate model environment. A validated numeric recognition kernel can be
reconstructed, tested and archived in a new executable using an installed Go
compiler. General model-written source edits remain review proposals.

[Local models and rebuilding](LOCAL_MODELS.md). Larger datasets, repeated
inference and more workers alone do not establish increased intelligence or
better realism. No superiority over other image generators is claimed.

## Build and contribute

Go 1.24+ builds the host; the Python helper embeds the exact source and invokes Go.

```sh
python3 build.py --target linux
# Windows: python build.py --target windows
```

Model licences and attribution requirements apply independently of ORIGIN-0's
MIT licence. Hardware, electricity and internet access remain necessary.

[Contributing](CONTRIBUTING.md) · [Hardware testing](https://github.com/HUGELU/HASL/issues/2) ·
[Model provenance](third_party/NOTICES.md). Human and AI-assisted changes use the
same tests and pull-request review process.
