package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

type StudioConfig struct {
	Brand         string `json:"brand"`
	Tagline       string `json:"tagline"`
	Accent        string `json:"accent"`
	Logo          string `json:"logo,omitempty"`
	ComfyURL      string `json:"comfy_url"`
	ActiveModel   string `json:"active_model"`
	ManagedPython string `json:"managed_python,omitempty"`
	ManagedRoot   string `json:"managed_root,omitempty"`
	Device        string `json:"device"`
}
type StudioRequest struct {
	Model      string   `json:"model"`
	Prompt     string   `json:"prompt"`
	Negative   string   `json:"negative_prompt"`
	Width      int      `json:"width"`
	Height     int      `json:"height"`
	Steps      int      `json:"steps"`
	Guidance   *float64 `json:"guidance_scale,omitempty"`
	Seed       int64    `json:"seed"`
	InitAsset  string   `json:"init_asset,omitempty"`
	Strength   float64  `json:"strength"`
	Lora       string   `json:"lora,omitempty"`
	LoraWeight float64  `json:"lora_weight"`
}
type ComfyJob struct {
	ID       string         `json:"id"`
	PromptID string         `json:"prompt_id"`
	Status   string         `json:"status"`
	Message  string         `json:"message"`
	Created  int64          `json:"created"`
	Request  StudioRequest  `json:"request"`
	Assets   []AssetRecord  `json:"assets"`
	Graph    map[string]any `json:"graph"`
	URL      string         `json:"-"`
}
type MediaStudio struct {
	e             *Engine
	mu            sync.Mutex
	persistMu     sync.Mutex
	wg            sync.WaitGroup
	config        StudioConfig
	catalog       []StudioModel
	verified      map[string]bool
	install       StudioInstall
	installCancel context.CancelFunc
	managedCancel context.CancelFunc
	client        *http.Client
	closed        bool
	jobs          []*ComfyJob
	cancels       map[string]context.CancelFunc
}

