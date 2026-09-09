# Contributing to ORIGIN-0

Start with a reproducible problem: failing startup, a missing device, a confusing
control, excessive memory, or a measurable image-quality limitation. Open an issue
with OS, RAM, GPU/driver, backend, model-pack ID and exact log lines. Remove private
prompts and invitation tokens before posting logs.

For a change, create a branch and a pull request. Explain the user-visible problem,
resulting behaviour, validation and remaining limitations. Run `go test ./...`,
`go vet ./...`, and JavaScript syntax checks. Download/protocol changes need tests
for cancellation, corruption and stale or unauthorized responses. Real generation
claims need the model revision, prompt, seed, settings, hardware and elapsed time.

Preserve existing user state. Never include user data, prompts, account keys,
private invitations, downloaded weights or generated credentials in a PR.
Application source uses MIT; disclose and preserve third-party licences. The
maintainer reviews executable changes and measured regressions before release.
CI results support review; this repository does not blindly merge AI proposals.

AI-assisted PRs are welcome. Name the tools used and identify which results were
actually executed. A benchmark produced by a fixture is a contract test, not
proof of image quality, learning, GPU acceleration or speedup.

Build release packages with `build.py`, and update its source list together with
`rebuildFiles` in `evolution_jobs.go` when adding embedded source dependencies.
