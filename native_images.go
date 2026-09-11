package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ImageRequest struct {
	Finish    *FinishOptions `json:"finish,omitempty"`
	Prompt    string         `json:"prompt"`
	Width     int            `json:"width"`
	Height    int            `json:"height"`
	Steps     int            `json:"steps"`
	Seed      int64          `json:"seed"`
	Shared    bool           `json:"shared"`
	InitAsset string         `json:"init_asset,omitempty"`
	Strength  float64        `json:"strength,omitempty"`
	Preview   bool           `json:"preview,omitempty"`
}
type ImageJob struct {
	ID         string         `json:"id"`
	Request    ImageRequest   `json:"request"`
	Status     string         `json:"status"`
	Created    int64          `json:"created"`
	Started    int64          `json:"started"`
	Finished   int64          `json:"finished"`
	Message    string         `json:"message"`
	Backend    string         `json:"backend"`
	Pack       string         `json:"pack"`
	Log        string         `json:"log"`
	Asset      *AssetRecord   `json:"asset,omitempty"`
	Worker     string         `json:"worker,omitempty"`
	Attempts   int            `json:"attempts"`
	Lease      string         `json:"-"`
	LeaseUntil int64          `json:"-"`
	Ratings    *ImageRatings  `json:"ratings,omitempty"`
	Progress   map[string]any `json:"progress,omitempty"`
}
type ImageConfig struct {
	Backend    string `json:"backend"`
	Threads    int    `json:"threads"`
	MaxMinutes int    `json:"max_minutes"`
}
type SetupStatus struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Done    int64  `json:"done"`
	Total   int64  `json:"total"`
	File    int    `json:"file"`
	Files   int    `json:"files"`
	Devices string `json:"devices"`
}
type NativeImages struct {
	e             *Engine
	mu            sync.Mutex
	persistMu     sync.Mutex
	wg            sync.WaitGroup
	closed        bool
	config        ImageConfig
	catalog       ModelCatalog
	setup         SetupStatus
	jobs          []*ImageJob
	ready         bool
	cli           string
	cancel        context.CancelFunc
	active        string
	pool          *ComputePool
	client        *http.Client
	actualBackend string
}

