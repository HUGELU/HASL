# ORIGIN-0 — local image generation and volunteer worker PCs

ORIGIN-0 is an open-source image studio and experimental learning workbench.
The immediate goal is reliable image generation on ordinary computers, with
visible setup, useful diagnostics, and a way for explicitly joined PCs to
share independent image jobs.

**Windows preview v1.5 is available.** The packaged app passed actual Windows CPU
startup, model setup, generation and PNG-download checks. A separate Linux CPU
run produced a visually checked 512 × 512 image.

[Download the Windows ZIP](https://github.com/HUGELU/HASL/releases/download/v1.5.0/ORIGIN0_WINDOWS_v1.5.zip) ·
[Download ORIGIN0.exe](https://github.com/HUGELU/HASL/releases/download/v1.5.0/ORIGIN0.exe) ·
[Release and checksums](https://github.com/HUGELU/HASL/releases/tag/v1.5.0)

## Image generation

- Native Go application; the primary image path needs no Python or API account.
- [stable-diffusion.cpp](https://github.com/leejet/stable-diffusion.cpp) executes
  [Z-Image-Turbo](https://huggingface.co/Tongyi-MAI/Z-Image-Turbo).
- Quantized diffusion and text-encoder weights plus a VAE total **6.52 GB**.
  First setup downloads them visibly, resumes partial files, and verifies
  pinned SHA-256 checksums. Subsequent use can be offline.
- CPU fallback and optional Vulkan acceleration. CPU generation can take minutes.
- Prompt, size, seed, step count, queue, cancellation, PNG downloads, and logs.
- Threads and per-job time limits remain under the owner's control.

The application code is MIT licensed. The default model pack uses Apache-2.0
weights. No ORIGIN-0 service subscription is required. Model licences, attribution
requirements, hardware costs, electricity and connectivity still apply.

## Run

Download the ZIP above, extract it into a new folder, and double-click
**ORIGIN0.exe**. The tested CPU runtime is inside the executable. Choose **CPU**
for the tested configuration. This release targets modern Windows x64 PCs with
AVX2; Android, iOS, Windows ARM and individual GPU drivers are not covered by this
release test.

Developers can build the source using Go 1.23+ and the `build.py` helper:

```sh
python3 build.py --target linux
# Windows: python build.py --target windows
```

Run the executable, keep its console open, then use the browser address printed
there. In **Image studio**, choose **Set up image engine**, wait for **Ready**,
enter a prompt and press **Generate image**. Keep at least 10 GB of free disk
space. 16 GB RAM is a practical starting point; 32 GB gives more room. This is
not a guarantee for every model size, GPU driver or operating system.

## Volunteer compute

1. On the coordinator, open **Worker PCs**, start a group and create an invitation.
2. On each worker, finish image setup, paste its invitation, choose a name and
   a finite job allowance, then join.
3. On the coordinator, select **Send this job to a joined worker PC**.

TLS encrypts traffic and each invitation pins the exact group certificate.
Workers pull image jobs, renew leases, return checked PNGs and stop at their
allowance. Cancelling invalidates a job lease; a late result is rejected.
A disconnected job can be reassigned once. Stop/leave ends participation.

This version uses a coordinator per group. It shares complete jobs, not GPU
memory. Public peer discovery, NAT traversal, torrent model transfer, federated
training, tensor-parallel inference and blockchain incentives are not implemented.
For remote PCs, arrange a reachable private-network address. A public computer or
free trial is not a volunteered worker just because it is discoverable.

## Experimental learning

The existing workbench remains available: bounded pattern graphs, independently
remembered hypotheses, interface candidates, measured usability trials, saved
branches, labels and small-feature recognition. A tested numeric distance kernel
can be generated and archived in a rebuilt application. Optional SDXL LoRA and
CogVideoX workers still need their separate Python/model environment.

These experiments do not train Z-Image-Turbo automatically, invent a stronger
foundation model, or demonstrate improving realism. More stored files or work
cycles alone are not a measure of intelligence. Arbitrary contributed code is
reviewed as normal source changes; it is not executed through the worker protocol.

## Contribute

See [CONTRIBUTING.md](CONTRIBUTING.md), [validation](VALIDATION_V15.md), and
[model provenance](third_party/NOTICES.md). Useful work includes clean Windows
startup, Intel GPU performance, accessible interfaces, reproducible image quality
benchmarks, and better distributed scheduling. Join the [hardware testing issue](https://github.com/HUGELU/HASL/issues/2). Human and AI-assisted contributions
use the same pull-request checks.
