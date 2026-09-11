package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type EvolutionConfig struct {
	Python           string `json:"python"`
	ImageModel       string `json:"image_model"`
	VideoModel       string `json:"video_model"`
	GoCompiler       string `json:"go_compiler"`
	AutoBuild        bool   `json:"auto_build"`
	AutoTrain        bool   `json:"auto_train"`
	TrainingRuns     int    `json:"training_runs"`
	TrainSteps       int    `json:"train_steps"`
	ValidationPrompt string `json:"validation_prompt"`
	MaxMinutes       int    `json:"max_minutes"`
}
type EvolutionJob struct {
	ID        string         `json:"id"`
	Kind      string         `json:"kind"`
	Status    string         `json:"status"`
	Created   int64          `json:"created"`
	Finished  int64          `json:"finished"`
	Message   string         `json:"message"`
	Artifacts []AssetRecord  `json:"artifacts"`
	Result    map[string]any `json:"result,omitempty"`
}
type JobRequest struct {
	ConceptID  string `json:"concept_id"`
	Kind       string `json:"kind"`
	Prompt     string `json:"prompt"`
	Steps      int    `json:"steps"`
	Seed       int64  `json:"seed"`
	AdapterJob string `json:"adapter_job"`
	Target     string `json:"target"`
}

var rebuildFiles = []string{
	"media_studio_test.go", "scripts/verify_media_browser.py", "RECOVERY_MEDIA_STUDIO.md",
	"comfy_live.go",
	"common_vision.go",
	"common_vision_test.go",
	"web/common_vision.js",
	"web/common_vision.css",
	"COMMON_VISION.md",
	"scripts/prepare_upscaler.py",
	"native/upscale/builds.json",
	"PRODUCTION_STUDIO.md",
	"INTEGRATION_REVIEW.md",
	"third_party/Real-ESRGAN-LICENSE.txt",
	"native/upscale/main.cpp",
	"native/upscale/CMakeLists.txt",
	"scripts/build_upscaler.py",
	"scripts/verify_upscaler_native.py",
	"scripts/acceptance_app.py",
	"scripts/verify_production_api.py",
	"scripts/verify_production_browser.py",
	"production_test.go",
	"architecture.go",
	"development.go",
	"web/architecture.js",
	"web/development.js",
	"finishing.go",
	"finish_pixels.go",
	"upscale_manifest.json",
	"web/production.js",
	"hardware.go",
	"hardware_windows.go",
	"hardware_other.go",
	"privacy.go",
	"studio_tools.go",
	"studio_tools_test.go",
	"privacy_test.go",
	"web/privacy.js",
	"web/studio_tools.js",
	"concept_contributions.go",
	"concept_contributions_test.go",
	"internet_relay.go",
	"internet_relay_test.go",
	"INTERNET_RELAY.md",
	"concept_studio.go",
	"concept_studio_test.go",
	"web/concepts.js",
	"web/concepts.css",
	"MODEL_GUIDE.md",
	"TECHNOLOGY_REPORT.md",
	"CONCEPT_STUDIO.md",
	"go.mod",
	"main.go",
	"laboratory.go",
	"media.go",
	"learning.go",
	"adaptive_kernel.go",
	"evolution_jobs.go",
	"native_images.go",
	"downloads.go",
	"compute_pool.go",
	"native_images_test.go",
	"native_process_windows.go",
	"native_process_other.go",
	"model_catalog.json",
	"bundled/README.txt",
	"web/generator.js",
	"web/generator.css",
	"sparse_windows_test.go",
	"sparse_other_test.go",
	"main_test.go",
	"laboratory_test.go",
	"release_test.go",
	"learning_test.go",
	"jobs_test.go",
	"web/index.html",
	"web/app.js",
	"web/style.css",
	"web/evolution.js",
	"worker/local_models.py",
	"worker/requirements.txt",
	"worker/test_worker.py",
	"LICENSE",
	"README.md",
	"third_party/NOTICES.md",
	"third_party/Apache-2.0.txt",
	"third_party/stable-diffusion.cpp-LICENSE.txt",
	"README_FIRST.txt",
	"SOURCE_README.md",
	"LOCAL_MODELS.md",
	"build.py",
	"model_sources.go", "stability_matrix.go", "studio_setup.go", "ecosystem_test.go", "web/open-studio/src/ecosystem.js",
	"media_studio.go",
	"media_catalog.go",
	"comfy.go",
	"comfy_install.go",
	"studio_catalog.json",
	"comfy_runtimes.json",
	"web/media_studio.js",
	"MEDIA_STUDIO.md",
	"web/open-studio/README.md",
	"web/open-studio/origin.css",
	"web/open-studio/tailwind.css",
	"web/open-studio/index.html",
	"web/open-studio/tailwind.config.js",
	"web/open-studio/src/main.js",
	"web/open-studio/src/style.css",
	"web/open-studio/src/counter.js",
	"web/open-studio/src/origin_pages.js",
	"web/open-studio/src/components/CameraControls.js",
	"web/open-studio/src/components/AuthModal.js",
	"web/open-studio/src/components/UploadPicker.js",
	"web/open-studio/src/components/Sidebar.js",
	"web/open-studio/src/components/VideoStudio.js",
	"web/open-studio/src/components/LipSyncStudio.js",
	"web/open-studio/src/components/CinemaStudio.js",
	"web/open-studio/src/components/Header.js",
	"web/open-studio/src/components/ImageStudio.js",
	"web/open-studio/src/components/SettingsModal.js",
	"web/open-studio/src/lib/promptUtils.js",
	"web/open-studio/src/lib/muapi.js",
	"web/open-studio/src/lib/uploadHistory.js",
	"web/open-studio/src/lib/pendingJobs.js",
	"web/open-studio/src/lib/camera_assets.js",
	"web/open-studio/src/lib/models.js",
	"web/open-studio/src/styles/variables.css",
	"web/open-studio/src/styles/studio.css",
	"web/open-studio/src/styles/global.css",
}

