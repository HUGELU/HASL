# ORIGIN-0: practical open-source technology report

Research checked 9 September 2026. This is an integration map, not a claim that
all these systems are bundled or that connecting them produces general intelligence.

## Recommended components

| Capability | Existing project and core licence | What ORIGIN-0 can use it for |
| --- | --- | --- |
| Internet peers | [go-libp2p](https://github.com/libp2p/go-libp2p), MIT | Peer identities, transports, NAT traversal and relay connections. Networking does not distribute a model by itself. Reachable bootstrap/relay infrastructure still needs an operator. |
| Shared model work | [Hivemind](https://github.com/learning-at-home/hivemind), MIT; [Petals](https://github.com/bigscience-workshop/petals), MIT | Decentralized training and partitioned inference/fine-tuning of supported language models. Petals demonstrates internet model sharing, not a universal pool of VRAM or a ready Z-Image backend. Its last repository push was September 2024 when checked; compatibility needs a fresh trial. |
| Local reasoning and vision | [llama.cpp](https://github.com/ggml-org/llama.cpp), MIT; [Qwen3-VL-8B-Instruct](https://huggingface.co/Qwen/Qwen3-VL-8B-Instruct), Apache-2.0 weights | Quantized local language/vision interpretation with compatible models. Captions and inferred attributes remain predictions to check. |
| Intel device acceleration | [OpenVINO](https://github.com/openvinotoolkit/openvino), Apache-2.0 | Compatible inference on Intel CPU/GPU/NPU. A candidate for Galaxy Book hardware; the present GGUF image backend cannot simply be assigned to the NPU. |
| Image discovery | [Openverse API client](https://docs.openverse.org/packages/js/api_client/index.html), MIT | Keyword search with creator, source and licence metadata. Use attribution records and bounded downloads. Openverse does not independently guarantee the source's licensing claims. |
| General search and crawling | [SearXNG](https://github.com/searxng/searxng), AGPL-3.0; [Scrapy](https://github.com/scrapy/scrapy), BSD-3-Clause | A self-hosted search API and controlled crawlers. SearXNG can query configured engines; JSON must be enabled. A Google result is not permission to reuse an image. No crawler has access to all internet content. |
| Dataset quality | [FiftyOne](https://github.com/voxel51/fiftyone), Apache-2.0; [OpenCLIP](https://github.com/mlfoundations/open_clip), MIT | Curate examples, inspect embeddings and near duplicates, propose image/text matches. Embedding similarity is not proof of meaning; pretrained checkpoint licences are separate. |
| Personalized image/video models | [Diffusers](https://github.com/huggingface/diffusers) and [PEFT](https://github.com/huggingface/peft), Apache-2.0 | Compatible diffusion pipelines and LoRA adapter training. These require model weights, relevant examples, compute and held-out evaluation. A quantized inference pack is not automatically a training pack. SDXL model licences are more restrictive than Apache-2.0 and are separate from the library licence. |
| Memory | [Qdrant](https://github.com/qdrant/qdrant), Apache-2.0 | Retrieve similar prior observations and evidence. Retrieval storage alone does not supply deduction. |
| Behavioural experiments | [Gymnasium](https://github.com/Farama-Foundation/Gymnasium) and [Stable-Baselines3](https://github.com/DLR-RM/stable-baselines3), MIT | Observation/action/reward tasks and reinforcement learning. We must define tasks, success measures and recovery states. |
| Browser and Windows interfaces | [Playwright](https://github.com/microsoft/playwright), Apache-2.0; [pywinauto](https://github.com/pywinauto/pywinauto), BSD-3-Clause | Browser actions, viewport/touch tests and Windows accessibility automation. Emulation does not prove compatibility with every real device. |
| Entire UI experiments | [rrweb](https://github.com/rrweb-io/rrweb), MIT core; [Optuna](https://github.com/optuna/optuna), MIT | Optional masked interaction replay and parameter optimization, combined with generated frontend components and Playwright trials. Neither library independently redesigns a useful interface. |
| Tested code evolution | [OpenEvolve](https://github.com/algorithmicsuperintelligence/openevolve), Apache-2.0 | Generate candidate implementations, evaluate them and retain measured improvements. A local compatible model can replace a paid API; evaluator quality determines what improves. |
| Efficient kernels/languages | [egg](https://github.com/egraphs-good/egg), MIT; [MLIR](https://mlir.llvm.org/), Apache-2.0 with LLVM exceptions | Equivalent-expression search and domain-specific compilation. Rewrites must preserve correctness; fewer source characters do not imply less RAM or faster execution. |
| Recovery from failed code | [Wasmtime](https://github.com/bytecodealliance/wasmtime), Apache-2.0 with LLVM exceptions | Run suitable experimental WebAssembly kernels with restricted interfaces. Full native/Python experiments need OS/VM isolation. Keep a separate supervisor, known-good version and rollback. |

[EGG](https://github.com/facebookresearch/EGG), predominantly MIT with a BSD-3-Clause
LARC component, is an additional research reference for learned communication
between agents. It is separate from lowercase `egg` and does not invent an
efficient executable language automatically.

## What to build and measure

1. **Concept Studio:** editable concepts and captions, controlled online collection,
   provenance, exact/near-duplicate review and group-based training/validation/audit
   splits. Synthetic diagrams can establish exact left/right, above/below, on/off
   and inside/outside labels; they do not establish understanding of real scenes.
2. **Personalization:** compare base and candidate adapters on unchanged prompts,
   seeds and held-out examples; retain visual reviews as well as numerical losses.
   A slider may change a prompt or an adapter weight. Label that distinction. A
   lower denoising loss alone does not prove more realistic images.
3. **Interface reconstruction:** identify a difficult task from local aggregate
   interactions, generate components/layout candidates, test keyboard/touch and
   multiple widths, then compare successful completion, mistakes and recovery.
   Keep Stop and restore available independently. Mask/exclude private form input;
   do not treat quick approval decisions as a quality score.
4. **Internet participation:** an owner joins, chooses a budget and can leave.
   Prefer whole jobs where WAN latency makes model splitting slower. Test a small
   partitioned-model trial separately before integrating it into the image path.
5. **Community contributions:** share a portable recipe and reproducible evaluation
   first. Publish model weights to a suitable model registry, with a model card,
   provenance and licence; use GitHub pull requests for code and compact recipes.
   Private images, access keys and local paths are not automatic contributions.

The current recommended implementation order is dataset/concept tools and measured
experiments, an internet transport, then a separately benchmarked distributed
model pilot. Do not install every research framework into the small desktop host.
Optional workers keep incompatible Python/model dependencies isolated.

Open-source code can be used without an ORIGIN-0 subscription; licence obligations
still apply. GPU time, electricity, storage, relays and bandwidth have owners and
costs. No listed component guarantees universal device support, exponential
self-improvement, unlimited free compute or automatic foundation-model advances.

## Additional primary references

- [libp2p hole punching and relay architecture](https://libp2p.io/docs/hole-punching/)
- [Petals paper](https://arxiv.org/abs/2209.01188)
- [SearXNG JSON API](https://docs.searxng.org/dev/search_api.html)
- [Openverse terms and metadata limitations](https://docs.openverse.org/terms_of_service.html)
- [Diffusers SDXL training examples](https://github.com/huggingface/diffusers/blob/main/examples/text_to_image/README_sdxl.md)
- [FiftyOne image deduplication](https://docs.voxel51.com/recipes/image_deduplication.html)
- [Playwright device emulation](https://playwright.dev/docs/emulation)
- [LLVM licence and exceptions](https://github.com/llvm/llvm-project/blob/main/LICENSE.TXT)
- [Wasmtime licence and exceptions](https://github.com/bytecodealliance/wasmtime/blob/main/LICENSE)
