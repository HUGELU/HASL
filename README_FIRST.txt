ORIGIN-0 CONNECTED STUDIO v1.9.0 — WINDOWS PREVIEW

1. Extract the ZIP into a writable folder, then run ORIGIN0.exe.
2. Keep its console open; the local studio opens in your browser.
3. Open Creative studio > Models & engines.
4. Select a starter pack and click Set up & use. ORIGIN installs the required
   local engine, downloads/verifies the model, checks readiness and selects it.
   The top strip shows its actual current stage. Stop download cancels setup;
   Set up & use resumes partial downloads.
5. Enter your prompt and generate. Gallery & queue retains outputs and parameters.

MODEL SOURCES
Starter packs: complete pinned recipes, including native Z-Image and ComfyUI packs.
Hugging Face / Civitai: live search, exact versions and files, sizes and SHA-256.
Add & download stores the selected component in the shared model directory.
Choose the correct recipe: complete SD 1.5 / SDXL checkpoints generate directly;
FLUX/Qwen components and other architectures need the matching full workflow.
LoRAs need a matching base model. A downloaded component alone is not a full model.
Gated items may need your own provider token and accepted model terms.

STABILITY MATRIX
Open its tab to install the official Windows manager or link an existing Data
folder. Existing models/settings are reused without being overwritten. The actual
Matrix desktop app opens separately and manages its supported packages and training
tools. Matrix's original branding/terms remain; ORIGIN supports its own white label.
ORIGIN-managed ComfyUI reads the linked Matrix model library on its next start.
For an external ComfyUI, download the shared-path YAML, configure it and restart
that engine. Installed models lists actual checkpoint/LoRA names from the engine.

WHEN SOMETHING FAILS
Creative studio > Engine status shows actual readiness and startup logs and exports
ORIGIN0_STUDIO_DIAGNOSTICS.json. Provider keys are excluded from that export.
The app prints a concrete error for unavailable engines, inaccessible/gated provider
items, missing model components, stalled downloads and invalid workflows.

HARDWARE AND SCOPE
This package targets Windows x64. Matrix auto-install also targets Windows x64.
macOS/Linux can build ORIGIN from source and connect their existing local ComfyUI.
The package includes native generation/finishing runtimes but generation models
require GB-sized first downloads. CPU inference can take minutes. More models do
not imply faster inference. Actual Intel GPU throughput on your Samsung laptop
remains unbenchmarked. Pick a pack within your RAM and available disk space.
Advanced video, masks and multi-reference work use installed ComfyUI workflows.
This does not include proprietary hosted Higgsfield/MuAPI models or subscriptions.

PRESERVATION
Stop your older ORIGIN process before running this version on the same data.
Keep the complete origin0_data folder. It contains your settings, models and media.
Existing Common Vision, architecture, upscaling, learning and PIN tools remain.
Original 12 + 1 values are still awaiting the owner's source documents.

SOURCE / RECOVERY
https://github.com/HUGELU/HASL/tree/origin0-media-studio
See MEDIA_STUDIO.md, MEDIA_RECOVERY.md and MEDIA_VALIDATION.json for exact results.
