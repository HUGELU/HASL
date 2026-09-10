# Concept Studio — v1.6

Open **Concept Studio** in the existing executable. Save a project with a subject
and editable attributes. Position sliders and custom attributes compose a prompt
that can be sent to the native Image studio. These are prompt controls; they do
not guarantee exact geometry or retrain model weights.

## Build a dataset

- Upload/drop PNG or JPEG files, or search the Openverse image index by keyword.
- Optional automatic collection saves at most 24 results per search, with a
  four-minute deadline and Stop control. Each image is limited to 12 MB and
  16 million pixels. Network errors, unsupported images and rate limits are shown.
- Creator, source URL, licence metadata, SHA-256 identity and collection query
  accompany images. Openverse metadata is not independent verification of rights.
- A self-hosted SearXNG instance can supply additional keyword/image searches,
  including its configured engines. Enable JSON in its configuration. Its search
  results generally lack licence metadata and are imported after recording rights.
- Each collected image starts pending. Give it a category and an accurate caption,
  then accept or exclude it. Project attributes are not automatically asserted as
  facts about every found image.
- Exact duplicates are one example. A conservative dHash/spatial-feature comparison
  groups near duplicates; inspect the groups, since no heuristic detects every copy.
- Groups stay together in training, validation or audit data. The sequence for new
  groups is three training groups, one validation group and one audit group.
  These splits help detect leakage but do not substitute for a large independent
  benchmark. Imports are limited to 500 images per project and 24 projects per state.

## What actually learns

**CPU project classifier:** accepted examples train six centroid/nearest-neighbour
candidates using explicit spatial RGB features. At least two categories, each
with three training groups, one validation group and one audit group, are needed.
Selection uses balanced validation accuracy; audit results are reported after
selection. Automatic evaluation runs when accepted examples change and enough
data exists. The previous classifier is kept for restore. This classifier is not
a general vision model and does not modify the diffusion generator.

**Exact logical exercises:** create 20 generated diagrams for left/right,
above/below, on/off, inside/outside or red/blue. Their labels come from construction.
They are useful for checking the learning path, not evidence of photographic
understanding. Use a separate project for each pair when first testing.

**Image adapter training:** the existing bundled SDXL LoRA worker accepts only the
selected project's accepted training and validation images. Audit images remain
excluded. Configure the worker in **Learning & upgrades** with a real Python/CUDA
environment and compatible local SDXL weights, then start the project adapter.
Logs and base/candidate comparison images appear there. Activation still requires
the measured validation result and visual review. CPU-only laptops can collect,
label, run the small classifier and export datasets; this worker does not supply
GPU training on those laptops. Native Z-Image-Turbo GGUF weights are not trainable
through this SDXL path. SDXL's model licence is separate from Apache-licensed
Diffusers/PEFT and is not an unrestricted Apache model licence.

## Reuse and contribute

**Recipe JSON** exports the subject, attributes, accepted example metadata,
provenance and evaluation runs. It includes no image bytes, keys or local paths.
Review the visible captions and source URLs before sharing.

**Training dataset ZIP** also includes accepted images and `metadata.jsonl`
files in `train`, `validation` and `audit` directories. Metadata uses `file_name`
and `text` columns for image-folder/caption workflows. Its grouped splits must
be preserved in downstream trainers. Audit data is not for fitting parameters.

Contribute compact recipes and reproducible code changes through GitHub pull
requests. Large datasets and weights belong in an appropriate dataset/model
registry with their own licence and model/dataset card. Adding an image to a local
project does not automatically improve the shared kernel or publish that image.

Projects and classifiers are part of saved application state and snapshots.
Search/collection grants and search-instance settings are session-only. Restoring
a saved state does not grant new network access. The existing worker-PC protocol
continues to schedule complete generation jobs; this stage does not make its
devices a shared VRAM address space.

See [the technology report](TECHNOLOGY_REPORT.md) for the researched internet,
distributed-training, source-evolution and full-interface reconstruction path.
