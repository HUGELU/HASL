# ORIGIN-0 v1.5: local models and rebuilding

For the tested native image generator, use **Image studio → Set up image engine**.
That path bundles stable-diffusion.cpp, downloads a verified Z-Image-Turbo model
pack, and needs no Python, paid API or separate compiler. See README_FIRST.txt.

The instructions below cover the **separate optional SDXL/LoRA/video workbench**.
Those optional workers require their own Python environment and model folders.
Their contract tests pass; GPU training and video rendering have not been
validated in this release. They do not automatically retrain the native Z-Image
model. The graph exploration, UI laboratory, snapshots and small taught recognizer
also run in the host independently of those optional workers.

## Local model setup on Windows

1. Install a real Python 3.12 interpreter if you do not have a model environment already. Use https://www.python.org/downloads/windows/ . A Microsoft Store execution alias is not an installed interpreter. The host runs independently of Python.
2. Create a dedicated environment using the full path to the installed interpreter. In Command Prompt, for example:

   ```bat
   "C:\Path\To\Python312\python.exe" -m venv C:\OriginModels\venv
   ```

3. Use the official PyTorch selector at https://pytorch.org/get-started/locally/ . Select your OS and actual GPU compute platform. Run its install command through **C:\OriginModels\venv\Scripts\python.exe -m pip**, so the package is installed into the same environment. Do not guess a CUDA wheel based solely on the GPU's name.
4. From the extracted ORIGIN-0 package folder, install the worker requirements:

   ```bat
   C:\OriginModels\venv\Scripts\python.exe -m pip install -r worker\requirements.txt
   ```

5. Obtain a complete **Diffusers-format** SDXL model folder with safetensors weights and model_index.json. The base model reference is https://huggingface.co/stabilityai/stable-diffusion-xl-base-1.0 . The optional 49-frame video reference is https://huggingface.co/zai-org/CogVideoX-2b . These downloads are large; choose the model components you intend to use. See each model's terms and hardware guidance.

   The Hugging Face CLI supports downloading a repository into a chosen directory: https://huggingface.co/docs/huggingface_hub/guides/cli . With the CLI installed in this environment, examples are:

   ```bat
   C:\OriginModels\venv\Scripts\hf.exe download stabilityai/stable-diffusion-xl-base-1.0 --local-dir C:\OriginModels\sdxl
   C:\OriginModels\venv\Scripts\hf.exe download zai-org/CogVideoX-2b --local-dir C:\OriginModels\cogvideox-2b
   ```

   ORIGIN-0 itself loads only the local directory. A single checkpoint file or a model repository name is not enough. Prefer safetensors; the worker requests that format. Downloading both full and alternate precision variants can consume extra disk space.
6. Open **Learning & upgrades → Connect local models and a compiler**. Enter your environment's full python.exe path and model-folder paths. Save the setup, then select **Run diagnostics**. Inspect **Show log** and result.json. The tensor probe runs an actual matrix multiplication on the device PyTorch selects. Missing packages and unavailable GPUs are reported.
7. Try **Generate local image** first. The worker uses a 512 × 512 comparison size. LoRA training and this CogVideoX implementation require a functioning CUDA GPU. Image inference has a CPU fallback, which can be slow. GPU memory exhaustion leaves the prior active adapter unchanged; the job records the error. No actual SDXL/LoRA or CogVideoX GPU run was validated for this optional worker. The native Z-Image path has its own real CPU image validation.

## Teach, train, compare

Upload files in Workspace. In Learning & upgrades, select an uploaded example, enter its category, and save it. Supply at least two categories with five independent groups each to evaluate the built-in recognizer. Each category's successive independent groups are assigned training/training/training/validation/audit; roles remain fixed. Give related images the same group name. Repeated bytes and identical measured feature vectors do not become independent evidence. These simple features cannot distinguish every meaningful semantic difference.

For image training, also write an accurate caption for each image. The worker needs three independent captioned training groups and a separate captioned validation group. It trains only SDXL UNet LoRA parameters, using up to 64 training images and 16 validation images. A small bounded experiment is not foundation-model pretraining.

Choose a **fixed validation prompt**, seed and training step budget. A training job saves:

- baseline.png and candidate.png, rendered with matching prompt/seed/size/steps;
- a safetensors LoRA adapter and adapter.zip;
- step losses, validation history, selected checkpoint and dataset identities;
- a persistent job log.

The lowest validation denoising loss selects a checkpoint. It does not establish better realism. Use **View results → Review adapter** and explain what improved or worsened. A preferred reviewed adapter becomes available for subsequent local image generation. Rejecting the active adapter returns to the base model. Your previous weights remain archived.

**Automatic training** can run when new captioned teaching evidence arrives. It respects Pause, one active local job, a per-job time limit, and a session allowance of 1–10 training runs. Each evidence revision is attempted once per session, including failures. Starting a new training experiment consumes one allowance. The engine does not automatically promote an adapter on loss alone.

The baseline recognizer compares six fixed algorithm/metric combinations. A candidate is promoted only if its balanced validation accuracy exceeds the incumbent and is at least 60%. Audit results are reported afterward and do not choose the candidate. Small and repeatedly viewed validation/audit sets can still mislead; use fresh independent evidence before making stronger performance claims.

## Rebuild a source candidate

Install a current Go compiler compatible with go.mod from https://go.dev/doc/install . In the local setup, enter its complete path, for example C:\Program Files\Go\bin\go.exe. Python is not needed for rebuilding from inside ORIGIN-0.

After the recognizer has a validated model, select **Test and build next Windows executable**. You can also enable **Rebuild after a recognition improvement** for this session.

The app reconstructs its embedded source, generates a pure numeric L1/L2 recognition kernel, runs the native host tests and go vet, then cross-compiles/compiles a new Windows x64 executable. Build commands use the local toolchain with network module downloads disabled. A failed test or build produces no executable archive. The current executable is never overwritten.

Download the completed job's archive and extract it to a new folder. It contains the new executable, changed source, tests and a checksum manifest. The compiler tests run on the build computer; a Windows target built on Linux still needs a Windows launch check. The current automatic source mutation scope is this numeric kernel. Model-written arbitrary source edits are review proposals, not executed code.

## Files and limits

Job working files remain in origin0_data/jobs. Downloadable outputs also enter the managed raw-object archive. The raw-object quota does not cover model folders, compiler caches, job workspaces or snapshots; those still need disk space. Copy the full data folder while stopped for a complete backup.

Model paths, compiler paths and automatic job settings are session configuration. API credentials never enter local job manifests. Logs may include local paths, prompts or captions you deliberately submitted. Pausing the cognitive engine prevents new automatic learning/model jobs; a currently running model/build job has its own Cancel control and time limit. Save and stop cancels a running worker.

There is no live connection to this ChatGPT conversation, general-purpose autonomous software engineering, GPU graph kernel, video-model training, or demonstrated improvement beyond established AI systems. Direct API model help and the manual ChatGPT handoff remain available. FLUX remains inspiration, not an integrated dependency.

Primary implementation references, checked 2026-09-08:
- https://huggingface.co/docs/diffusers/training/lora
- https://huggingface.co/docs/diffusers/api/pipelines/cogvideox
- https://huggingface.co/docs/diffusers/api/pipelines/stable_diffusion/stable_diffusion_xl
- https://pytorch.org/get-started/locally/
