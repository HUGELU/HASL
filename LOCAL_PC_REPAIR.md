# ORIGIN-0: reproduce and repair on the owner's PC

## Current status — 11 September 2026

The owner reports that the published application still does not work and does not meet the intended experience. The exact error, failing action and local hardware/runtime state have not been supplied. The cloud development session cannot see the owner's desktop. Do not infer a cause or report the local failure fixed from CI results alone.

Repository: https://github.com/HUGELU/HASL
Working branch: origin0-media-studio; PR #6.
Preserved documentation checkpoint before this handoff: c5bb77094cf6c1edee3de98442dda6d673c11ad2.
Published v1.9.0 application commit: 013e48806a2bb1fbf461fb3e6475c0960ab817f2.
Executable SHA-256: c46316e3090115fd4cbee80da5cb3219dd3598a6a009a0c4463a2e555512e06b.

Read RECOVERY_MEDIA_STUDIO.md and MEDIA_STUDIO.md. Preserve all previous branches, releases and useful components. Main has not been merged; the earlier automatic approval requirement for that particular merge remains unresolved. This handoff does not grant or change machine permissions.

## Local session setup

Use the official Windows desktop app with Codex in Windows-native/local execution. Open a local project folder for this repository. If there is an existing checkout, inspect its branch and dirty work before switching anything; otherwise create a separate checkout of origin0-media-studio. Enable Computer Use in the desktop app if it is available for the account/region, and approve access to ORIGIN and the browser for this repair. Verify which machine the tools actually operate on before claiming a local test. On Windows, Computer Use uses the active foreground desktop.

Official setup references:
- https://learn.chatgpt.com/docs/windows/windows-app
- https://learn.chatgpt.com/docs/computer-use

## First task: one real reproduction and verified repair

1. Locate the user's actual executable and data directory; compare version/hash. Preserve uncommitted source work and the original data. Before any migration or repair that changes stored data, stop ORIGIN normally and make a recoverable copy. Reuse existing model files.
2. Reproduce the user's failing action in the visible interface. If the action is unknown and not apparent, ask the owner to show it once. Record the exact error and stage: process launch, browser/session, model download, engine startup, model loading, generation, output display, or unsatisfactory output quality. These require different fixes.
3. Read Creative studio > Engine status and export ORIGIN0_STUDIO_DIAGNOSTICS.json when available. If the UI cannot open, inspect the console and the actual data directory's logs/runtime.log and media-studio/comfy-runtime.log. Redact session tokens and private paths from shared evidence; do not copy provider-keys.json or unrelated personal material.
4. Record Windows version, CPU, RAM, free disk space, GPU identity/driver and the backend the engine actually uses. The owner's described laptop is a Samsung Galaxy Book5 Pro 360, 32 GB RAM / 1 TB disk; verify the machine rather than assuming that this is the current test device.
5. Fix the evidenced cause on an isolated branch/worktree. Retain the existing Open-Higgsfield adaptation, ComfyUI workflows, Matrix library, model providers, media, privacy, Common Vision and learning tools. Do not substitute another unconnected mock interface.
6. Repeat the same failing flow, then generate and save an actual image using an installed compatible model. Test a reference revision and queue cancellation where relevant. Record model, backend, prompt, seed, dimensions, steps, elapsed time, memory, output and errors. Ask the owner to judge image usefulness separately from execution success. Run regression tests appropriate to the change and preserve rollback.
7. If blocked, report the last output, elapsed time and specific dependency/access problem. Do not endlessly wait or silently switch to hosted generation. Package a new version only after the changed flow works; preserve v1.9.0 and its evidence.

## Next development direction after the repair

The current coding lab supports selected source proposals and bounded numeric-kernel rebuilding. It is not a general autonomous coding agent or a demonstrated ChatGPT competitor.

Build further around a capable configured coding model, repository retrieval, explicit tasks, isolated source edits, executable tests, benchmark comparison and a recoverable promotion step. Start with one measured improvement on this PC. Record quality, latency and memory against a fixed baseline. Failed candidates retain their evidence and do not replace the stable version. Extend image-model learning through reviewed datasets and evaluated adapters; preference scores, generated outputs and graph-cycle counts are not themselves proof of better model weights. Treat new representations or languages as experiments until they improve measured work. Preserve separate human review for foundational values and consequential actions.
