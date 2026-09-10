ORIGIN-0 v1.6 — LOCAL IMAGE STUDIO

WINDOWS FIRST RUN
1. Put ORIGIN0.exe in a normal folder and double-click it.
2. Keep the console open. It prints the local browser address and opens it.
3. In Image studio, click Set up image engine. The first run downloads
   6.52 GB of pinned model files. Progress, verification and errors stay visible.
4. Wait for Ready, enter a prompt and click Generate image.
5. Download the PNG when the queue says Completed.

No Python, pip, API key or paid model service is needed for Image studio.
The release executable includes the native CPU runtime. Keep at least 10 GB of
free disk space for the model pack and setup. 16 GB RAM is a practical starting
point; 32 GB gives more room. CPU generation takes minutes for a 512-pixel image.
The Windows CPU package targets modern x64 processors with AVX2. It is not an
Android, iOS or ARM Windows executable. GPU speed depends on the device/driver.

IF SOMETHING FAILS
Keep the console open and use Download diagnostics in Image studio. Setup and
generation jobs show the actual error. Try CPU mode if the Vulkan driver or
runtime fails. Stop and retry an interrupted model download to resume it.
Never switch off your antivirus to run ORIGIN-0. The executable is unsigned;
if Windows blocks it, use the documented source/build and report that failure.

YOUR FILES
origin0_data holds state, generated images, model cache and logs. DROP_HERE
accepts your supplied files. Copy the complete data folder for a full backup.
ORIGIN0_HOME may point to another writable disk before starting the executable.
Model-cache files are separate from the research workspace raw archive quota.
The saved-branch export does not include the multi-GB model cache.

WORKER PCS
Each PC first prepares its own image engine. The coordinator starts a Worker PCs
group and creates a private invitation per worker. A worker pastes the invitation,
sets its job allowance and joins. On the coordinator, select the shared-job
checkbox before generating. Only explicitly shared prompts go to workers.
Direct groups need a reachable LAN/private-network address. For internet groups,
use an operator-deployed HTTPS relay (INTERNET_RELAY.md). Both laptops connect
outbound; the relay carries encrypted job envelopes. No public relay is supplied.
This does not pool VRAM or implement torrent transfers/public peer discovery.

SOURCE AND UPDATES
https://github.com/HUGELU/HASL
Application: MIT. Default model pack: Apache-2.0. No ORIGIN-0 usage subscription.
The legacy research, interface and classifier experiments are still available.
They do not establish automatic improvement of the image model's weights.

NEW CONTROLS
Concept Studio collects and reviews image examples and evaluates small classifiers.
Image studio now offers one-image revision, rough previews, batches, ratings and
saved settings. Hardware defaults are suggestions, not certified performance.
Settings can protect the interface with a PIN. Save its recovery-code download.
The PIN does not encrypt files or hide the process from Windows.