func newNativeImages(e *Engine) *NativeImages {
	s := &NativeImages{e: e, config: recommendImage(HardwareProfile{Threads: runtime.NumCPU(), Memory: physicalMemory()}, "balanced", false).Config, setup: SetupStatus{Status: "needed", Message: "Set up the local image engine to begin."}, client: &http.Client{Transport: &http.Transport{Proxy: http.ProxyFromEnvironment, ResponseHeaderTimeout: 45 * time.Second, IdleConnTimeout: 60 * time.Second}}}
	b, _ := assets.ReadFile("model_catalog.json")
	_ = json.Unmarshal(b, &s.catalog)
	var saved struct {
		Config ImageConfig `json:"config"`
		Jobs   []*ImageJob `json:"jobs"`
	}
	if b, err := os.ReadFile(filepath.Join(e.dataDir, "generator.json")); err == nil && json.Unmarshal(b, &saved) == nil {
		s.config = saved.Config
		s.jobs = saved.Jobs
		if s.config.Threads < 1 || s.config.Threads > runtime.NumCPU() {
			s.config.Threads = minInt(8, maxInt(1, runtime.NumCPU()-2))
		}
		if s.config.MaxMinutes < 1 || s.config.MaxMinutes > 120 {
			s.config.MaxMinutes = 60
		}
		if s.config.Backend != "auto" && s.config.Backend != "cpu" && s.config.Backend != "vulkan" && s.config.Backend != "metal" {
			s.config.Backend = "auto"
		}
		for _, j := range s.jobs {
			if j.Status == "running" || j.Status == "queued" {
				j.Status = "interrupted"
				j.Message = "The previous process ended. Submit the prompt again to retry."
			}
		}
	}
	s.pool = newComputePool(s)
	// No executable is launched during construction or before explicit setup.
	return s
}
func (s *NativeImages) root() string { return filepath.Join(s.e.dataDir, "generator") }
func (s *NativeImages) modelPath(role string) string {
	for _, f := range s.catalog.Files {
		if f.Role == role {
			return filepath.Join(s.root(), "models", f.Name)
		}
	}
	return ""
}
func (s *NativeImages) save() {
	s.persistMu.Lock()
	defer s.persistMu.Unlock()
	s.mu.Lock()
	b, err := json.Marshal(struct {
		Config ImageConfig `json:"config"`
		Jobs   []*ImageJob `json:"jobs"`
	}{s.config, s.jobs})
	s.mu.Unlock()
	if err == nil {
		if err = atomicWrite(filepath.Join(s.e.dataDir, "generator.json"), b); err != nil {
			s.e.addEvent("GENERATOR-SAVE", err.Error())
		}
	}
}
func (s *NativeImages) close() {
	s.mu.Lock()
	s.closed = true
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()
	s.pool.close()
	s.wg.Wait()
	s.save()
}
func (s *NativeImages) status() any {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Copy before releasing the lock; the HTTP encoder runs later.
	jobs := make([]*ImageJob, 0, len(s.jobs))
	for i, j := range s.jobs {
		x := *j
		x.Progress = imageProgress(x)
		if i < len(s.jobs)-32 {
			x.Log = ""
		}
		jobs = append(jobs, &x)
	}
	b, _ := json.Marshal(struct {
		Config   ImageConfig  `json:"config"`
		Setup    SetupStatus  `json:"setup"`
		Ready    bool         `json:"ready"`
		Jobs     []*ImageJob  `json:"jobs"`
		Catalog  ModelCatalog `json:"catalog"`
		Platform string       `json:"platform"`
		Folder   string       `json:"folder"`
		Busy     bool         `json:"busy"`
	}{s.config, s.setup, s.ready, jobs, s.catalog, runtime.GOOS + "/" + runtime.GOARCH, s.root(), s.cancel != nil})
	var v any
	_ = json.Unmarshal(b, &v)
	return v
}
func (s *NativeImages) runtimeKey(backend string) string {
	return runtime.GOOS + "-" + runtime.GOARCH + "-" + backend
}
func (s *NativeImages) setupProgress(message string, done, total int64) {
	s.mu.Lock()
	s.setup.Message = message
	s.setup.Done = done
	s.setup.Total = total
	s.mu.Unlock()
}
func (s *NativeImages) startSetup(cfg ImageConfig) error {
	if cfg.Backend != "auto" && cfg.Backend != "cpu" && cfg.Backend != "vulkan" && cfg.Backend != "metal" {
		return errors.New("choose Auto, CPU, Vulkan or Metal")
	}
	if cfg.Threads < 1 || cfg.Threads > runtime.NumCPU() || cfg.MaxMinutes < 1 || cfg.MaxMinutes > 120 {
		return errors.New("invalid thread or time budget")
	}
	s.mu.Lock()
	if s.closed || s.cancel != nil {
		s.mu.Unlock()
		return errors.New("another local task is running; finish or cancel it first")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
	s.cancel = cancel
	s.active = "setup"
	s.ready = false
	s.config = cfg
	s.setup = SetupStatus{Status: "running", Message: "Preparing pinned native runtime", Files: len(s.catalog.Files) + 1}
	s.wg.Add(1)
	s.mu.Unlock()
	s.save()
	go func() {
		defer s.wg.Done()
		defer cancel()
		err := s.setupEngine(ctx, cfg)
		s.mu.Lock()
		s.cancel = nil
		s.active = ""
		if err != nil {
			s.setup.Status = "error"
			s.setup.Message = err.Error()
			if ctx.Err() != nil {
				s.setup.Status = "cancelled"
				s.setup.Message = "Setup stopped. Retry resumes incomplete model downloads."
			}
		} else {
			s.ready = true
			s.setup.Status = "ready"
			s.setup.Message = "Engine and models verified. Ready to generate locally."
		}
		s.mu.Unlock()
		s.save()
		s.kick()
	}()
	return nil
}
func (s *NativeImages) setupRuntime(ctx context.Context, backend string) (string, string, error) {
	spec, ok := s.catalog.Runtimes[s.runtimeKey(backend)]
	if !ok {
		return "", "", fmt.Errorf("no bundled runtime for %s/%s with %s; use a supported worker PC", runtime.GOOS, runtime.GOARCH, backend)
	}
	folder := filepath.Join(s.root(), "runtimes", spec.SHA256[:12])
	archive := filepath.Join(s.root(), "downloads", spec.Name)
	if _, err := os.Stat(archive); os.IsNotExist(err) {
		if b, err := assets.ReadFile("bundled/" + spec.Name); err == nil {
			if err = atomicWrite(archive, b); err != nil {
				return "", "", err
			}
		}
	}
	if err := downloadPinned(ctx, s.client, spec, archive, s.setupProgress); err != nil {
		return "", "", err
	}
	s.setupProgress("Extracting "+backend+" runtime", 0, 0)
	// Re-extract a verified runtime at each setup, repairing missing native libraries.
	if err := extractRuntime(archive, folder); err != nil {
		return "", "", err
	}
	var cli string
	want := "sd-cli"
	if runtime.GOOS == "windows" {
		want += ".exe"
	}
	_ = filepath.WalkDir(folder, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == want && cli == "" {
			cli = path
		}
		return nil
	})
	if cli == "" {
		return "", "", errors.New("the native runtime archive did not contain sd-cli")
	}
	probe, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := nativeCommand(probe, cli, "--list-devices")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", string(out), fmt.Errorf("%s runtime could not start: %w. %s", backend, err, tail(string(out), 1800))
	}
	return cli, string(out), nil
}
func (s *NativeImages) setupEngine(ctx context.Context, cfg ImageConfig) error {
	backend := cfg.Backend
	if backend == "auto" {
		backend = "cpu"
		if runtime.GOOS == "windows" || runtime.GOOS == "linux" {
			backend = "vulkan"
		}
		if runtime.GOOS == "darwin" {
			backend = "metal"
		}
	}
	cli, devices, err := s.setupRuntime(ctx, backend)
	if cfg.Backend == "auto" && (err != nil || backend == "vulkan" && !strings.Contains(strings.ToLower(devices), "vulkan")) && ctx.Err() == nil {
		s.setupProgress("GPU runtime unavailable; preparing CPU fallback", 0, 0)
		cli, devices, err = s.setupRuntime(ctx, "cpu")
		backend = "cpu"
	}
	if err != nil {
		return err
	}
	for i, f := range s.catalog.Files {
		s.mu.Lock()
		s.setup.File = i + 2
		s.mu.Unlock()
		if err := downloadPinned(ctx, s.client, f, filepath.Join(s.root(), "models", f.Name), s.setupProgress); err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
	}
	s.mu.Lock()
	s.cli = cli
	s.actualBackend = backend
	s.setup.Devices = strings.TrimSpace(devices)
	s.setup.File = s.setup.Files
	s.mu.Unlock()
	receipt, _ := json.Marshal(map[string]any{"catalog": s.catalog.ID, "runtime": backend, "cli": cli, "verified": now()})
	return atomicWrite(filepath.Join(s.root(), "verified.json"), receipt)
}
func nativeCommand(ctx context.Context, cli string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, cli, args...)
	prepareNativeCommand(cmd)
	cmd.Dir = filepath.Dir(cli)
	cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+filepath.Dir(cli))
	cmd.WaitDelay = 5 * time.Second
	return cmd
}
func tail(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}
func validateImageRequest(r ImageRequest) error {
	if strings.TrimSpace(r.Prompt) == "" || len(r.Prompt) > 4000 {
		return errors.New("enter a prompt up to 4000 bytes")
	}
	if r.Width < 256 || r.Width > 1024 || r.Height < 256 || r.Height > 1024 || r.Width%64 != 0 || r.Height%64 != 0 {
		return errors.New("image sides must be 256–1024 pixels, in multiples of 64")
	}
	if r.Steps < 1 || r.Steps > 20 || r.Seed < 0 || r.Seed > 2147483647 {
		return errors.New("use 1–20 steps and a seed from 0 to 2147483647")
	}
	if r.InitAsset != "" {
		if !validAssetID(r.InitAsset) || r.Strength < 0.15 || r.Strength > 1 || int(float64(r.Steps)*r.Strength) < 1 {
			return errors.New("use a stored PNG/JPEG, strength 0.15–1.0 and enough steps for at least one denoising step")
		}
		if r.Shared {
			return errors.New("image-to-image references currently stay local; turn off worker sharing for this job")
		}
	}
	return nil
}
func (s *NativeImages) submit(r ImageRequest) (ImageJob, error) {
	if r.Finish != nil {
		if err := validFinish(*r.Finish); err != nil {
			return ImageJob{}, err
		}
	}
	if err := validateImageRequest(r); err != nil {
		return ImageJob{}, err
	}
	if r.InitAsset != "" {
		if _, err := s.referenceImage(r.InitAsset); err != nil {
			return ImageJob{}, err
		}
	}
	if r.Shared {
		s.pool.mu.Lock()
		hosting := s.pool.server != nil
		s.pool.mu.Unlock()
		if !hosting {
			return ImageJob{}, errors.New("start a worker group before submitting shared jobs")
		}
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ImageJob{}, errors.New("generator is stopping")
	}
	if !r.Shared && !s.ready {
		s.mu.Unlock()
		return ImageJob{}, errors.New("finish image engine setup first")
	}
	pending := 0
	for _, j := range s.jobs {
		if j.Status == "queued" || j.Status == "running" {
			pending++
		}
	}
	if pending >= 256 {
		s.mu.Unlock()
		return ImageJob{}, errors.New("256 jobs are queued; wait for a result before adding more")
	}
	j := &ImageJob{ID: "img-" + randomID()[:16], Request: r, Status: "queued", Created: now(), Message: "Waiting for local generator", Pack: s.catalog.ID}
	if r.Shared {
		j.Message = "Waiting for an explicitly joined worker PC"
	}
	s.jobs = append(s.jobs, j)
	if len(s.jobs) > 1000 {
		for i, x := range s.jobs {
			if x.Status != "running" && x.Status != "queued" {
				s.jobs = append(s.jobs[:i], s.jobs[i+1:]...)
				break
			}
		}
	}
	copy := *j
	s.mu.Unlock()
	s.save()
	s.kick()
	return copy, nil
}
func (s *NativeImages) kick() {
	s.mu.Lock()
	if s.closed || s.cancel != nil || !s.ready {
		s.mu.Unlock()
		return
	}
	var job *ImageJob
	for _, j := range s.jobs {
		if j.Status == "queued" && !j.Request.Shared {
			job = j
			break
		}
	}
	if job == nil {
		s.mu.Unlock()
		return
	}
	cfg := s.config
	if s.actualBackend != "" {
		cfg.Backend = s.actualBackend
	}
	cli := s.cli
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.MaxMinutes)*time.Minute)
	s.cancel = cancel
	s.active = job.ID
	job.Status = "running"
	job.Started = now()
	job.Backend = cfg.Backend
	job.Message = "Loading model weights"
	s.wg.Add(1)
	req := job.Request
	id := job.ID
	s.mu.Unlock()
	go func() {
		defer s.wg.Done()
		defer cancel()
		var result []byte
		err := s.e.acquireHeavy(ctx)
		if err == nil {
			result, err = s.run(ctx, cli, cfg, req, id, func(line string) {
				s.mu.Lock()
				job.Log = tail(job.Log+line, 16000)
				job.Message = tail(strings.TrimSpace(line), 350)
				s.mu.Unlock()
			})
			s.e.releaseHeavy()
		}
		var a AssetRecord
		if err == nil {
			a, err = s.e.storeObject(bytes.NewReader(result), id+".png", "image/png", "native Z-Image-Turbo generation")
		}
		s.mu.Lock()
		if job.Status != "cancelled" {
			job.Finished = now()
			if err != nil {
				job.Status = "failed"
				job.Message = err.Error()
			} else {
				job.Status = "completed"
				job.Message = "Image generated locally"
				job.Asset = &a
			}
		}
		s.cancel = nil
		s.active = ""
		s.mu.Unlock()
		s.save()
		if err == nil && req.Finish != nil && s.e.finishing != nil && ctx.Err() == nil {
			if _, finishErr := s.e.finishing.submit(FinishRequest{Asset: a.ID, FinishOptions: *req.Finish}); finishErr != nil {
				s.e.addEvent("FINISH", finishErr.Error())
			}
		}
		s.kick()
	}()
}

