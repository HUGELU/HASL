package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Interoperate with the actual desktop manager. Never rewrite its settings or package environments.
type MatrixPackage struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	PackageName string `json:"package_name"`
	LibraryPath string `json:"library_path"`
}
type matrixSettings struct {
	ModelDirectoryOverride string
	InstalledPackages      []struct {
		ID          string
		DisplayName string
		PackageName string
		LibraryPath string
	}
}

func readSmallJSON(p string, v any) error {
	f, e := os.Open(p)
	if e != nil {
		return e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, (4<<20)+1))
	if e != nil {
		return e
	}
	if len(b) > 4<<20 {
		return errors.New("settings file exceeds 4 MB")
	}
	return json.Unmarshal(b, v)
}
func detectMatrixLibrary() string {
	config, e := os.UserConfigDir()
	if e != nil {
		return ""
	}
	var v struct{ LibraryPath string }
	if readSmallJSON(filepath.Join(config, "StabilityMatrix", "library.json"), &v) != nil {
		return ""
	}
	if st, e := os.Stat(v.LibraryPath); e == nil && st.IsDir() {
		return v.LibraryPath
	}
	return ""
}
func matrixModelRoot(library string) string {
	var settings matrixSettings
	if readSmallJSON(filepath.Join(library, "settings.json"), &settings) == nil && filepath.IsAbs(settings.ModelDirectoryOverride) {
		return settings.ModelDirectoryOverride
	}
	return filepath.Join(library, "Models")
}
func (s *MediaStudio) matrixState() any {
	s.mu.Lock()
	c := s.config
	s.mu.Unlock()
	packages := []MatrixPackage{}
	var settings matrixSettings
	if c.MatrixLibrary != "" && readSmallJSON(filepath.Join(c.MatrixLibrary, "settings.json"), &settings) == nil {
		for _, p := range settings.InstalledPackages {
			packages = append(packages, MatrixPackage{p.ID, p.DisplayName, p.PackageName, p.LibraryPath})
		}
	}
	exe := c.MatrixExecutable
	if exe == "" {
		exe = filepath.Join(s.root(), "stability-matrix-v2.16.3", "StabilityMatrix.exe")
	}
	_, err := os.Stat(exe)
	models := ""
	if c.MatrixLibrary != "" {
		models = matrixModelRoot(c.MatrixLibrary)
	}
	return map[string]any{"executable": exe, "installed": err == nil, "library": c.MatrixLibrary, "models": models, "packages": packages, "source": "https://github.com/LykosAI/StabilityMatrix", "version": "2.16.3", "download_bytes": int64(141010114), "binary_terms": "https://lykos.ai/eula"}
}
func (s *MediaStudio) linkMatrix(library, executable string) error {
	library = strings.TrimSpace(library)
	executable = strings.TrimSpace(executable)
	if !filepath.IsAbs(library) {
		return errors.New("choose the full path to your Stability Matrix Data directory")
	}
	st, e := os.Stat(library)
	if e != nil || !st.IsDir() {
		return errors.New("Stability Matrix Data directory not found")
	}
	if _, e = os.Stat(filepath.Join(library, "settings.json")); e != nil {
		if _, e = os.Stat(filepath.Join(library, "Models")); e != nil {
			return errors.New("select the Data directory containing settings.json or Models")
		}
	}
	if executable != "" {
		if !filepath.IsAbs(executable) {
			return errors.New("use the full Stability Matrix executable path")
		}
		st, e = os.Stat(executable)
		if e != nil || !st.Mode().IsRegular() {
			return errors.New("Stability Matrix executable not found")
		}
	}
	s.mu.Lock()
	s.config.MatrixLibrary = filepath.Clean(library)
	if executable != "" {
		s.config.MatrixExecutable = filepath.Clean(executable)
	}
	s.mu.Unlock()
	s.save()
	return nil
}

var matrixSpec = DownloadSpec{Name: "StabilityMatrix-win-x64.zip", URL: "https://github.com/LykosAI/StabilityMatrix/releases/download/v2.16.3/StabilityMatrix-win-x64.zip", SHA256: "45b2c306753ea9ab9620d4c12bf72bd9778c4014bc85bebd02ccb5eb8c92851a", Size: 141010114, License: "Official binary EULA; AGPL-3.0 source", Source: "https://github.com/LykosAI/StabilityMatrix/tree/v2.16.3"}

