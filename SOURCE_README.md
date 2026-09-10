# Building ORIGIN-0 v1.6

See README.md for capabilities and limits. Go 1.24+ builds the native host;
Python 3 runs the build helper and validation scripts only. End users do not
need Python for the default image studio.

`python3 build.py --target windows` builds Windows x64.
`python3 build.py --target linux` builds Linux x64.
`--arch arm64 --target darwin` builds an Apple silicon host, but this release
has not completed a Mac validation gate. A Windows binary is not an Android app.

Before building, the helper embeds an exact source snapshot and JSON bundle.
A release can place verified native runtime ZIPs in bundled/. Runtime binaries
are not in Git; model_catalog.json pins the downloads. scripts/build_native_windows.ps1
builds the pinned backend on Windows with its C++ runtime linked statically, then
checks PE imports before packaging. It intentionally leaves the import gate intact.

Run `go test -race ./...`, `go vet ./...` and JavaScript syntax checks. The real
image integration gate is scripts/verify_image_runtime.py; it downloads and
verifies actual weights, starts the app, completes native inference and fetches
the persisted PNG. Fixture-based unit tests are labelled separately.

The source bundle and rebuildFiles list must remain consistent. An experimental
recognition improvement may rebuild the small numeric distance kernel through
an explicitly configured Go compiler. That produces an archive, not silent
replacement of the active executable or training of the Z-Image model.