type progressLog struct {
	fn   func(string)
	mu   sync.Mutex
	file *os.File
}

func (w *progressLog) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		_, _ = w.file.Write(b)
	}
	w.fn(strings.ReplaceAll(string(b), "\r", "\n"))
	return len(b), nil
}
func imageArgs(model, text, vae, output string, r ImageRequest, cfg ImageConfig) []string {
	a := []string{"--diffusion-model", model, "--llm", text, "--vae", vae, "--prompt", r.Prompt, "--width", strconv.Itoa(r.Width), "--height", strconv.Itoa(r.Height), "--steps", strconv.Itoa(r.Steps), "--seed", strconv.FormatInt(r.Seed, 10), "--cfg-scale", "1.0", "--sampling-method", "euler", "--threads", strconv.Itoa(cfg.Threads), "--diffusion-fa", "--vae-tiling", "--output", output}
	if cfg.Backend == "cpu" {
		a = append(a, "--backend", "cpu")
	} else {
		a = append(a, "--auto-fit", "on")
	}
	return a
}
func (s *NativeImages) run(ctx context.Context, cli string, cfg ImageConfig, r ImageRequest, id string, log func(string)) ([]byte, error) {
	if err := validateImageRequest(r); err != nil {
		return nil, err
	}
	dir := filepath.Join(s.root(), "runs", id)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "generation.log"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	output := filepath.Join(dir, "output.png")
	args := imageArgs(s.modelPath("image"), s.modelPath("text"), s.modelPath("vae"), output, r, cfg)
	if r.InitAsset != "" {
		b, err := s.referenceImage(r.InitAsset)
		if err != nil {
			return nil, err
		}
		input := filepath.Join(dir, "input.png")
		if err = atomicWrite(input, b); err != nil {
			return nil, err
		}
		args = append(args, "--init-img", input, "--strength", strconv.FormatFloat(r.Strength, 'f', 3, 64))
	}
	if r.Preview {
		args = append(args, "--preview", "proj", "--preview-interval", "2", "--preview-path", filepath.Join(dir, "preview.png"))
	}
	cmd := nativeCommand(ctx, cli, args...)
	writer := &progressLog{fn: log, file: f}
	cmd.Stdout = writer
	cmd.Stderr = writer
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("generation stopped: %w", ctx.Err())
		}
		return nil, fmt.Errorf("native generator failed: %w; open the job log. Try CPU mode if the graphics driver failed", err)
	}
	b, err := os.ReadFile(output)
	if err != nil {
		return nil, fmt.Errorf("backend exited without an output image: %w", err)
	}
	if err := validatePNG(b, r); err != nil {
		return nil, err
	}
	return b, nil
}
func validatePNG(b []byte, r ImageRequest) error {
	if len(b) > 16<<20 {
		return errors.New("result exceeds the 16 MiB image limit")
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("result is not a PNG image: %w", err)
	}
	if cfg.Width != r.Width || cfg.Height != r.Height {
		return errors.New("result dimensions do not match the submitted job")
	}
	// Decode after dimensions have been bounded, rejecting corrupt image payloads.
	_, err = png.Decode(bytes.NewReader(b))
	return err
}
func (s *NativeImages) cancelJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "setup" && s.active == "setup" && s.cancel != nil {
		s.cancel()
		return nil
	}
	for _, j := range s.jobs {
		if j.ID == id && (j.Status == "running" || j.Status == "queued") {
			j.Status = "cancelled"
			j.Message = "Cancelled by you"
			j.Finished = now()
			j.Lease = ""
			if s.active == id && s.cancel != nil {
				s.cancel()
			}
			return nil
		}
	}
	return errors.New("active job not found")
}

