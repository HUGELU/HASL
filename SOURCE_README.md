# ORIGIN-0 v1.5 source

Go standard library only. The release includes an embedded browser interface;
users do not need a separate language runtime to execute the binary.

Build with a Go toolchain compatible with go.mod:

```sh
python3 build.py --target windows
python3 build.py --target linux
```

The helper regenerates source_snapshot.txt from the exact application source,
also embeds source_bundle.json for reproducible local reconstruction,
sets CGO_ENABLED=0, and writes the native output to release/. Python is required
only for this build helper. A supplied Windows executable runs independently.
Use --go /absolute/path/to/go to select a toolchain.

Validation used Go 1.27.1 on Linux x64:

```sh
go test -race -count=1 -v ./...
go vet ./...
node --check web/app.js
python3 scripts/verify_runtime.py
```

The runtime verifier uses the matching local release binary. It creates only
synthetic local test data and makes no external provider request. Adapter tests
use an HTTP stub; they do not demonstrate successful access to a paid model.

Core files:
- main.go: bounded ingestion, graph operations, descendant memories, startup.
- laboratory.go: layouts, measurements, archives, local API and exports.
- media.go: explicit model adapters and manual ChatGPT handoff.
- learning.go: labels, group-preserving splits, candidate evaluation and rollback.
- evolution_jobs.go: local worker jobs, cancellation and tested source rebuilding.
- adaptive_kernel.go: the restricted generated numeric source target.
- worker/local_models.py: SDXL inference/LoRA training and CogVideoX video.
- web/: embedded application UI, styles, candidate-template measurements.
- *_test.go: regression and concurrency coverage.

Model endpoint references checked on 2026-09-08:
- https://developers.openai.com/api/docs/guides/text
- https://developers.openai.com/api/docs/guides/image-generation
- https://developers.openai.com/api/docs/guides/speech-to-text

Source snapshot text is input material, not an executable instruction channel.
Runnable branch export archives this host binary and state. It does not compile
or install a source modification. See README_FIRST.txt for operational limits.

Local model execution requirements and its unvalidated GPU integration boundary
are documented in LOCAL_MODELS.md. Source rebuilding in Learning & upgrades is
separate from the ordinary runnable-branch export.