func newMediaStudio(e *Engine) *MediaStudio {
	s := &MediaStudio{e: e, catalog: studioCatalog(), verified: map[string]bool{}, cancels: map[string]context.CancelFunc{}, client: &http.Client{Transport: &http.Transport{ResponseHeaderTimeout: 45 * time.Second, IdleConnTimeout: 60 * time.Second}}, config: StudioConfig{Brand: "ORIGIN-0", Tagline: "Your local creative studio", Accent: "#d9ff00", ComfyURL: "http://127.0.0.1:8188", ActiveModel: "native-z-image", Device: "auto"}}
	var saved struct {
		Config   StudioConfig
		Verified map[string]bool
		Jobs     []*ComfyJob
	}
	if b, err := os.ReadFile(filepath.Join(s.root(), "studio.json")); err == nil && json.Unmarshal(b, &saved) == nil {
		if validStudioConfig(saved.Config) == nil {
			s.config = saved.Config
		}
		if saved.Verified != nil {
			s.verified = saved.Verified
		}
		s.jobs = saved.Jobs
		for _, j := range s.jobs {
			if j.Status == "queued" || j.Status == "running" {
				j.Status = "interrupted"
				j.Message = "ORIGIN restarted. Check ComfyUI history before resubmitting; an externally managed engine may still have the task."
			}
		}
	}
	return s
}
func (s *MediaStudio) root() string { return filepath.Join(s.e.dataDir, "media-studio") }
func (s *MediaStudio) save() {
	s.persistMu.Lock()
	defer s.persistMu.Unlock()
	s.mu.Lock()
	b, _ := json.MarshalIndent(struct {
		Config   StudioConfig
		Verified map[string]bool
		Jobs     []*ComfyJob
	}{s.config, s.verified, s.jobs}, "", "  ")
	s.mu.Unlock()
	_ = atomicWrite(filepath.Join(s.root(), "studio.json"), b)
}
func (s *MediaStudio) close() {
	s.mu.Lock()
	s.closed = true
	if s.installCancel != nil {
		s.installCancel()
	}
	if s.managedCancel != nil {
		s.managedCancel()
	}
	for _, c := range s.cancels {
		c()
	}
	s.mu.Unlock()
	s.wg.Wait()
	s.save()
}
func validStudioConfig(c StudioConfig) error {
	if len(c.Brand) < 1 || len(c.Brand) > 60 || len(c.Tagline) > 160 || !regexp.MustCompile(`^#[0-9a-fA-F]{6}$`).MatchString(c.Accent) {
		return errors.New("enter a name up to 60 characters, a short tagline and a six-digit colour")
	}
	if err := validComfyURL(c.ComfyURL); err != nil {
		return err
	}
	switch c.Device {
	case "auto", "cpu", "intel", "nvidia", "amd":
	default:
		return errors.New("invalid compute device")
	}
	return nil
}
func (s *MediaStudio) state() any {
	s.mu.Lock()
	c := s.config
	i := s.install
	b, _ := json.Marshal(s.jobs)
	s.mu.Unlock()
	var jobs any
	_ = json.Unmarshal(b, &jobs)
	list := []any{}
	for _, m := range s.catalog {
		var size int64
		for _, f := range m.Files {
			size += f.Size
		}
		list = append(list, map[string]any{"model": m, "installed": s.modelInstalled(m), "download_bytes": size})
	}
	return map[string]any{"config": c, "catalog": list, "install": i, "jobs": jobs, "hardware": hardwareProfile(), "model_folder": s.modelRoot(), "upstream": "Autom8AI/Open-Higgsfield-AI", "upstream_commit": "b578108936e83a3b2a5e86644057a56f8aea73a1"}
}
func (s *MediaStudio) generate(q StudioRequest) (any, error) {
	m, err := s.model(q.Model)
	if err != nil {
		return nil, err
	}
	if q.Width == 0 {
		q.Width = m.Width
	}
	if q.Height == 0 {
		q.Height = q.Width
	}
	if q.Steps == 0 {
		q.Steps = m.Steps
	}
	if q.Guidance == nil {
		q.Guidance = &m.Guidance
	}
	if m.Engine == "native" {
		if q.Seed == -1 {
			q.Seed = time.Now().UnixNano() & 0x7fffffff
		}
		if q.Negative != "" || *q.Guidance != 1 {
			return nil, errors.New("the native distilled model uses fixed guidance; choose a ComfyUI model for negative prompts and guidance controls")
		}
		if q.Lora != "" {
			return nil, errors.New("use ComfyUI for LoRA workflows")
		}
		r := ImageRequest{Prompt: q.Prompt, Width: q.Width, Height: q.Height, Steps: q.Steps, Seed: q.Seed, InitAsset: q.InitAsset, Strength: q.Strength, Preview: true}
		return s.e.images.submit(r)
	}
	if !s.modelInstalled(m) {
		return nil, errors.New("download and verify this model pack first")
	}
	return s.submitComfy(q, m)
}
func (e *Engine) mediaStudioRoutes(mux *http.ServeMux) {
	s := e.mediaStudio
	mux.HandleFunc("/studio/ws", e.comfyLive)
	sub, _ := fs.Sub(assets, "web/open-studio")
	mux.Handle("/open-studio/", http.StripPrefix("/open-studio/", http.FileServer(http.FS(sub))))
	mux.HandleFunc("/api/studio/state", func(w http.ResponseWriter, r *http.Request) { jsonReply(w, s.state()) })
	mux.HandleFunc("/api/studio/config", func(w http.ResponseWriter, r *http.Request) {
		var q StudioConfig
		if err := decode(r, &q); err != nil {
			apiError(w, err)
			return
		}
		if err := validStudioConfig(q); err != nil {
			apiError(w, err)
			return
		}
		if _, err := s.model(q.ActiveModel); err != nil {
			apiError(w, err)
			return
		}
		if q.Logo != "" {
			if _, err := e.objectPath(q.Logo); err != nil {
				apiError(w, err)
				return
			}
		}
		s.mu.Lock()
		q.ManagedPython = s.config.ManagedPython
		q.ManagedRoot = s.config.ManagedRoot
		s.config = q
		s.mu.Unlock()
		s.save()
		jsonReply(w, s.state())
	})
	mux.HandleFunc("/api/studio/install", func(w http.ResponseWriter, r *http.Request) {
		var q struct{ ID string }
		if err := decode(r, &q); err != nil {
			apiError(w, err)
			return
		}
		if err := s.installModel(q.ID); err != nil {
			apiError(w, err)
			return
		}
		jsonReply(w, s.state())
	})
	mux.HandleFunc("/api/studio/stop-download", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		if s.installCancel != nil {
			s.installCancel()
		}
		s.mu.Unlock()
		w.WriteHeader(204)
	})
	mux.HandleFunc("/api/studio/generate", func(w http.ResponseWriter, r *http.Request) {
		var q StudioRequest
		if err := decode(r, &q); err != nil {
			apiError(w, err)
			return
		}
		v, err := s.generate(q)
		if err != nil {
			apiError(w, err)
			return
		}
		jsonReply(w, v)
	})
	mux.HandleFunc("/api/studio/comfy", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.comfyProbe(r.Context())
		if err != nil {
			apiError(w, err)
			return
		}
		jsonReply(w, v)
	})
	mux.HandleFunc("/api/studio/comfy-setup", func(w http.ResponseWriter, r *http.Request) {
		if err := s.setupComfy(); err != nil {
			apiError(w, err)
			return
		}
		jsonReply(w, s.state())
	})
	mux.HandleFunc("/api/studio/comfy-start", func(w http.ResponseWriter, r *http.Request) {
		if err := s.startComfy(); err != nil {
			apiError(w, err)
			return
		}
		jsonReply(w, s.state())
	})
	mux.HandleFunc("/api/studio/workflow", func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Graph  map[string]any `json:"graph"`
			Prompt string         `json:"prompt"`
		}
		if err := decode(r, &q); err != nil {
			apiError(w, err)
			return
		}
		j, err := s.submitGraph(StudioRequest{Model: "custom-workflow", Prompt: q.Prompt}, q.Graph)
		if err != nil {
			apiError(w, err)
			return
		}
		jsonReply(w, j)
	})
	mux.HandleFunc("/api/studio/cancel", func(w http.ResponseWriter, r *http.Request) {
		var q struct{ ID string }
		if err := decode(r, &q); err != nil {
			apiError(w, err)
			return
		}
		if err := s.cancelComfy(q.ID); err != nil {
			apiError(w, err)
			return
		}
		w.WriteHeader(204)
	})
}
