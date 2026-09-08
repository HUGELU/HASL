"""ORIGIN-0 local model worker. Only explicit local files; never installs or downloads.

Implements SDXL inference/UNet LoRA training and CogVideoX text-to-video.
Training loss measures denoising error; a human comparison decides visual usefulness.
"""
import contextlib
import importlib.metadata
import importlib.util
import json
import math
import os
from pathlib import Path
import random
import sys
import time
import zipfile

os.environ["HF_HUB_OFFLINE"] = "1"
os.environ["TRANSFORMERS_OFFLINE"] = "1"
os.environ["HF_HUB_DISABLE_TELEMETRY"] = "1"


def emit(event, **values):
    print(json.dumps({"event": event, **values}, allow_nan=False), flush=True)


def validate_job(job):
    if job.get("kind") not in {"diagnostics", "image", "video", "train_lora"}:
        raise ValueError("Unknown local job")
    steps = job.get("steps", 25)
    if isinstance(steps, bool) or not isinstance(steps, int) or not 1 <= steps <= 2000:
        raise ValueError("Steps must be an integer from 1 to 2000")
    if job["kind"] != "train_lora" and steps > 80:
        raise ValueError("Generation step budget is 80")
    out = Path(job["output_dir"]).resolve()
    if not out.is_dir():
        raise ValueError("Output directory must already exist")
    if job["kind"] != "diagnostics":
        model = job.get("video_model" if job["kind"] == "video" else "image_model", "")
        if not model or not Path(model).is_dir():
            raise ValueError("A complete local Diffusers model folder is required")
        if not (Path(model) / "model_index.json").is_file():
            raise ValueError("Choose a Diffusers-format model folder containing model_index.json")
    if job["kind"] == "train_lora":
        train, val = job.get("train", []), job.get("validation", [])
        tg = {x["group"] for x in train}
        vg = {x["group"] for x in val}
        if len(tg) < 3 or not vg or tg & vg:
            raise ValueError("Training requires three independent training groups and a separate validation group")
        if {x["asset"] for x in train} & {x["asset"] for x in val}:
            raise ValueError("An asset cannot be in both training and validation")
        if any(not x.get("caption", "").strip() or not Path(x["path"]).is_file() for x in train + val):
            raise ValueError("Every training/validation image needs a caption and an existing file")
    return out


def diagnostics():
    result = {"python": sys.version.split()[0], "packages": {}, "device": "unavailable"}
    for package in ("torch", "diffusers", "transformers", "peft", "accelerate", "safetensors", "Pillow", "imageio-ffmpeg"):
        try:
            result["packages"][package] = importlib.metadata.version(package)
        except importlib.metadata.PackageNotFoundError:
            result["packages"][package] = "missing"
    try:
        import torch
        device = "cuda" if torch.cuda.is_available() else "mps" if torch.backends.mps.is_available() else "cpu"
        torch.set_num_threads(max(1, min(8, os.cpu_count() or 1)))
        a = torch.arange(256 * 256, dtype=torch.float32, device=device).reshape(256, 256) / 65536
        start = time.perf_counter()
        product = a @ a.T
        checksum = float(product.sum().item())  # Synchronizes the actual device calculation.
        if not math.isfinite(checksum):
            raise ValueError("Device returned a non-finite result")
        result.update(device=device, tensor_probe_ms=(time.perf_counter() - start) * 1000,
                      tensor_checksum=checksum, cuda=torch.version.cuda)
        if device == "cuda":
            result.update(gpu=torch.cuda.get_device_name(0),
                          vram_bytes=torch.cuda.get_device_properties(0).total_memory)
    except Exception as exc:
        result["error"] = str(exc)
    emit("diagnostics", **result)
    return result


def gpu_context(require_cuda=False):
    import torch
    torch.set_num_threads(max(1, min(8, os.cpu_count() or 1)))
    device = "cuda" if torch.cuda.is_available() else "cpu"
    if require_cuda and device != "cuda":
        raise RuntimeError("This training/video worker requires a working CUDA GPU runtime. Diagnostics reports what is available.")
    return torch, device, torch.float16 if device == "cuda" else torch.float32


def progress(pipe, step, timestep, values):
    emit("inference_step", step=step + 1)
    return values


def sdxl_pipeline(job, torch, device, dtype):
    from diffusers import StableDiffusionXLPipeline
    emit("loading", model=job["image_model"], device=device)
    pipe = StableDiffusionXLPipeline.from_pretrained(
        job["image_model"], local_files_only=True, use_safetensors=True, torch_dtype=dtype)
    pipe.set_progress_bar_config(disable=True)
    pipe.enable_vae_slicing()
    pipe.enable_vae_tiling()
    pipe.to(device)
    return pipe


def render(pipe, prompt, seed, steps, device, torch):
    # A matched seed/prompt/size supports before-after inspection.
    return pipe(prompt=prompt, height=512, width=512, num_inference_steps=steps,
                generator=torch.Generator(device=device).manual_seed(seed),
                callback_on_step_end=progress).images[0]


