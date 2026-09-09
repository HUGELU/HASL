# ORIGIN-0 v1.5 validation — in progress

Verified so far:
- Default model files downloaded and passed pinned SHA-256 checks.
- Actual stable-diffusion.cpp generation on Linux x64 CPU: 512 × 512, 8 Euler
  steps, seed 42, six threads. Native generation reported 465.97 seconds.
- Prompt: a natural photograph of an orange tabby cat sitting on a blue chair
  beside a sunlit window, green houseplants, realistic fur, soft daylight.
- The returned PNG was decoded and visually inspected. It matches the requested
  scene. This is a single example, not a general quality benchmark.
- All inherited Go tests and new downloader/job/TLS worker contract tests pass.

Pending: Windows native launch, packaged setup, and actual app-level image job.
Contract tests that use a generated PNG fixture are explicitly labelled; they
are not counted as real diffusion inference. The browser preview environment
previously blocked loopback navigation; no visual browser QA is claimed.
