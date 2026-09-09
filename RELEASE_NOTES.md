ORIGIN-0 v1.5 is a working native image studio and an experimental learning workbench.

**Download `ORIGIN0_WINDOWS_v1.5.zip`, extract it, and double-click `ORIGIN0.exe`.**
The browser opens automatically. In Image studio, choose **Set up image engine**.
The first setup downloads a checksum-verified 6.52 GB model pack; wait for Ready,
then enter a prompt and choose Generate image. No Python, API key or subscription
is needed for this image path. The executable includes the native CPU runtime.
Keep at least 10 GB of free disk space. CPU generation can take several minutes.

This Windows preview targets modern x64 PCs with AVX2, including the processor
class in the user's 32 GB Galaxy Book. It is not an Android/iOS application.
Start with the CPU backend for the tested configuration. Optional Vulkan depends
on the installed driver and is not covered by the CPU acceptance result.

The release pipeline requires source tests, a Windows native startup probe, PE
import inspection, and an actual app-to-model-to-PNG test before publication.
`report.json`, `generation.log`, `imports.txt` and `image.png` contain that run's
evidence. The 256 × 256, one-step image checks execution, not final image quality.
A separate Linux CPU run generated and visually checked a 512 × 512, eight-step
image in 465.97 seconds. Speeds vary with hardware and settings.

Worker PCs can join a private group, contribute a chosen number of whole image
jobs, and leave at any time. This is coordinator-based volunteer scheduling;
it does not combine GPU memory or provide public torrent discovery.

Image generation is real. The inherited learning, interface experiments and
source-rebuild workbench are experimental. They do not automatically retrain the
image model or establish increasing realism or general intelligence. Video uses
a separate optional worker and is not part of this tested native image package.

Application code: MIT. Default model weights: Apache-2.0, with provenance and
notices included. Hardware, electricity and internet access remain necessary.
Contributions are welcome through https://github.com/HUGELU/HASL .