func (e *Engine) nativeImageRoutes(mux *http.ServeMux) {
	s := e.images
	mux.HandleFunc("/api/images/state", func(w http.ResponseWriter, r *http.Request) {
		if err := decode(r, &struct{}{}); err != nil {
			apiError(w, err)
			return
		}
		jsonReply(w, s.status())
	})
	mux.HandleFunc("/api/images/setup", func(w http.ResponseWriter, r *http.Request) {
		var cfg ImageConfig
		if err := decode(r, &cfg); err != nil {
			apiError(w, err)
			return
		}
		if err := s.startSetup(cfg); err != nil {
			apiError(w, err)
			return
		}
		jsonReply(w, map[string]any{"started": true})
	})
	mux.HandleFunc("/api/images/generate", func(w http.ResponseWriter, r *http.Request) {
		var req ImageRequest
		if err := decode(r, &req); err != nil {
			apiError(w, err)
			return
		}
		j, err := s.submit(req)
		if err != nil {
			apiError(w, err)
			return
		}
		jsonReply(w, j)
	})
	mux.HandleFunc("/api/images/cancel", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			ID string `json:"id"`
		}
		if err := decode(r, &v); err != nil {
			apiError(w, err)
			return
		}
		if err := s.cancelJob(v.ID); err != nil {
			apiError(w, err)
			return
		}
		s.save()
		w.WriteHeader(204)
	})
	mux.HandleFunc("/api/images/diagnostics", func(w http.ResponseWriter, r *http.Request) {
		if err := decode(r, &struct{}{}); err != nil {
			apiError(w, err)
			return
		}
		w.Header().Set("Content-Disposition", "attachment; filename=ORIGIN0_DIAGNOSTICS.json")
		// Deliberately excludes prompts, images, peer credentials and session keys.
		s.mu.Lock()
		v := map[string]any{"version": "1.9.0", "os": runtime.GOOS, "arch": runtime.GOARCH, "cpus": runtime.NumCPU(), "config": s.config, "setup": s.setup, "ready": s.ready, "catalog": s.catalog.ID, "folder": s.root(), "actual_backend": s.actualBackend}
		s.mu.Unlock()
		jsonReply(w, v)
	})
	s.pool.routes(mux)
	s.hardwareRoutes(mux)
	s.studioRoutes(mux)
}

