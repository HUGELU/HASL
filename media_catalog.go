package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Catalogue packs contain complete, pinned components, not names of hosted APIs.
type StudioModel struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Engine      string         `json:"engine"`
	Recipe      string         `json:"recipe"`
	Description string         `json:"description"`
	License     string         `json:"license"`
	Source      string         `json:"source"`
	RAMGB       int            `json:"ram_gb"`
	Width       int            `json:"width"`
	Steps       int            `json:"steps"`
	Guidance    float64        `json:"guidance"`
	Files       []DownloadSpec `json:"files"`
}
type StudioInstall struct {
	ID string `json:"id"`
	SetupStatus
}

func (s *MediaStudio) model(id string) (StudioModel, error) {
	for _, m := range s.catalog {
		if m.ID == id {
			return m, nil
		}
	}
	return StudioModel{}, errors.New("unknown local model; choose a catalogue model or an imported workflow")
}
func (s *MediaStudio) modelRoot() string { return filepath.Join(s.root(), "models") }
func (s *MediaStudio) modelFile(f DownloadSpec) string {
	return filepath.Join(s.modelRoot(), f.Role, f.Name)
}
func (s *MediaStudio) modelInstalled(m StudioModel) bool {
	if m.Engine == "native" {
		s.e.images.mu.Lock()
		defer s.e.images.mu.Unlock()
		return s.e.images.ready
	}
	s.mu.Lock()
	verified := s.verified[m.ID]
	s.mu.Unlock()
	if !verified {
		return false
	}
	for _, f := range m.Files {
		st, err := os.Stat(s.modelFile(f))
		if err != nil || !st.Mode().IsRegular() || st.Size() != f.Size {
			return false
		}
	}
	return true
}
func (s *MediaStudio) installModel(id string) error {
	m, err := s.model(id)
	if err != nil {
		return err
	}
	if m.Engine == "native" {
		return s.e.images.startSetup(recommendImage(hardwareProfile(), "balanced", false).Config)
	}
	s.mu.Lock()
	if s.closed || s.installCancel != nil {
		s.mu.Unlock()
		return errors.New("a download is already running, or the application is closing")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Hour)
	s.installCancel = cancel
	var total int64
	for _, f := range m.Files {
		total += f.Size
	}
	s.install = StudioInstall{ID: id, SetupStatus: SetupStatus{Status: "downloading", Message: "Preparing verified model download", Total: total, Files: len(m.Files)}}
	s.wg.Add(1)
	s.mu.Unlock()
	go func() {
		defer s.wg.Done()
		defer cancel()
		var done int64
		var failure error
		for i, f := range m.Files {
			s.mu.Lock()
			s.install.File = i + 1
			s.mu.Unlock()
			failure = downloadPinned(ctx, s.client, f, s.modelFile(f), func(msg string, n, t int64) {
				s.mu.Lock()
				s.install.Message = msg
				s.install.Done = done + n
				s.mu.Unlock()
			})
			if failure != nil {
				break
			}
			done += f.Size
		}
		s.mu.Lock()
		s.installCancel = nil
		if failure != nil {
			s.install.Status = "failed"
			s.install.Message = failure.Error()
		} else {
			s.verified[id] = true
			s.install.Status = "ready"
			s.install.Done = total
			s.install.Message = "All model files verified. Start or refresh ComfyUI to make them available."
		}
		s.mu.Unlock()
		s.save()
	}()
	return nil
}
func validateStudioCatalogue(models []StudioModel) error {
	seen := map[string]bool{}
	for _, m := range models {
		if m.ID == "" || seen[m.ID] {
			return errors.New("duplicate or empty model id")
		}
		seen[m.ID] = true
		if m.Engine == "native" {
			continue
		}
		if len(m.Files) == 0 {
			return fmt.Errorf("%s has no downloadable components", m.ID)
		}
		for _, f := range m.Files {
			if filepath.Base(f.Name) != f.Name || strings.ContainsAny(f.Name, "/\\") || f.Name == "." || f.Name == ".." {
				return errors.New("unsafe model filename")
			}
			switch f.Role {
			case "checkpoints", "diffusion_models", "text_encoders", "vae", "loras":
			default:
				return errors.New("unsupported ComfyUI model folder")
			}
			if len(f.SHA256) != 64 || f.Size <= 0 || !strings.HasPrefix(f.URL, "https://huggingface.co/") {
				return errors.New("unverified model download")
			}
		}
	}
	return nil
}
func studioCatalog() []StudioModel {
	b, _ := assets.ReadFile("studio_catalog.json")
	var c []StudioModel
	if json.Unmarshal(b, &c) != nil || validateStudioCatalogue(c) != nil {
		return nil
	}
	return c
}