func (s *MediaStudio) installMatrix() error {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		return errors.New("automatic Stability Matrix installation supports Windows x64; connect an existing installation on other platforms")
	}
	dest := filepath.Join(s.root(), "stability-matrix-v2.16.3")
	if _, e := os.Stat(dest); e == nil {
		return errors.New("Stability Matrix is already installed; use Open Stability Matrix")
	}
	s.mu.Lock()
	if s.closed || s.installCancel != nil || s.prepareCancel != nil {
		s.mu.Unlock()
		return errors.New("finish or stop the current setup first")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	s.installCancel = cancel
	s.install = StudioInstall{ID: "stability-matrix", SetupStatus: SetupStatus{Status: "downloading", Message: "Downloading official Stability Matrix", Total: matrixSpec.Size}}
	s.wg.Add(1)
	s.mu.Unlock()
	go func() {
		defer s.wg.Done()
		defer cancel()
		archive := filepath.Join(s.root(), "downloads", matrixSpec.Name)
		err := downloadPinned(ctx, s.client, matrixSpec, archive, func(msg string, n, t int64) {
			s.mu.Lock()
			s.install.Message = msg
			s.install.Done = n
			s.install.Total = t
			s.mu.Unlock()
		})
		staging := ""
		if err == nil {
			staging, err = os.MkdirTemp(s.root(), "matrix-staging-")
		}
		if err == nil {
			s.mu.Lock()
			s.install.Status = "extracting"
			s.install.Message = "Extracting verified Stability Matrix"
			s.mu.Unlock()
			err = extractRuntime(archive, staging)
		}
		executable := ""
		if err == nil {
			err = filepath.WalkDir(staging, func(p string, d os.DirEntry, e error) error {
				if e != nil {
					return e
				}
				if !d.IsDir() && strings.EqualFold(d.Name(), "StabilityMatrix.exe") {
					executable = p
				}
				return nil
			})
			if executable == "" {
				err = errors.New("verified archive has no StabilityMatrix.exe")
			}
		}
		if err == nil {
			rel, _ := filepath.Rel(staging, executable)
			err = os.Rename(staging, dest)
			executable = filepath.Join(dest, rel)
		}
		if err != nil && staging != "" {
			_ = os.RemoveAll(staging)
		}
		s.mu.Lock()
		s.installCancel = nil
		if err != nil {
			s.install.Status = "failed"
			s.install.Message = err.Error()
		} else {
			s.config.MatrixExecutable = executable
			if s.config.MatrixLibrary == "" {
				s.config.MatrixLibrary = filepath.Join(s.root(), "matrix-data")
			}
			s.install.Status = "ready"
			s.install.Message = "Stability Matrix installed. Open it to manage packages and connect its shared library."
		}
		s.mu.Unlock()
		s.save()
	}()
	return nil
}
func (s *MediaStudio) launchMatrix(packageID string) error {
	s.mu.Lock()
	c := s.config
	s.mu.Unlock()
	if c.MatrixExecutable == "" {
		return errors.New("install or link Stability Matrix first")
	}
	if !filepath.IsAbs(c.MatrixExecutable) {
		return errors.New("invalid Stability Matrix executable path")
	}
	if c.MatrixLibrary == "" {
		return errors.New("link its Data directory first")
	}
	if packageID != "" {
		var v matrixSettings
		if e := readSmallJSON(filepath.Join(c.MatrixLibrary, "settings.json"), &v); e != nil {
			return e
		}
		found := false
		for _, p := range v.InstalledPackages {
			if p.ID == packageID {
				found = true
			}
		}
		if !found {
			return errors.New("package is not installed in this Stability Matrix library")
		}
	}
	if e := os.MkdirAll(c.MatrixLibrary, 0700); e != nil {
		return e
	}
	args := []string{"--data-dir", c.MatrixLibrary, "--no-sentry"}
	if packageID != "" {
		args = append(args, "--launch-package", packageID)
	}
	log, e := os.OpenFile(filepath.Join(s.root(), "stability-matrix.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	cmd := nativeCommand(context.Background(), c.MatrixExecutable, args...)
	cmd.Dir = filepath.Dir(c.MatrixExecutable)
	cmd.Stdout = log
	cmd.Stderr = log
	if e = cmd.Start(); e != nil {
		log.Close()
		return e
	}
	// Matrix is a user-opened desktop application; closing ORIGIN does not terminate it.
	go func() { defer log.Close(); _ = cmd.Wait() }()
	return nil
}
func (s *MediaStudio) extraModelPaths() string {
	s.mu.Lock()
	library := s.config.MatrixLibrary
	s.mu.Unlock()
	yaml := "origin0:\n  base_path: " + fmt.Sprintf("%q", filepath.ToSlash(s.modelRoot())) + "\n  is_default: true\n"
	for _, role := range []string{"checkpoints", "diffusion_models", "text_encoders", "vae", "loras", "controlnet", "clip_vision", "upscale_models", "embeddings"} {
		yaml += "  " + role + ": " + role + "\n"
	}
	if library != "" {
		yaml += "stability_matrix:\n  base_path: " + fmt.Sprintf("%q", filepath.ToSlash(matrixModelRoot(library))) + "\n  checkpoints: StableDiffusion\n  diffusion_models: DiffusionModels\n  text_encoders: TextEncoders\n  loras: |\n    Lora\n    LyCORIS\n  vae: VAE\n  controlnet: ControlNet\n  clip_vision: ClipVision\n  embeddings: Embeddings\n  upscale_models: |\n    ESRGAN\n    RealESRGAN\n    SwinIR\n"
	}
	return yaml
}
func (s *MediaStudio) registerCheckpoint(ctx context.Context, name, profile string) (StudioModel, error) {
	recipe, width, steps, guidance, e := recipeDefaults(profile)
	if e != nil || recipe == "workflow" {
		return StudioModel{}, errors.New("choose SD 1.5 or SDXL for this checkpoint")
	}
	var info map[string]any
	if e = s.comfyJSON(ctx, s.comfyURL(), "/object_info/CheckpointLoaderSimple", nil, &info); e != nil {
		return StudioModel{}, e
	}
	found := false
	for _, n := range loaderChoices(info, "CheckpointLoaderSimple", "ckpt_name") {
		if n == name {
			found = true
		}
	}
	if !found {
		return StudioModel{}, errors.New("this checkpoint is no longer listed by the running engine; refresh installed models")
	}
	key := shortHash(name + "\n" + profile)
	m := StudioModel{ID: "connected-" + key, Name: name, Engine: "comfy", Recipe: recipe, Description: "Checkpoint from your connected ComfyUI / Stability Matrix library", License: "Existing local model; see its original model card", Source: "https://github.com/LykosAI/StabilityMatrix", Width: width, Steps: steps, Guidance: guidance, RAMGB: 16, External: true, Files: []DownloadSpec{{Role: "checkpoints", Name: name}}}
	s.mu.Lock()
	found = false
	for i := range s.community {
		if s.community[i].ID == m.ID {
			s.community[i] = m
			found = true
		}
	}
	if !found {
		s.community = append(s.community, m)
	}
	s.config.ActiveModel = m.ID
	s.mu.Unlock()
	s.save()
	return m, nil
}
func loaderChoices(info map[string]any, class, field string) []string {
	out := []string{}
	n, _ := info[class].(map[string]any)
	inputs, _ := n["input"].(map[string]any)
	required, _ := inputs["required"].(map[string]any)
	values, _ := required[field].([]any)
	if len(values) > 0 {
		list, _ := values[0].([]any)
		for _, v := range list {
			if x, ok := v.(string); ok {
				out = append(out, x)
			}
		}
	}
	return out
}
func (s *MediaStudio) studioDiagnostics() any {
	s.mu.Lock()
	c := s.config
	i := s.install
	p := s.preparation
	s.mu.Unlock()
	return map[string]any{"version": "1.9.0", "hardware": hardwareProfile(), "config": c, "install": i, "preparation": p, "comfy_log": logTail(filepath.Join(s.root(), "comfy-runtime.log")), "matrix_log": logTail(filepath.Join(s.root(), "stability-matrix.log"))}
}
func (s *MediaStudio) writeModelPaths(w http.ResponseWriter, r *http.Request) {
	if e := decode(r, &struct{}{}); e != nil {
		apiError(w, e)
		return
	}
	w.Header().Set("Content-Type", "text/yaml")
	w.Header().Set("Content-Disposition", "attachment; filename=origin0_shared_models.yaml")
	_, _ = io.WriteString(w, s.extraModelPaths())
}
