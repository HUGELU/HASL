# Models, references and media in ORIGIN-0 v1.6

This is a capability map, not a claim that every listed model is installed.
Checked against primary project documentation on 10 September 2026. No single
model is reliably best at every prompt, device, identity, layout or style.

| Model / runtime | Useful role | Licence | ORIGIN-0 status |
| --- | --- | --- | --- |
| Z-Image-Turbo + stable-diffusion.cpp | Native text-to-image and one-image revision; quantized weights, CPU/Vulkan/Metal paths | Default weights Apache-2.0; runtime MIT | Automatic verified download in Image studio; Windows CPU acceptance gate |
| Stable Diffusion XL + Diffusers/PEFT | Captioned image LoRA experiments and adapter comparisons | Diffusers/PEFT Apache-2.0; SDXL weights use their separate OpenRAIL terms | Optional existing worker; provide its model environment and folder; LoRA requires CUDA in this worker |
| CogVideoX-2b | Short text-to-video jobs | Check the exact model card and code licences | Optional existing 49-frame CUDA worker; not part of native CPU validation |
| Qwen-Image-Edit-2511 | Multiple reference images, instruction editing and improved character consistency | Apache-2.0 model card | Researched integration candidate, not connected to the native Z-Image controls |
| Wan2.2 | Text/image-to-video generation; 5B and larger variants | Apache-2.0 project | Researched integration candidate; separate video runtime/model download needed |
| Real-ESRGAN / ncnn Vulkan | Tiled learned upscaling and restoration | BSD-3-Clause project | Researched integration candidate; neural 4K/8K upscaling is not bundled |
| whisper.cpp | Local multilingual transcription on several CPU/GPU backends | MIT | Researched native packaging candidate; current Voice to prompt uses the configured transcription provider |
| OpenVINO | Compatible Intel CPU, GPU and NPU inference | Apache-2.0 | Candidate for model-specific conversion and benchmarks; a detected Intel NPU is not used by this release |

FLUX remains an inspiration reference, consistent with the project's earlier
design constraint. Higgsfield is a hosted commercial service with its own access,
usage and billing terms; it is not a bundled unrestricted open-source runtime.
ORIGIN-0 does not use somebody else's account or unvolunteered compute.

## Useful workflows available now

**Photography or interiors:** describe the subject, viewpoint, geometry,
lighting and material texture. The visible prompt helpers add a starting style;
you can inspect and edit every word. Try several random seeds, rate the results,
and save settings for the result you want to revise. More sampling steps do not
necessarily improve a distilled model; the balanced native preset uses eight.

**A recurring fictional character or product:** create a Concept Studio project
with stable appearance, wardrobe, scene and role attributes. Review a set of
reference images with accurate captions and independent groups. Use the same
project prompt and a selected image as the next revision's starting point.
Optional project-specific SDXL LoRA training can compare an adapter on separate
validation examples. Native Z-Image weights are not modified by that SDXL worker.

**Brand accuracy:** retain original logo/product assets and review their exact
appearance separately. A diffusion model can change lettering or geometry.
Pixel-exact logo compositing, packaging templates and multi-view product locks
need additional implementation; the reference slider is not a substitute.

**Feedback:** the four ratings are entered by the user, not invented by a model.
For an exact prompt and reference, three or more rated examples of a setting
combination can support reusing the highest-rated combination. This is a local
preference heuristic over width, height, steps and revision strength. It neither
establishes causality nor trains a new foundation model. Synthetic diagrams in
Concept Studio have explicit constructed labels; generated photographs do not
become verified real-world evidence automatically.

## Next integration gates

Before calling another model supported, add a pinned downloadable manifest,
verify licences and memory requirements, implement the exact input/output
contract, and run a real generation on its target device. Keep timing, peak
memory, prompt match and consistency evidence separate. Preserve the current
working model and settings for rollback.

For character continuity, evaluate held-out views and scenes; a repeated seed is
not an identity guarantee. For upscaling, inspect seams, lettering and invented
detail; a larger pixel count does not recreate unknown real detail. For video,
measure temporal consistency and actual frame rate, not only individual frames.
These remain engineering tests to implement, rather than claims of perfect
quality or superiority over commercial generators.

## Primary references

- https://github.com/leejet/stable-diffusion.cpp/blob/d04e8950c1ec8d30248cbe996682b3182fb1adf6/docs/z_image.md
- https://huggingface.co/Tongyi-MAI/Z-Image-Turbo
- https://huggingface.co/docs/diffusers/training/lora
- https://huggingface.co/stabilityai/stable-diffusion-xl-base-1.0
- https://huggingface.co/zai-org/CogVideoX-2b
- https://huggingface.co/Qwen/Qwen-Image-Edit-2511
- https://github.com/Wan-Video/Wan2.2
- https://github.com/xinntao/Real-ESRGAN
- https://github.com/xinntao/Real-ESRGAN-ncnn-vulkan
- https://github.com/ggml-org/whisper.cpp
- https://github.com/openvinotoolkit/openvino
- https://higgsfield.ai/terms-of-use-agreement
