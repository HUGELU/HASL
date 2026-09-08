ORIGIN-0 v1.5 — WINDOWS x64 — FIRST RUN

1. Extract ORIGIN0_STANDALONE_v1.5_WINDOWS.zip into its own folder.
2. Double-click ORIGIN0.exe. Keep the console window open.
3. Your browser opens the local workspace. If it does not, copy the COMPLETE
   address printed in the console, including everything after the # symbol.

The core application needs no Python, pip, Node, installer, or model download.
Optional local model training/generation needs a separate model environment.
The native executable includes its local server, interface, and source snapshot.
Use Settings > Save and stop ORIGIN-0 when finished. Launch it again to resume.
A second double-click reopens the existing instance for that data folder.

TRY THIS FIRST

Open Learning & upgrades to teach categories, inspect measured trials,
configure optional local model jobs, or test and rebuild a source candidate.

- Workspace: fill in the HUGE property-observation form and Save observation.
- Add an observation or drag a file onto the page.
- To generate new fields, supply a small JSON example such as:
  {"address":"Example street","asking_price":345000,"vacant":true}
- Interface lab: inspect four candidates, run responsive checks, then try the
  find-control exercise. Apply a layout you prefer after it passes the checks.
- Saved branches: save a named state. Branch here explores from that state;
  Restore returns to it; Merge knowledge adds missing structures without
  counting old evidence twice; Join as descendant adds its swarm memory.

WHAT RUNS LOCALLY

Continuous motif extraction, prefix coding, relations, tentative multi-hop
hypotheses, separate descendant memories, source-structure reflection, and
candidate interface generation. Six internal descendants start by default;
archived descendants can join up to a limit of twelve.

Pins and manual ordering express your preference. Candidate layouts do not
mutate the active layout. Automatic promotion requires passing browser layout
checks and at least five completed find-control trials for both the baseline
and candidate, with a 10% average-time improvement and no additional errors.
Promotion waits for interaction to stop and saves a restore point first.
These thresholds are practical heuristics, not statistical proof of superiority.

The lab checks its candidate templates at 280, 320, 390, 768, 1440 and 2560
CSS pixels. Real human task data must come from use; no trials are fabricated.
Scroll, touch, swipe, pointer activity and input-event counts can be aggregated
locally. Typed content is submitted separately. Turn measurement off in Settings
to clear current timing aggregates and prevent new measurements.

VOICE, IMAGES AND MODEL HELP

Voice & creation records audio locally when you press Record and allow your
browser to use the microphone. You can upload saved audio and images too.
Transcription, image generation, image analysis and model-assisted code proposals
can use a configured provider. Open Settings to connect OpenAI or a compatible
local server. Each request is started by you; credentials remain in memory and
are excluded from saved state. Provider model access and charges apply.
A local provider must implement the selected OpenAI-compatible endpoint.

Copy a ChatGPT handoff creates text you paste into ChatGPT yourself.
This is not an automatic connection to your existing ChatGPT conversation.

FILES, RESUME AND LARGE INPUTS

Normally the application creates these folders beside the executable:
  DROP_HERE          Files and folders for sampled observation.
  origin0_data       Working state, raw uploads, snapshots and logs.
If that location cannot be written, the console prints the alternative location.

Browser uploads are streamed to a SHA-256-addressed raw archive, with a 16 GiB
per-upload limit. DROP_HERE files stay where you put them and are sampled with
64-bit offsets. Their sampled hash is not a full-file identity check.
The working graph stays bounded; the managed raw archive defaults to 100 GiB.
Settings accepts up to 65536 GiB. For 2/4/8 TiB enter 2048/4096/8192.
This does not preallocate space or guarantee performance on a full archive.

Save and stop before making a full backup. Copy both origin0_data and DROP_HERE.
Export runnable branch packages the same executable and current working state;
it excludes raw objects, old snapshots and credentials. It is not a full backup
and is not a newly compiled version of the program.

UPGRADING

Keep your old application folder intact. This release changes the archive/UI
schema. If you choose to bring an older state.json into it, the original bytes
are preserved as state.before-laboratory.json and supported graph fields are
loaded. Earlier media/capability manifests and snapshot formats are retained in
your files but are not all represented by the new laboratory interface.

HONEST LIMITS OF THIS RELEASE

This is an experimental pattern-learning application. It does not establish
semantic comprehension, subjective senses, general intelligence, or increasing
intelligence just from more cycles or disk space. FLUX is inspiration only.
Learning & upgrades adds a small trained recognizer, tested numeric-kernel
rebuilding, and an optional local SDXL/LoRA/CogVideoX worker. See LOCAL_MODELS.md.
Local model/GPU execution needs installed dependencies and weights. Rebuilding
needs an installed Go compiler. Their setup is additional to this standalone app.
Arbitrary source engineering, direct live ChatGPT-session access, GPU graph
computation, video-model training and general intelligence are not implemented.
Engine profiles can be proposed; their heuristic scores do not auto-promote them
as proven improvements. Model-written source proposals remain proposals for review.

BUILD VALIDATION

See VALIDATION_V14.md and test-results.txt for the exact checks and their limits.
The Windows binary was cross-compiled here; it has not been executed on a
Windows desktop in this environment. Browser visual QA was blocked here.
This development executable is unsigned.

IF STARTUP FAILS

Keep the console open and read its message. The complete local URL appears there.
Runtime logs are in origin0_data\logs\runtime.log. If the latest state is corrupt,
the application preserves it and tries state.previous.json automatically.
Report the visible error and log excerpt; do not send API keys or private files.