def generate_image(job, out):
    torch, device, dtype = gpu_context()
    pipe = sdxl_pipeline(job, torch, device, dtype)
    adapter = job.get("adapter")
    if adapter:
        pipe.load_lora_weights(adapter, weight_name="pytorch_lora_weights.safetensors",
                               local_files_only=True, use_safetensors=True)
    with torch.inference_mode():
        render(pipe, job["prompt"], job["seed"], job["steps"], device, torch).save(out / "generated.png")
    return {"device": device, "model": job["image_model"], "adapter": adapter or "",
            "prompt": job["prompt"], "seed": job["seed"], "steps": job["steps"],
            "generated": True, "size": [512, 512]}


def generate_video(job, out):
    from diffusers import CogVideoXPipeline
    from diffusers.utils import export_to_video
    torch, device, dtype = gpu_context(require_cuda=True)
    emit("loading_video", model=job["video_model"], device=device)
    pipe = CogVideoXPipeline.from_pretrained(
        job["video_model"], local_files_only=True, use_safetensors=True, torch_dtype=dtype)
    pipe.set_progress_bar_config(disable=True)
    pipe.enable_model_cpu_offload()
    pipe.vae.enable_tiling()
    with torch.inference_mode():
        frames = pipe(prompt=job["prompt"], num_inference_steps=job["steps"], num_frames=49,
                      generator=torch.Generator(device=device).manual_seed(job["seed"]),
                      callback_on_step_end=progress).frames[0]
    export_to_video(frames, str(out / "generated.mp4"), fps=8)
    return {"device": device, "model": job["video_model"], "frames": len(frames), "fps": 8,
            "prompt": job["prompt"], "seed": job["seed"], "steps": job["steps"], "generated": True,
            "architecture": "CogVideoX (49-frame checkpoints)"}


