package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type StudioPreparation struct {
	Model   string `json:"model"`
	Status  string `json:"status"`
	Stage   string `json:"stage"`
	Message string `json:"message"`
	Started int64  `json:"started"`
}

func (s *MediaStudio) prepMessage(stage, message string) {
	s.mu.Lock()
	s.preparation.Stage = stage
	s.preparation.Message = message
	s.mu.Unlock()
}
func (s *MediaStudio) awaitInstall(ctx context.Context, native bool) error {
	timer := time.NewTicker(400 * time.Millisecond)
	defer timer.Stop()
	lastMessage := ""
	lastActivity := time.Now()
	for {
		var st SetupStatus
		if native {
			s.e.images.mu.Lock()
			st = s.e.images.setup
			s.e.images.mu.Unlock()
		} else {
			s.mu.Lock()
			st = s.install.SetupStatus
			s.mu.Unlock()
		}
		activity := fmt.Sprintf("%s:%s:%d", st.Status, st.Message, st.Done)
		if activity != lastMessage {
			lastMessage = activity
			lastActivity = time.Now()
			s.prepMessage(st.Status, st.Message)
		}
		switch st.Status {
		case "ready":
			return nil
		case "failed", "error", "cancelled":
			return errors.New(st.Message)
		}
		if strings.HasPrefix(st.Message, "Downloading") && time.Since(lastActivity) > 3*time.Minute {
			return errors.New("download has transferred no data for three minutes. Stop/retry resumes the partial file; check connection or provider access")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
}
func (s *MediaStudio) awaitComfy(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	last := "engine has not answered"
	for {
		probeCtx, stop := context.WithTimeout(ctx, 3*time.Second)
		var system map[string]any
		e := s.comfyJSON(probeCtx, s.comfyURL(), "/system_stats", nil, &system)
		stop()
		if e == nil {
			return nil
		}
		last = e.Error()
		select {
		case <-ctx.Done():
			return fmt.Errorf("ComfyUI did not become ready within three minutes: %s. Open Engine status for the startup log", last)
		case <-ticker.C:
		}
	}
}
func (s *MediaStudio) prepareModel(id string) error {
	m, e := s.model(id)
	if e != nil {
		return e
	}
	if m.Recipe == "workflow" {
		return errors.New("this is a workflow component; use its matching workflow")
	}
	if m.Engine == "native" {
		s.e.images.mu.Lock()
		busy := s.e.images.cancel != nil
		s.e.images.mu.Unlock()
		if busy {
			return errors.New("finish or stop the current native engine task first")
		}
	}
	s.mu.Lock()
	if s.closed || s.prepareCancel != nil || s.installCancel != nil {
		s.mu.Unlock()
		return errors.New("finish or stop the current setup first")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Hour)
	s.prepareCancel = cancel
	s.preparation = StudioPreparation{Model: id, Status: "running", Stage: "checking", Message: "Checking model and engine", Started: time.Now().Unix()}
	s.wg.Add(1)
	s.mu.Unlock()
	go func() {
		defer s.wg.Done()
		defer cancel()
		err := func() error {
			if m.Engine == "native" {
				if !s.modelInstalled(m) {
					if e := s.installModel(id); e != nil {
						return e
					}
					if e := s.awaitInstall(ctx, true); e != nil {
						return e
					}
				}
				return nil
			}
			probeCtx, done := context.WithTimeout(ctx, 3*time.Second)
			var system map[string]any
			e := s.comfyJSON(probeCtx, s.comfyURL(), "/system_stats", nil, &system)
			done()
			if e != nil {
				s.prepMessage("engine", "Preparing ComfyUI")
				s.mu.Lock()
				installed := s.config.ManagedPython != ""
				running := s.managedCancel != nil
				s.mu.Unlock()
				if !running {
					if installed {
						e = s.startComfy()
					} else {
						e = s.setupComfy()
					}
					if e != nil {
						return e
					}
				}
				if !installed && !running {
					if e = s.awaitInstall(ctx, false); e != nil {
						return e
					}
				}
				s.prepMessage("starting", "Waiting for the engine to answer")
				if e = s.awaitComfy(ctx); e != nil {
					return e
				}
			}
			if !s.modelInstalled(m) {
				s.prepMessage("model", "Downloading the complete model pack")
				if e = s.installModel(id); e != nil {
					return e
				}
				if e = s.awaitInstall(ctx, false); e != nil {
					return e
				}
			}
			s.prepMessage("checking", "Verifying model availability in the running engine")
			checkCtx, stop := context.WithTimeout(ctx, 30*time.Second)
			defer stop()
			return s.checkModelLoaders(checkCtx, m)
		}()
		if err != nil {
			s.mu.Lock()
			if s.installCancel != nil {
				s.installCancel()
			}
			s.mu.Unlock()
			if m.Engine == "native" {
				s.e.images.mu.Lock()
				if s.e.images.cancel != nil {
					s.e.images.cancel()
				}
				s.e.images.mu.Unlock()
			}
		}
		s.mu.Lock()
		s.prepareCancel = nil
		if err != nil {
			s.preparation.Status = "failed"
			s.preparation.Message = err.Error()
			if ctx.Err() != nil {
				s.preparation.Status = "cancelled"
				s.preparation.Message = "Setup stopped. Set up & use resumes verified downloads."
			}
		} else {
			s.config.ActiveModel = id
			s.preparation.Status = "ready"
			s.preparation.Stage = "ready"
			s.preparation.Message = "Engine ready and model files available. Create an image."
		}
		s.mu.Unlock()
		s.save()
	}()
	return nil
}
func (s *MediaStudio) checkModelLoaders(ctx context.Context, m StudioModel) error {
	var info map[string]any
	if e := s.comfyJSON(ctx, s.comfyURL(), "/object_info", nil, &info); e != nil {
		return e
	}
	for _, f := range m.Files {
		class, field := "", ""
		switch f.Role {
		case "checkpoints":
			class, field = "CheckpointLoaderSimple", "ckpt_name"
		case "diffusion_models":
			class, field = "UNETLoader", "unet_name"
		case "text_encoders":
			class, field = "CLIPLoader", "clip_name"
		case "vae":
			class, field = "VAELoader", "vae_name"
		default:
			continue
		}
		found := false
		for _, v := range loaderChoices(info, class, field) {
			if v == f.Name {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("%s is downloaded but is not listed by %s. Restart your connected engine with ORIGIN's shared model paths", f.Name, class)
		}
	}
	return nil
}
func (s *MediaStudio) readiness(ctx context.Context) any {
	s.mu.Lock()
	c := s.config
	p := s.preparation
	s.mu.Unlock()
	m, e := s.model(c.ActiveModel)
	if e != nil {
		return map[string]any{"ready": false, "message": e.Error(), "preparation": p}
	}
	if m.Engine == "native" {
		s.e.images.mu.Lock()
		ready := s.e.images.ready
		backend := s.e.images.actualBackend
		setup := s.e.images.setup
		s.e.images.mu.Unlock()
		return map[string]any{"ready": ready, "engine": "native", "device": backend, "message": setup.Message, "preparation": p}
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	var stats map[string]any
	e = s.comfyJSON(ctx, c.ComfyURL, "/system_stats", nil, &stats)
	message := "Engine connected"
	if e != nil {
		message = "ComfyUI is not answering. Use Set up & use, or open Engine status."
	}
	return map[string]any{"ready": e == nil && s.modelInstalled(m), "engine": "comfy", "system": stats, "message": message, "preparation": p}
}
func (s *MediaStudio) ecosystemRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/studio/prepare", func(w http.ResponseWriter, r *http.Request) {
		var q struct{ ID string }
		if e := decode(r, &q); e != nil {
			apiError(w, e)
			return
		}
		if e := s.prepareModel(q.ID); e != nil {
			apiError(w, e)
			return
		}
		jsonReply(w, s.state())
	})
	mux.HandleFunc("/api/studio/readiness", func(w http.ResponseWriter, r *http.Request) {
		if e := decode(r, &struct{}{}); e != nil {
			apiError(w, e)
			return
		}
		jsonReply(w, s.readiness(r.Context()))
	})
	mux.HandleFunc("/api/studio/source/search", func(w http.ResponseWriter, r *http.Request) {
		var q SourceQuery
		if e := decode(r, &q); e != nil {
			apiError(w, e)
			return
		}
		v, e := s.searchSource(r.Context(), q)
		if e != nil {
			apiError(w, e)
			return
		}
		jsonReply(w, v)
	})
	mux.HandleFunc("/api/studio/source/inspect", func(w http.ResponseWriter, r *http.Request) {
		var q SourceQuery
		if e := decode(r, &q); e != nil {
			apiError(w, e)
			return
		}
		v, e := s.inspectSource(r.Context(), q)
		if e != nil {
			apiError(w, e)
			return
		}
		jsonReply(w, v)
	})
	mux.HandleFunc("/api/studio/source/import", func(w http.ResponseWriter, r *http.Request) {
		var q SourceQuery
		if e := decode(r, &q); e != nil {
			apiError(w, e)
			return
		}
		v, e := s.registerSource(r.Context(), q)
		if e != nil {
			apiError(w, e)
			return
		}
		jsonReply(w, v)
	})
	mux.HandleFunc("/api/studio/source/keys", func(w http.ResponseWriter, r *http.Request) {
		var q ProviderKeys
		if e := decode(r, &q); e != nil {
			apiError(w, e)
			return
		}
		if len(q.HF) > 1000 || len(q.Civitai) > 1000 || strings.ContainsAny(q.HF+q.Civitai, "\r\n") {
			apiError(w, errors.New("invalid provider token"))
			return
		}
		if e := s.saveProviderKeys(q); e != nil {
			apiError(w, e)
			return
		}
		jsonReply(w, map[string]bool{"saved": true})
	})
	mux.HandleFunc("/api/studio/matrix", func(w http.ResponseWriter, r *http.Request) {
		if e := decode(r, &struct{}{}); e != nil {
			apiError(w, e)
			return
		}
		jsonReply(w, s.matrixState())
	})
	mux.HandleFunc("/api/studio/matrix/link", func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Library    string
			Executable string
		}
		if e := decode(r, &q); e != nil {
			apiError(w, e)
			return
		}
		if e := s.linkMatrix(q.Library, q.Executable); e != nil {
			apiError(w, e)
			return
		}
		jsonReply(w, s.matrixState())
	})
	mux.HandleFunc("/api/studio/matrix/install", func(w http.ResponseWriter, r *http.Request) {
		if e := decode(r, &struct{}{}); e != nil {
			apiError(w, e)
			return
		}
		if e := s.installMatrix(); e != nil {
			apiError(w, e)
			return
		}
		jsonReply(w, s.state())
	})
	mux.HandleFunc("/api/studio/matrix/open", func(w http.ResponseWriter, r *http.Request) {
		var q struct{ Package string }
		if e := decode(r, &q); e != nil {
			apiError(w, e)
			return
		}
		if e := s.launchMatrix(q.Package); e != nil {
			apiError(w, e)
			return
		}
		jsonReply(w, map[string]bool{"launched": true})
	})
	mux.HandleFunc("/api/studio/checkpoint", func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Name    string
			Profile string
		}
		if e := decode(r, &q); e != nil {
			apiError(w, e)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		v, e := s.registerCheckpoint(ctx, q.Name, q.Profile)
		if e != nil {
			apiError(w, e)
			return
		}
		jsonReply(w, v)
	})
	mux.HandleFunc("/api/studio/model-paths", s.writeModelPaths)
	mux.HandleFunc("/api/studio/diagnostics", func(w http.ResponseWriter, r *http.Request) {
		if e := decode(r, &struct{}{}); e != nil {
			apiError(w, e)
			return
		}
		jsonReply(w, s.studioDiagnostics())
	})
}
