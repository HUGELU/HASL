ORIGIN-0 v1.7 adds **Production Studio** while retaining the existing native image generator, Concept Studio, encrypted worker relay, interface experiments and saved branches.

**Extract `ORIGIN0_WINDOWS_v1.7.0.zip` and double-click `ORIGIN0.exe`.** Open **Production** to finish an imported image immediately. The upscaler and its weights are embedded. Native diffusion generation still uses the separate verified 6.52 GB first-run model download. Neither path requires Python, an API key or an ORIGIN subscription.

- **Local neural finishing:** original Real-ESRGAN x4plus weights with NCNN, tiled CPU/Vulkan execution, bounded parallel CPU workers, detail blending, fast Lanczos sizing and sharpening. Final long edge up to 7680 pixels; originals, checksums and recipes retained.
- **Connected generation:** automatic finishing after generation, explicit Finish/Revise/Add to property actions, a separate finishing queue, cancellation and reuse of identical completed recipes. Heavy local tasks share a resource gate.
- **Property workbench:** measured rectangular rooms, door positions, non-overlap checks and dimensioned SVG plans; up to 12 ordered room-associated reference photos; camera/light prompt guidance and image-revision handoff.
- **Walkthrough export:** a self-contained offline photo sequence and actual browser WebM recording at 1920×1080. Save and restore project recipes.
- **Local development lab:** a connected local coding model proposes revisions to selected numerical source, receives validation feedback across 1–3 iterations, and produces a review archive. Optional executable tests run with fixed tests in a configured restricted Docker container. Syntax-only results are labelled.
- **Integration review:** the requested Autom8AI/Open-Higgsfield-AI and the separate Higgsfield service were inspected. Useful production ideas are reflected here; their hosted model catalogues are not misrepresented as bundled, free local inference.

The release gate requires Go race/vet checks, real CPU neural inference, identical-pixel sequential/parallel tile tests, browser workflows and WebM download, responsive/touch checks, actual container tests, Windows loader checks, native diffusion, image revision and automatic neural finishing. Included reports record the actual results. Fixture timings and tiny acceptance generations are execution checks, not photorealism or comparative-quality benchmarks.

Windows x64/AVX2 is the packaged generation target. The 32 GB laptop configuration remains practical, with CPU generation taking minutes. A particular Samsung/Intel GPU or NPU is not certified by these CPU tests. An 8K export can include interpolation; estimated texture is not recovered ground truth. Plans are schematics from your measurements. Photo sequences are not navigable 3D reconstructions. General model-written source does not replace the running executable, and no model-weight improvement is inferred from extra cycles alone.

See **PRODUCTION_STUDIO.md** for operation and limits, and **INTEGRATION_REVIEW.md** for the open-tools mapping and remaining adapters. The optional SDXL/CogVideoX environment, multi-reference identity locking, calibrated 3D/BIM, lip sync and private commercial model internals are separate capabilities; this release does not claim all of them are implemented.

Application: MIT. Default diffusion weights: Apache-2.0. Real-ESRGAN and NCNN: BSD-3-Clause with notices included. Source: https://github.com/HUGELU/HASL .