// On subsequent starts this only probes a previously verified native runtime.
// A changed/missing model requires setup again; no network request is made here.
func (s *NativeImages) resumeReady() {
	var receipt struct {
		Catalog string `json:"catalog"`
		CLI     string `json:"cli"`
		Runtime string `json:"runtime"`
	}
	b, err := os.ReadFile(filepath.Join(s.root(), "verified.json"))
	if err != nil || json.Unmarshal(b, &receipt) != nil || receipt.Catalog != s.catalog.ID {
		return
	}
	rel, err := filepath.Rel(filepath.Join(s.root(), "runtimes"), receipt.CLI)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return
	}
	for _, spec := range s.catalog.Files {
		st, err := os.Stat(filepath.Join(s.root(), "models", spec.Name))
		if err != nil || st.Size() != spec.Size {
			return
		}
	}
	s.mu.Lock()
	if s.closed || s.cancel != nil {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	s.cancel = cancel
	s.active = "setup"
	s.setup.Status = "running"
	s.setup.Message = "Checking the installed image engine"
	s.wg.Add(1)
	s.mu.Unlock()
	go func() {
		defer s.wg.Done()
		defer cancel()
		out, err := nativeCommand(ctx, receipt.CLI, "--list-devices").CombinedOutput()
		s.mu.Lock()
		defer s.mu.Unlock()
		s.cancel = nil
		s.active = ""
		if err != nil {
			s.setup.Status = "error"
			s.setup.Message = "Installed runtime did not start. Run setup again to repair it: " + err.Error()
			return
		}
		s.cli = receipt.CLI
		s.actualBackend = receipt.Runtime
		s.ready = true
		s.setup.Status = "ready"
		s.setup.Message = "Installed engine ready. No download needed."
		s.setup.Devices = tail(string(out), 6000)
	}()
}

var _ io.Writer = (*progressLog)(nil)