func (e *Engine) initEvolutionJobs() {
	_ = os.MkdirAll(filepath.Join(e.dataDir, "jobs"), 0700)
	e.jobConfig = EvolutionConfig{MaxMinutes: 30}
	for i := range e.lab.Jobs {
		if e.lab.Jobs[i].Status == "running" || e.lab.Jobs[i].Status == "queued" {
			e.lab.Jobs[i].Status = "interrupted"
			e.lab.Jobs[i].Message = "Previous process ended before completion; no result was promoted."
		}
	}
}
func (e *Engine) stopEvolutionJobs() {
	e.jobsMu.Lock()
	e.jobsClosing = true
	if e.jobCancel != nil {
		e.jobCancel()
	}
	e.jobsMu.Unlock()
	e.jobWG.Wait()
}
func (e *Engine) setJob(id string, fn func(*EvolutionJob)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := range e.lab.Jobs {
		if e.lab.Jobs[i].ID == id {
			fn(&e.lab.Jobs[i])
			return
		}
	}
}
func (e *Engine) maybeAutoBuild() {
	e.jobsMu.Lock()
	yes := e.jobConfig.AutoBuild && e.jobConfig.GoCompiler != ""
	e.jobsMu.Unlock()
	if yes {
		if _, err := e.startEvolutionJob(JobRequest{Kind: "rebuild", Target: runtime.GOOS}); err != nil {
			e.addEvent("BUILD-DEFERRED", err.Error())
		}
	}
}
func (e *Engine) maybeAutoTrain() {
	e.jobsMu.Lock()
	cfg := e.jobConfig
	last := e.lastAutoTraining
	busy := e.jobCancel != nil
	e.jobsMu.Unlock()
	if !cfg.AutoTrain || cfg.TrainingRuns < 1 || busy {
		return
	}
	e.mu.RLock()
	revision := e.lab.Learning.Revision
	tr, va := map[string]bool{}, map[string]bool{}
	for _, x := range e.lab.Learning.Examples {
		if x.Kind == "image" && x.Caption != "" {
			if x.Split == "train" {
				tr[x.Group] = true
			}
			if x.Split == "validation" {
				va[x.Group] = true
			}
		}
	}
	e.mu.RUnlock()
	if revision <= last || len(tr) < 3 || len(va) < 1 {
		return
	}
	if _, err := e.startEvolutionJob(JobRequest{Kind: "train_lora", Prompt: cfg.ValidationPrompt, Steps: cfg.TrainSteps, Seed: 42}); err == nil {
		e.jobsMu.Lock()
		e.lastAutoTraining = revision
		e.jobConfig.TrainingRuns = maxInt(0, e.jobConfig.TrainingRuns-1)
		e.jobsMu.Unlock()
	}
}
func executablePath(s string) (string, error) {
	if strings.TrimSpace(s) == "" {
		return "", errors.New("configure the full path to the required executable first")
	}
	p, err := exec.LookPath(s)
	if err != nil {
		return "", err
	}
	p, err = filepath.Abs(p)
	if err != nil {
		return "", err
	}
	if strings.Contains(strings.ToLower(p), "microsoft/windowsapps") || strings.Contains(strings.ToLower(p), `microsoft\windowsapps`) {
		return "", errors.New("the Microsoft Store alias is not a Python runtime; choose an installed interpreter")
	}
	st, err := os.Stat(p)
	if err != nil || st.IsDir() {
		return "", errors.New("executable path is not a file")
	}
	return p, nil
}
func directoryPath(s string) (string, error) {
	p, err := filepath.Abs(s)
	if err != nil || strings.TrimSpace(s) == "" {
		return "", errors.New("choose an existing local model directory")
	}
	st, err := os.Stat(p)
	if err != nil || !st.IsDir() {
		return "", errors.New("model directory is missing")
	}
	return p, nil
}
func (e *Engine) startEvolutionJob(req JobRequest) (EvolutionJob, error) {
	var empty EvolutionJob
	if req.Kind != "diagnostics" && req.Kind != "image" && req.Kind != "video" && req.Kind != "train_lora" && req.Kind != "rebuild" {
		return empty, errors.New("unknown job kind")
	}
	if len(req.Prompt) > 4000 {
		return empty, errors.New("prompt limit is 4000 characters")
	}
	if (req.Kind == "image" || req.Kind == "video" || req.Kind == "train_lora") && strings.TrimSpace(req.Prompt) == "" {
		return empty, errors.New("enter a generation or validation prompt")
	}
	if req.Target == "" {
		req.Target = runtime.GOOS
	}
	if req.Target != "windows" && req.Target != "linux" {
		return empty, errors.New("build target must be windows or linux")
	}
	if req.Steps == 0 {
		req.Steps = 25
		if req.Kind == "train_lora" {
			req.Steps = 100
		}
	}
	if req.Steps < 1 || req.Steps > 2000 || (req.Kind != "train_lora" && req.Steps > 80) {
		return empty, errors.New("use 1–80 generation steps or 1–2000 training steps")
	}
	e.jobsMu.Lock()
	defer e.jobsMu.Unlock()
	if e.jobsClosing {
		return empty, errors.New("engine is stopping")
	}
	if e.jobCancel != nil {
		return empty, errors.New("a local compute job is already running; wait or cancel it")
	}
	if e.paused.Load() {
		return empty, errors.New("resume the engine before starting a compute job")
	}
	cfg := e.jobConfig
	var err error
	if req.Kind == "rebuild" {
		cfg.GoCompiler, err = executablePath(cfg.GoCompiler)
	} else {
		cfg.Python, err = executablePath(cfg.Python)
	}
	if err != nil {
		return empty, err
	}
	if req.Kind == "image" || req.Kind == "train_lora" {
		cfg.ImageModel, err = directoryPath(cfg.ImageModel)
	}
	if req.Kind == "video" {
		cfg.VideoModel, err = directoryPath(cfg.VideoModel)
	}
	if err != nil {
		return empty, err
	}
	if req.Kind == "rebuild" {
		e.mu.RLock()
		model := e.lab.Learning.Active
		e.mu.RUnlock()
		if model.ID == "" {
			return empty, errors.New("teach and validate a recognition model before rebuilding its numeric kernel")
		}
	}
	id := randomID()[:24]
	dir := filepath.Join(e.dataDir, "jobs", id)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return empty, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(maxInt(1, minInt(120, cfg.MaxMinutes)))*time.Minute)
	e.jobCancel = cancel
	e.jobID = id
	job := EvolutionJob{ID: id, Kind: req.Kind, Status: "running", Created: now(), Message: "Preparing local job. Engine exploration continues."}
	e.mu.Lock()
	e.lab.Jobs = append(e.lab.Jobs, job)
	if len(e.lab.Jobs) > 40 {
		e.lab.Jobs = e.lab.Jobs[len(e.lab.Jobs)-40:]
	}
	e.mu.Unlock()
	e.jobWG.Add(1)
	go func() {
		defer e.jobWG.Done()
		defer cancel()
		log, err := os.OpenFile(filepath.Join(dir, "run.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err == nil {
			err = e.acquireHeavy(ctx)
			if err == nil {
				if req.Kind == "rebuild" {
					err = e.runRebuild(ctx, cfg, req, id, dir, log)
				} else {
					err = e.runLocalModel(ctx, cfg, req, id, dir, log)
				}
				e.releaseHeavy()
			}
			_ = log.Close()
		}
		status, message := "completed", "Completed. Inspect the artifacts and measured result."
		if err != nil {
			status, message = "failed", err.Error()
		}
		if ctx.Err() != nil {
			status, message = "cancelled", "Job cancelled or its time budget ended. Partial files remain in the job folder; nothing was promoted."
		}
		e.setJob(id, func(j *EvolutionJob) { j.Status = status; j.Message = message; j.Finished = now() })
		e.jobsMu.Lock()
		e.jobCancel = nil
		e.jobID = ""
		e.jobsMu.Unlock()
		e.persist()
		e.addEvent("JOB", req.Kind+": "+status+" — "+message)
	}()
	return job, nil
}
func (e *Engine) runLocalModel(ctx context.Context, cfg EvolutionConfig, req JobRequest, id, dir string, log io.Writer) error {
	b, err := assets.ReadFile("worker/local_models.py")
	if err != nil {
		return err
	}
	script := filepath.Join(dir, "local_models.py")
	if err = atomicWrite(script, b); err != nil {
		return err
	}
	manifest := map[string]any{"kind": req.Kind, "prompt": req.Prompt, "steps": req.Steps, "seed": req.Seed, "output_dir": dir, "image_model": cfg.ImageModel, "video_model": cfg.VideoModel}
	if req.AdapterJob != "" {
		if !validID(req.AdapterJob) {
			return errors.New("invalid adapter job")
		}
		e.mu.RLock()
		approved := e.lab.ActiveAdapter == req.AdapterJob
		e.mu.RUnlock()
		if !approved {
			return errors.New("review and activate this adapter before using it")
		}
		ap := filepath.Join(e.dataDir, "jobs", req.AdapterJob, "adapter")
		if _, err = os.Stat(filepath.Join(ap, "pytorch_lora_weights.safetensors")); err != nil {
			return errors.New("adapter weights are unavailable")
		}
		manifest["adapter"] = ap
	}
	if req.Kind == "train_lora" {
		e.mu.RLock()
		ls := cloneLearning(e.lab.Learning)
		e.mu.RUnlock()
		train, val := []map[string]string{}, []map[string]string{}
		for _, x := range ls.Examples {
			if x.Kind != "image" || x.Caption == "" {
				continue
			}
			p, err := e.objectPath(x.Asset)
			if err != nil {
				continue
			}
			row := map[string]string{"path": p, "caption": x.Caption, "asset": x.Asset, "group": x.Group}
			if x.Split == "train" {
				train = append(train, row)
			} else if x.Split == "validation" {
				val = append(val, row)
			}
		}
		if req.ConceptID != "" {
			var revision int
			train, val, revision, err = e.conceptTrainingRows(req.ConceptID)
			if err != nil {
				return err
			}
			ls.Revision = revision
			manifest["concept_id"] = req.ConceptID
		}
		if len(train) < 3 || len(val) < 1 {
			return errors.New("LoRA training needs at least three captioned training images and one captioned validation image; teach them in Recognition")
		}
		manifest["train"] = train
		manifest["validation"] = val
		manifest["revision"] = ls.Revision
	}
	data, _ := json.Marshal(manifest)
	mp := filepath.Join(dir, "job.json")
	if err = atomicWrite(mp, data); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, cfg.Python, "-u", script, mp)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "HF_HUB_OFFLINE=1", "TRANSFORMERS_OFFLINE=1", "HF_HUB_DISABLE_TELEMETRY=1", "PYTHONUNBUFFERED=1")
	cmd.Stdout = log
	cmd.Stderr = log
	fmt.Fprintln(log, "Running the bundled local worker. Model loading is offline.")
	if err = cmd.Run(); err != nil {
		return fmt.Errorf("local worker failed: %w; open this job's log for the missing dependency or model error", err)
	}
	data, err = os.ReadFile(filepath.Join(dir, "result.json"))
	if err != nil {
		return err
	}
	var result map[string]any
	if err = json.Unmarshal(data, &result); err != nil {
		return err
	}
	// Only known output filenames from the bundled worker enter the object archive.
	for _, name := range []string{"generated.png", "generated.mp4", "baseline.png", "candidate.png", "adapter.zip", "result.json"} {
		p := filepath.Join(dir, name)
		f, err := os.Open(p)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		mime := "application/json"
		switch filepath.Ext(name) {
		case ".png":
			mime = "image/png"
		case ".mp4":
			mime = "video/mp4"
		case ".zip":
			mime = "application/zip"
		}
		a, se := e.storeObject(f, name, mime, "local job:"+req.Kind)
		_ = f.Close()
		if se != nil {
			return se
		}
		e.setJob(id, func(j *EvolutionJob) { j.Artifacts = append(j.Artifacts, a) })
	}
	e.setJob(id, func(j *EvolutionJob) { j.Result = result })
	return nil
}
func generatedKernel(metric string) (string, error) {
	if metric != "l1" && metric != "l2" {
		return "", errors.New("only validated numeric distance kernels can be generated")
	}
	term := "total += weight*delta*delta"
	if metric == "l1" {
		term = "if delta < 0 { delta = -delta }; total += weight*delta"
	}
	return fmt.Sprintf("package main\n// Generated from measured recognition evidence. Pure numeric code.\nconst compiledRecognitionMetric = %q\nfunc compiledFeatureDistance(a,b []float64) float64 {\ntotal:=0.0\nfor i:=range a {weight:=1.0;if a[63]>0.5 && b[63]>0.5 && i<32 {weight=0.02};delta:=a[i]-b[i];%s}\nreturn total\n}\n", metric, term), nil
}
func embeddedSource() (map[string]string, error) {
	b, err := assets.ReadFile("source_bundle.json")
	if err != nil {
		return nil, err
	}
	var src map[string]string
	if err = json.Unmarshal(b, &src); err != nil {
		return nil, err
	}
	if len(src) != len(rebuildFiles) {
		return nil, errors.New("source bundle does not match the trusted build manifest")
	}
	for _, name := range rebuildFiles {
		if src[name] == "" {
			return nil, fmt.Errorf("source bundle missing %s", name)
		}
	}
	return src, nil
}
func writeSourceTree(src map[string]string, dir string) error {
	names := append([]string{}, rebuildFiles...)
	sort.Strings(names)
	var snapshot strings.Builder
	snapshot.WriteString("ORIGIN-0 v1.7.0: exact source used for this build.\n")
	for _, name := range names {
		if err := atomicWrite(filepath.Join(dir, filepath.FromSlash(name)), []byte(src[name])); err != nil {
			return err
		}
		snapshot.WriteString("\n===== " + name + " =====\n" + src[name])
	}
	b, _ := json.Marshal(src)
	if err := atomicWrite(filepath.Join(dir, "source_bundle.json"), b); err != nil {
		return err
	}
	return atomicWrite(filepath.Join(dir, "source_snapshot.txt"), []byte(snapshot.String()))
}
func buildEnvironment(target string) []string {
	env := []string{}
	for _, line := range os.Environ() {
		k := strings.SplitN(line, "=", 2)[0]
		if k == "GOOS" || k == "GOARCH" || k == "CGO_ENABLED" || k == "GOWORK" || k == "GOFLAGS" || k == "GOENV" || k == "GOTOOLCHAIN" || k == "GOPROXY" || k == "GOSUMDB" {
			continue
		}
		env = append(env, line)
	}
	env = append(env, "GOENV=off", "GOWORK=off", "GOFLAGS=", "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "CGO_ENABLED=0", "GOARCH=amd64")
	if target != "" {
		env = append(env, "GOOS="+target)
	}
	return env
}
func (e *Engine) runRebuild(ctx context.Context, cfg EvolutionConfig, req JobRequest, id, dir string, log io.Writer) error {
	src, err := embeddedSource()
	if err != nil {
		return err
	}
	e.mu.RLock()
	model := e.lab.Learning.Active
	e.mu.RUnlock()
	kernel, err := generatedKernel(model.Metric)
	if err != nil {
		return err
	}
	original := src["adaptive_kernel.go"]
	src["adaptive_kernel.go"] = kernel
	sourceDir := filepath.Join(dir, "source")
	if err = writeSourceTree(src, sourceDir); err != nil {
		return err
	}
	commands := [][]string{
		{"version"},
		{"test", "-buildvcs=false", "-count=1", "-timeout=180s", "./..."},
		{"vet", "-buildvcs=false", "./..."},
	}
	for _, args := range commands {
		fmt.Fprintln(log, "GO", strings.Join(args, " "))
		cmd := exec.CommandContext(ctx, cfg.GoCompiler, args...)
		cmd.Dir = sourceDir
		cmd.Env = buildEnvironment(runtime.GOOS)
		cmd.Stdout = log
		cmd.Stderr = log
		if err = cmd.Run(); err != nil {
			return fmt.Errorf("candidate failed its build gate: %w; incumbent executable is unchanged", err)
		}
	}
	name := "ORIGIN0-next.exe"
	if req.Target == "linux" {
		name = "origin0-next"
	}
	output := filepath.Join(dir, name)
	cmd := exec.CommandContext(ctx, cfg.GoCompiler, "build", "-buildvcs=false", "-trimpath", "-ldflags=-s -w", "-o", output, ".")
	cmd.Dir = sourceDir
	cmd.Env = buildEnvironment(req.Target)
	cmd.Stdout = log
	cmd.Stderr = log
	fmt.Fprintln(log, "Compiling target:", req.Target, "amd64")
	if err = cmd.Run(); err != nil {
		return err
	}
	b, err := os.ReadFile(output)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(b)
	sourceHash := sha256.Sum256([]byte(kernel))
	manifest := map[string]any{"target": req.Target + "/amd64", "binary_sha256": hex.EncodeToString(hash[:]), "kernel_sha256": hex.EncodeToString(sourceHash[:]), "recognition_model": model.ID, "evidence_revision": model.Revision, "metric": model.Metric, "tests": "passed", "vet": "passed", "scope": "Generated pure numeric recognition kernel. Full host recompiled. Existing executable retained.", "old_kernel": original, "new_kernel": kernel, "runtime_tested_on_target": false}
	mb, _ := json.MarshalIndent(manifest, "", "  ")
	if err = atomicWrite(filepath.Join(dir, "build-manifest.json"), mb); err != nil {
		return err
	}
	var archive bytes.Buffer
	z := zip.NewWriter(&archive)
	for _, n := range rebuildFiles {
		w, err := z.Create("source/" + n)
		if err != nil {
			return err
		}
		if _, err = io.WriteString(w, src[n]); err != nil {
			return err
		}
	}
	for n, data := range map[string][]byte{name: b, "BUILD_MANIFEST.json": mb, "READ_FIRST.txt": []byte("Extract into a new folder and run the new program. The current program and its data have not been overwritten. This archive contains a measured numeric-kernel change, not arbitrary model-written host code. Copy origin0_data from a stopped instance if you want its existing learning state.\n")} {
		w, err := z.Create(n)
		if err != nil {
			return err
		}
		if _, err = w.Write(data); err != nil {
			return err
		}
	}
	if err = z.Close(); err != nil {
		return err
	}
	a, err := e.storeObject(bytes.NewReader(archive.Bytes()), "ORIGIN0_REBUILT_"+id+".zip", "application/zip", "verified local rebuild")
	if err != nil {
		return err
	}
	e.setJob(id, func(j *EvolutionJob) { j.Artifacts = append(j.Artifacts, a); j.Result = manifest })
	return nil
}
func logTail(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return ""
	}
	if st.Size() > 14000 {
		_, _ = f.Seek(-14000, io.SeekEnd)
	}
	b, _ := io.ReadAll(io.LimitReader(f, 14000))
	return string(b)
}
func (e *Engine) evolutionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/evolution/state", func(w http.ResponseWriter, r *http.Request) {
		e.jobsMu.Lock()
		cfg := e.jobConfig
		active := e.jobID
		e.jobsMu.Unlock()
		e.mu.RLock()
		b, _ := json.Marshal(e.lab.Jobs)
		adapter := e.lab.ActiveAdapter
		e.mu.RUnlock()
		var jobs []EvolutionJob
		_ = json.Unmarshal(b, &jobs)
		jsonReply(w, map[string]any{"config": cfg, "jobs": jobs, "active": active, "active_adapter": adapter, "compiled_metric": compiledRecognitionMetric})
	})
	mux.HandleFunc("/api/evolution/config", func(w http.ResponseWriter, r *http.Request) {
		var cfg EvolutionConfig
		if err := decode(r, &cfg); err != nil {
			apiError(w, err)
			return
		}
		cfg.MaxMinutes = maxInt(1, minInt(120, cfg.MaxMinutes))
		cfg.TrainSteps = maxInt(1, minInt(2000, cfg.TrainSteps))
		cfg.TrainingRuns = maxInt(0, minInt(10, cfg.TrainingRuns))
		if len(cfg.ValidationPrompt) > 4000 {
			apiError(w, errors.New("validation prompt limit is 4000 characters"))
			return
		}
		if cfg.AutoTrain && (cfg.Python == "" || cfg.ImageModel == "" || strings.TrimSpace(cfg.ValidationPrompt) == "" || cfg.TrainingRuns < 1) {
			apiError(w, errors.New("automatic training requires Python, a local SDXL model, a validation prompt, and a session run allowance"))
			return
		}
		for _, p := range []string{cfg.Python, cfg.GoCompiler} {
			if p != "" {
				if _, err := executablePath(p); err != nil {
					apiError(w, err)
					return
				}
			}
		}
		if cfg.AutoBuild && cfg.GoCompiler == "" {
			apiError(w, errors.New("select an installed Go compiler for automatic builds"))
			return
		}
		e.jobsMu.Lock()
		e.jobConfig = cfg
		e.jobsMu.Unlock()
		jsonReply(w, cfg)
	})
	mux.HandleFunc("/api/evolution/start", func(w http.ResponseWriter, r *http.Request) {
		var x JobRequest
		if err := decode(r, &x); err != nil {
			apiError(w, err)
			return
		}
		job, err := e.startEvolutionJob(x)
		if err != nil {
			apiError(w, err)
			return
		}
		jsonReply(w, job)
	})
	mux.HandleFunc("/api/evolution/cancel", func(w http.ResponseWriter, r *http.Request) {
		e.jobsMu.Lock()
		if e.jobCancel != nil {
			e.jobCancel()
		}
		e.jobsMu.Unlock()
		w.WriteHeader(204)
	})
	mux.HandleFunc("/api/evolution/log", func(w http.ResponseWriter, r *http.Request) {
		var x struct{ ID string }
		if err := decode(r, &x); err != nil {
			apiError(w, err)
			return
		}
		if !validID(x.ID) {
			apiError(w, errors.New("invalid job ID"))
			return
		}
		jsonReply(w, map[string]string{"log": logTail(filepath.Join(e.dataDir, "jobs", x.ID, "run.log"))})
	})
	mux.HandleFunc("/api/evolution/adapter", func(w http.ResponseWriter, r *http.Request) {
		var x struct {
			ID        string
			Preferred bool
			Reason    string
		}
		if err := decode(r, &x); err != nil {
			apiError(w, err)
			return
		}
		if len(strings.TrimSpace(x.Reason)) < 5 || len(x.Reason) > 1000 {
			apiError(w, errors.New("record what improved or failed in the before/after images"))
			return
		}
		e.mu.Lock()
		found := false
		for i, j := range e.lab.Jobs {
			if j.ID == x.ID && j.Kind == "train_lora" && j.Status == "completed" {
				found = true
				if e.lab.Jobs[i].Result == nil {
					e.lab.Jobs[i].Result = map[string]any{}
				}
				e.lab.Jobs[i].Result["human_review"] = x.Reason
				e.lab.Jobs[i].Result["preferred"] = x.Preferred
				if x.Preferred {
					e.lab.ActiveAdapter = x.ID
				} else if e.lab.ActiveAdapter == x.ID {
					e.lab.ActiveAdapter = ""
				}
			}
		}
		e.mu.Unlock()
		if !found {
			apiError(w, errors.New("completed training job not found"))
			return
		}
		e.persist()
		w.WriteHeader(204)
	})
}
