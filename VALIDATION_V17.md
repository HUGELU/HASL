# v1.7 release evidence

Publication requires all checks in `.github/workflows/validate.yml` to pass for
the tagged commit. The release includes actual reports and output files. Source
checks alone cannot publish a Windows executable.

| Gate | Evidence |
|---|---|
| Go correctness/concurrency | `go test -race ./...` and `go vet ./...` |
| Native neural CPU, Windows/Linux | `scripts/verify_upscaler_native.py`: actual Real-ESRGAN weights, dimensions, output variation, identical sequential/parallel pixels |
| Local source candidate execution | `TestCandidateInRealDockerSandbox` in a digest-pinned official Go container, with the runtime restrictions in `development.go` |
| Actual app production | `scripts/verify_production_api.py`: 7680-pixel long-edge export, neural output, recipes, cache, original preservation, SVG and photo project |
| Browser/touch | Existing Concept Studio/PIN gate plus `scripts/verify_production_browser.py`: upload, finish, download, room association, reference handoff, project restore, actual WebM and six screen widths |
| Windows package | Loader dependencies, native Z-Image generation, image-to-image revision, automatic neural finishing, separate production acceptance, artifact checksums |

The pinned upscaler source build is run
[34481515851](https://github.com/HUGELU/HASL/actions/runs/34481515851), commit
`1fae18f89c70b9bb8d9b8128614ce01b6af207e6`. It passed on both Windows and Linux.
Its checksum-pinned archives can be reused only while the source fingerprint in
`native/upscale/builds.json` matches. The app is rebuilt and re-exercised afterward.
If those build artifacts expire, the preparation script rebuilds from pinned
dependencies and the current native source.

On the Linux runner's small 96×80 synthetic tile fixture, one CPU worker took
4.89 s and three workers took 3.04 s with identical pixels. This is a narrow
scheduling measurement, not a claim about full-size photo throughput or GPU speed.

The local coding-model transport test uses a deterministic local response fixture.
The container test uses the baseline source as its candidate. These establish the
adapter and test path, not the competence or performance of any downloaded LLM.
Vulkan driver coverage, Samsung laptop performance, inferred-detail accuracy,
seam/face/logo quality and comparison with commercial generators need separate
hardware and image-quality evaluation. No general superiority claim is made.