def train_lora(job, out):
    import numpy as np
    from PIL import Image, ImageOps
    from diffusers import DDPMScheduler, StableDiffusionXLPipeline
    from diffusers.utils import convert_state_dict_to_diffusers
    from peft import LoraConfig, get_peft_model_state_dict, set_peft_model_state_dict
    torch, device, dtype = gpu_context(require_cuda=True)
    torch.manual_seed(job["seed"])
    random.seed(job["seed"])
    pipe = sdxl_pipeline(job, torch, device, dtype)
    with torch.inference_mode():
        render(pipe, job["prompt"], job["seed"], 20, device, torch).save(out / "baseline.png")
    pipe.vae.to(dtype=torch.float32)
    for module in (pipe.unet, pipe.vae, pipe.text_encoder, pipe.text_encoder_2):
        module.requires_grad_(False)
    pipe.unet.add_adapter(LoraConfig(r=4, lora_alpha=4,
                                    target_modules=["to_q", "to_k", "to_v", "to_out.0"]))
    for p in pipe.unet.parameters():
        if p.requires_grad:
            p.data = p.data.float()
    pipe.unet.enable_gradient_checkpointing()
    scheduler = DDPMScheduler.from_config(pipe.scheduler.config)
    params = [p for p in pipe.unet.parameters() if p.requires_grad]
    optimizer = torch.optim.AdamW(params, lr=1e-4)
    scaler = torch.amp.GradScaler("cuda")
    selected_train = sorted(job["train"], key=lambda x: x["asset"])[:64]
    selected_val = sorted(job["validation"], key=lambda x: x["asset"])[:16]
    emit("preparing_training", training=len(selected_train), validation=len(selected_val), rank=4)

    def encode(row):
        with Image.open(row["path"]) as source:
            if source.width * source.height > 16_000_000:
                raise ValueError("Training image exceeds 16 million pixels")
            source = ImageOps.exif_transpose(source).convert("RGB")
            w, h = source.size
            fitted = ImageOps.fit(source, (512, 512), method=Image.Resampling.LANCZOS)
            pixels = np.asarray(fitted, dtype=np.float32).copy() / 127.5 - 1
        tensor = torch.from_numpy(pixels).permute(2, 0, 1).unsqueeze(0).to(device, torch.float32)
        with torch.no_grad():
            latent = pipe.vae.encode(tensor).latent_dist.mode() * pipe.vae.config.scaling_factor
            emb, _, pooled, _ = pipe.encode_prompt(row["caption"], device=device, do_classifier_free_guidance=False)
        tids = torch.tensor([[h, w, 0, 0, 512, 512]], dtype=emb.dtype)
        return latent.cpu(), emb.cpu(), pooled.cpu(), tids

    train = [encode(row) for row in selected_train]
    validation = [encode(row) for row in selected_val]
    # Frozen encoders and VAE can leave the GPU during UNet optimization.
    for module in (pipe.vae, pipe.text_encoder, pipe.text_encoder_2):
        module.to("cpu")
    torch.cuda.empty_cache()

    def loss_for(record, seed):
        latent, emb, pooled, tids = [x.to(device, dtype) for x in record]
        g = torch.Generator(device=device).manual_seed(seed)
        noise = torch.randn(latent.shape, device=device, dtype=dtype, generator=g)
        times = torch.randint(0, scheduler.config.num_train_timesteps, (1,), device=device, generator=g)
        noisy = scheduler.add_noise(latent, noise, times)
        target = noise
        if scheduler.config.prediction_type == "v_prediction":
            target = scheduler.get_velocity(latent, noise, times)
        elif scheduler.config.prediction_type != "epsilon":
            raise ValueError("Unsupported diffusion training target")
        with torch.autocast("cuda", dtype=dtype):
            output = pipe.unet(noisy, times, encoder_hidden_states=emb,
                               added_cond_kwargs={"text_embeds": pooled, "time_ids": tids}).sample
        return torch.nn.functional.mse_loss(output.float(), target.float())

    def evaluate():
        pipe.unet.eval()
        with torch.no_grad():
            value = sum(loss_for(row, 900001 + i).item() for i, row in enumerate(validation)) / len(validation)
        pipe.unet.train()
        if not math.isfinite(value):
            raise ValueError("Non-finite validation loss; candidate discarded")
        return value

    def weights():
        return {k: v.detach().float().cpu().clone() for k, v in get_peft_model_state_dict(pipe.unet).items()}

    before_loss = evaluate()
    best_loss, best_step, best = before_loss, 0, weights()
    history = [{"step": 0, "validation_loss": before_loss}]
    order = list(range(len(train)))
    for step in range(1, job["steps"] + 1):
        if (step - 1) % len(order) == 0:
            random.shuffle(order)
        optimizer.zero_grad(set_to_none=True)
        loss = loss_for(train[order[(step - 1) % len(order)]], job["seed"] + step + 100)
        if not torch.isfinite(loss).item():
            raise ValueError("Non-finite training loss; existing adapter retained")
        scaler.scale(loss).backward()
        scaler.unscale_(optimizer)
        torch.nn.utils.clip_grad_norm_(params, 1)
        scaler.step(optimizer)
        scaler.update()
        emit("training_step", step=step, total=job["steps"], loss=float(loss.item()))
        if step % 10 == 0 or step == job["steps"]:
            val_loss = evaluate()
            history.append({"step": step, "validation_loss": val_loss})
            emit("validation", step=step, loss=val_loss, initial=before_loss)
            if val_loss < best_loss:
                best_loss, best_step, best = val_loss, step, weights()
    set_peft_model_state_dict(pipe.unet, best)
    adapter_dir = out / "adapter"
    adapter_dir.mkdir(exist_ok=True)
    StableDiffusionXLPipeline.save_lora_weights(
        str(adapter_dir), unet_lora_layers=convert_state_dict_to_diffusers(best), safe_serialization=True)
    # Use the same baseline inputs. Numeric improvement alone never activates this adapter.
    pipe.unet.eval()
    pipe.to(device)
    pipe.unet.to(dtype=dtype)
    pipe.vae.to(dtype=dtype)  # Let the pipeline perform its usual VAE precision upcast.
    with torch.inference_mode():
        render(pipe, job["prompt"], job["seed"], 20, device, torch).save(out / "candidate.png")
    result = {
        "device": device, "architecture": "SDXL UNet LoRA", "rank": 4, "model": job["image_model"],
        "training_examples": len(train), "validation_examples": len(validation),
        "training_assets": [x["asset"] for x in selected_train],
        "validation_assets": [x["asset"] for x in selected_val],
        "steps": job["steps"], "selected_step": best_step, "initial_validation_loss": before_loss,
        "selected_validation_loss": best_loss, "history": history, "evidence_revision": job.get("revision"),
        "seed": job["seed"], "prompt": job["prompt"], "active": False,
        "note": "Validation measures denoising loss. Review baseline.png and candidate.png for realism and prompt fidelity. No automatic visual-quality claim.",
    }
    (adapter_dir / "training.json").write_text(json.dumps(result, indent=2), encoding="utf-8")
    with zipfile.ZipFile(out / "adapter.zip", "w", zipfile.ZIP_DEFLATED) as z:
        for path in sorted(adapter_dir.iterdir()):
            if path.is_file():
                z.write(path, path.name)
    return result


def main():
    if len(sys.argv) != 2:
        raise ValueError("Usage: local_models.py path/to/job.json")
    job = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
    out = validate_job(job)
    start = time.perf_counter()
    kind = job["kind"]
    if kind == "diagnostics":
        result = diagnostics()
    elif kind == "image":
        result = generate_image(job, out)
    elif kind == "video":
        result = generate_video(job, out)
    else:
        result = train_lora(job, out)
    result["elapsed_seconds"] = time.perf_counter() - start
    tmp = out / "result.partial.json"
    tmp.write_text(json.dumps(result, indent=2, allow_nan=False), encoding="utf-8")
    tmp.replace(out / "result.json")
    emit("completed", kind=kind, seconds=result["elapsed_seconds"])


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        emit("failed", error=str(exc), exception=type(exc).__name__)
        raise
