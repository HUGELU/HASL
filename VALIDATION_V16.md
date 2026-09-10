# ORIGIN-0 v1.6 validation

The [release workflow](.github/workflows/validate.yml) publishes the Windows preview only after these gates pass:

1. Linux source build, full Go race suite, go vet, JavaScript syntax and optional Python-worker contract tests.
2. Real Chromium against an isolated running application: project creation, 20 synthetic examples, six classifier candidates, category/caption review, keyboard placement controls, split dataset export and touch navigation. Concept Studio is checked at 280, 320, 390, 768, 1440 and 2560 pixels. Image studio is checked at 280, 390, 768 and 1440 pixels. Hardware preset controls, PIN setup, lock, API denial, unlock and recovery are exercised.
3. Native Windows CPU backend execution and PE import inspection, then Windows Go tests and vet.
4. Actual app startup, checksum-verified model download/setup, native text-to-image generation, persisted PNG download, a second image-to-image generation and metadata round-trip.
5. ZIP packaging verifies the generated PNG hashes before publishing a release. Publication requires the source commit's explicit release marker; failures publish no executable.

The release's `report.json`, original PNG, revised PNG and generation logs are the runtime evidence. The native acceptance images are 256 × 256; the first uses one sampling step and the revision uses four requested steps with strength 0.5. These are execution tests, not image-quality benchmarks.

Core tests include bounded collection, cancellation, duplicate/split isolation, source metadata, export, GitHub contribution sequencing, authenticated TLS relay communication, AES-GCM tamper rejection, job leases, memory reservations, hardware recommendations, reference validation, feedback persistence, sufficient-evidence preference selection and PIN/recovery behaviour.

The relay test uses actual local TLS HTTP servers and encrypted transport. It does not establish latency or throughput across a public internet deployment. Openverse collection is tested against API fixtures; availability and licence metadata from live indexes can vary. The optional SDXL/LoRA/CogVideoX worker is contract-tested, not GPU acceptance-tested.

Not established by these gates: universal Windows/driver compatibility, perfect realism, consistent characters across arbitrary views, native neural upscaling, pooled internet VRAM, autonomously improved foundation models, or performance superior to other generators. The included technology report identifies separate integration candidates.
