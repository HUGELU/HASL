# Third-party provenance

ORIGIN-0 application: MIT, copyright HUGELU and contributors. See ../LICENSE.

Native backend: leejet/stable-diffusion.cpp, commit
`d04e8950c1ec8d30248cbe996682b3182fb1adf6` (MIT). Its release also contains
GGML and image/video encoding libraries. Runtime licence notices must accompany
those binaries; the native release packaging preserves them. The top-level
backend licence is in stable-diffusion.cpp-LICENSE.txt.

Default model pack (no API service):

| Component | Publisher | Revision | Licence |
|---|---|---|---|
| Z-Image-Turbo Q4_0 | leejet, derived from Tongyi-MAI | c61c0e422dc8b541b7548cf33a4ef8302b0f8085 | Apache-2.0 |
| Qwen3-4B-Instruct-2507 Q4_K_M | unsloth, derived from Qwen | a06e946bb6b655725eafa393f4a9745d460374c9 | Apache-2.0 |
| Z-Image VAE | Comfy-Org | 08d04455279082882deaabc8d0d09fc914c071e1 | Apache-2.0 |

The exact HTTPS URLs, sizes and SHA-256 digests are in ../model_catalog.json.
Apache-2.0.txt contains the licence text. Model cards remain available at the
source URLs in that manifest. We do not relabel third-party work as invented by
ORIGIN-0 or claim the licences have no obligations.

Older optional SDXL models have different terms and are not part of the default
model pack. The separate optional Python worker's requirements retain their
upstream licences.

Neural finishing: original Real-ESRGAN x4plus NCNN weights from Xintao Wang,
BSD-3-Clause. See Real-ESRGAN-LICENSE.txt. NCNN is BSD-3-Clause; its licence
notices are retained in the packaged runtime. ORIGIN's small native host is MIT.
Exact source/model download hashes appear in scripts/build_upscaler.py, and the
compiled archive hash appears in upscale_manifest.json. Neither the proprietary
Higgsfield service nor Autom8AI's MuAPI UI code is incorporated into this binary.
