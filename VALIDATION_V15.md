# ORIGIN-0 v1.5 validation

The Windows preview is published at https://github.com/HUGELU/HASL/releases/tag/v1.5.0 .
The executable was built from commit `23570a14f0a442af44ae9b4e9652971a13609ce1`.
[The complete Windows/source run passed](https://github.com/HUGELU/HASL/actions/runs/34328285346).

| Check | Result |
|---|---|
| Go tests | 50 pass on Windows; 50 pass with race detection on Linux |
| Go vet | Pass on Windows and Linux |
| Optional Python worker contracts | 5 pass; no GPU training/video claim |
| JavaScript | Syntax checks pass; generator control references and form labels checked |
| Native Windows dependency check | CPU executable starts and imports no unbundled MSVCP/VCRUNTIME/GGML DLL |
| Windows clean app launch | Pass; authenticated local HTTP API works |
| Windows model setup | Actual 6.52 GB model pack downloaded and verified against pinned SHA-256 |
| Windows image generation | Actual Z-Image-Turbo, 256 × 256, one Euler step, seed 42; 63.17 seconds |
| Windows output | PNG persisted and downloaded through the application API |
| Large-file handling | Sampling a 9 TiB sparse file and a 16 TiB configured quota pass |
| Volunteer workers | Two engine instances pass invitation/TLS, lease, output, cancellation and finite-job contracts |

The Windows runner reported AMD EPYC 7763 CPU. The one-step image is an execution
check, not a quality benchmark. Higher-resolution, normal-step generation takes
longer. The user's particular Samsung laptop and its Intel GPU have not been
physically tested here.

The release contains [report.json](https://github.com/HUGELU/HASL/releases/download/v1.5.0/report.json),
[generation.log](https://github.com/HUGELU/HASL/releases/download/v1.5.0/generation.log),
[the actual PNG](https://github.com/HUGELU/HASL/releases/download/v1.5.0/image.png),
[native imports](https://github.com/HUGELU/HASL/releases/download/v1.5.0/imports.txt)
and [SHA256SUMS.txt](https://github.com/HUGELU/HASL/releases/download/v1.5.0/SHA256SUMS.txt).
The PNG SHA-256 is `21d07b1d127bf5169e74c92ca779ee52f76b80b25b574dbe1b9783fda8d00a6c`.

Earlier real Linux CPU validation generated a 512 × 512, eight-Euler-step image,
seed 42, six threads, in 465.97 seconds. The prompt requested an orange tabby cat
on a blue chair beside a sunlit window and plants. The output was decoded and
visually checked against that scene. A separate Linux app-level 256 × 256,
one-step generation completed in 33.11 seconds. These are individual runs,
not broad quality or speed benchmarks.

Release fixes include Windows static-runtime policy, isolation of downloaded
C++ sources from Go package discovery, actual NTFS sparse-file preparation,
complete request-body handling for status responses, and visible failure logs.
The release pipeline refuses publication unless actual Windows generation passes.

The worker protocol tests use explicitly labelled PNG fixtures. They establish
job transport and control, not distributed diffusion performance. A two-physical-PC
image benchmark, Intel/Vulkan measurement and rendered browser accessibility review
remain open. Browser preview access to the local app was blocked in this environment;
no visual browser QA is claimed.

The earlier workbench is preserved: supplied-file ingestion, pattern/hypothesis
state, internal descendants, measured interface candidates, saved branches and
restores, taught recognition, optional voice/provider tools and a bounded numeric
source-rebuild experiment. Native image inference does not automatically improve
its weights. Autonomous general source rewriting, public torrent discovery,
shared GPU memory and unlimited compute are not implemented.
