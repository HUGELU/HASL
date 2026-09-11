"""Build standalone ORIGIN-0. Requires Go only for compilation, Python for this helper."""
import argparse
import os
import json
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parent
FILES = ['model_sources.go', 'stability_matrix.go', 'studio_setup.go', 'ecosystem_test.go', 'web/open-studio/src/ecosystem.js', 'media_studio_test.go', 'scripts/verify_media_browser.py', 'RECOVERY_MEDIA_STUDIO.md', 'comfy_live.go', 'common_vision.go', 'common_vision_test.go', 'web/common_vision.js', 'web/common_vision.css', 'COMMON_VISION.md', 'scripts/prepare_upscaler.py', 'native/upscale/builds.json', 'PRODUCTION_STUDIO.md', 'INTEGRATION_REVIEW.md', 'third_party/Real-ESRGAN-LICENSE.txt', 'native/upscale/main.cpp', 'native/upscale/CMakeLists.txt', 'scripts/build_upscaler.py', 'scripts/verify_upscaler_native.py', 'scripts/acceptance_app.py', 'scripts/verify_production_api.py', 'scripts/verify_production_browser.py', 'production_test.go', 'architecture.go', 'development.go', 'web/architecture.js', 'web/development.js', 'finishing.go', 'finish_pixels.go', 'upscale_manifest.json', 'web/production.js', 'hardware.go', 'hardware_windows.go', 'hardware_other.go', 'privacy.go', 'studio_tools.go', 'studio_tools_test.go', 'privacy_test.go', 'web/privacy.js', 'web/studio_tools.js', 'concept_contributions.go', 'concept_contributions_test.go', 'internet_relay.go', 'internet_relay_test.go', 'INTERNET_RELAY.md', 'concept_studio.go', 'concept_studio_test.go', 'web/concepts.js', 'web/concepts.css', 'MODEL_GUIDE.md', 'TECHNOLOGY_REPORT.md', 'CONCEPT_STUDIO.md', 'go.mod', 'main.go', 'laboratory.go', 'media.go', 'learning.go', 'adaptive_kernel.go', 'evolution_jobs.go', 'native_images.go', 'downloads.go', 'compute_pool.go', 'native_images_test.go', 'native_process_windows.go', 'native_process_other.go', 'model_catalog.json', 'bundled/README.txt', 'web/generator.js', 'web/generator.css', 'sparse_windows_test.go', 'sparse_other_test.go', 'main_test.go', 'laboratory_test.go', 'release_test.go', 'learning_test.go', 'jobs_test.go', 'web/index.html', 'web/app.js', 'web/style.css', 'web/evolution.js', 'worker/local_models.py', 'worker/requirements.txt', 'worker/test_worker.py', 'LICENSE', 'README.md', 'third_party/NOTICES.md', 'third_party/Apache-2.0.txt', 'third_party/stable-diffusion.cpp-LICENSE.txt', 'README_FIRST.txt', 'SOURCE_README.md', 'LOCAL_MODELS.md', 'build.py', 'media_studio.go', 'media_catalog.go', 'comfy.go', 'comfy_install.go', 'studio_catalog.json', 'comfy_runtimes.json', 'web/media_studio.js', 'MEDIA_STUDIO.md', 'web/open-studio/README.md', 'web/open-studio/origin.css', 'web/open-studio/tailwind.css', 'web/open-studio/index.html', 'web/open-studio/tailwind.config.js', 'web/open-studio/src/main.js', 'web/open-studio/src/style.css', 'web/open-studio/src/counter.js', 'web/open-studio/src/origin_pages.js', 'web/open-studio/src/components/CameraControls.js', 'web/open-studio/src/components/AuthModal.js', 'web/open-studio/src/components/UploadPicker.js', 'web/open-studio/src/components/Sidebar.js', 'web/open-studio/src/components/VideoStudio.js', 'web/open-studio/src/components/LipSyncStudio.js', 'web/open-studio/src/components/CinemaStudio.js', 'web/open-studio/src/components/Header.js', 'web/open-studio/src/components/ImageStudio.js', 'web/open-studio/src/components/SettingsModal.js', 'web/open-studio/src/lib/promptUtils.js', 'web/open-studio/src/lib/muapi.js', 'web/open-studio/src/lib/uploadHistory.js', 'web/open-studio/src/lib/pendingJobs.js', 'web/open-studio/src/lib/camera_assets.js', 'web/open-studio/src/lib/models.js', 'web/open-studio/src/styles/variables.css', 'web/open-studio/src/styles/studio.css', 'web/open-studio/src/styles/global.css']

def main():
    p = argparse.ArgumentParser()
    p.add_argument('--go', default='go')
    p.add_argument('--target', choices=['windows', 'linux', 'darwin'], default='windows')
    p.add_argument('--arch', choices=['amd64', 'arm64'], default='amd64')
    args = p.parse_args()
    bundle = {name: (ROOT / name).read_text(encoding='utf-8') for name in FILES}
    (ROOT / 'source_bundle.json').write_text(json.dumps(bundle, sort_keys=True), encoding='utf-8')
    text = 'ORIGIN-0 v1.9.0: exact source snapshot embedded at build time.\n'
    for name in FILES:
        text += '\n===== ' + name + ' =====\n' + (ROOT / name).read_text(encoding='utf-8')
    (ROOT / 'source_snapshot.txt').write_text(text, encoding='utf-8')
    env = os.environ.copy()
    env.update(CGO_ENABLED='0', GOOS=args.target, GOARCH=args.arch)
    out = ROOT / 'release' / ('ORIGIN0.exe' if args.target == 'windows' and args.arch == 'amd64' else 'origin0-' + args.target + '-' + args.arch + ('.exe' if args.target == 'windows' else ''))
    out.parent.mkdir(exist_ok=True)
    subprocess.run([args.go, 'build', '-buildvcs=false', '-trimpath', '-ldflags=-s -w', '-o', str(out), '.'], cwd=ROOT, env=env, check=True)
    print(out)

if __name__ == '__main__':
    main()
