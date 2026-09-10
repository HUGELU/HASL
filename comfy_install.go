package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Extract only ordinary files inside a new staging directory. Never overwrite an existing installation.
func extractComfyArchive(ctx context.Context, archive, dest string) error {
	// Windows' bundled bsdtar reads 7z without a separate installer. The archive is
	// pinned and SHA-256 verified before this function is called.
	listing, err := nativeCommand(ctx, "tar.exe", "-tf", archive).Output()
	if err != nil {
		return fmt.Errorf("Windows tar could not inspect the verified portable archive: %w", err)
	}
	for _, name := range strings.Split(string(listing), "\n") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		clean := filepath.Clean(filepath.FromSlash(strings.ReplaceAll(name, "\\", "/")))
		if !filepath.IsLocal(clean) || strings.Contains(clean, ":") {
			return errors.New("unsafe archive path")
		}
	}
	cmd := nativeCommand(ctx, "tar.exe", "-xf", archive, "-C", dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("portable extraction failed: %w: %s", err, shortText(string(out), 2000))
	}
	return nil
}

func (s *MediaStudio) setupComfy() error {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		return errors.New("managed portable setup currently supports Windows x64. On macOS/Linux connect an installed ComfyUI at the local address below")
	}
	s.mu.Lock()
	if s.closed || s.installCancel != nil {
		s.mu.Unlock()
		return errors.New("finish the current download before installing ComfyUI")
	}
	device := s.config.Device
	s.mu.Unlock()
	if device == "auto" {
		g := strings.ToLower(hardwareProfile().GPUs)
		switch {
		case strings.Contains(g, "nvidia"):
			device = "nvidia"
		case strings.Contains(g, "amd") || strings.Contains(g, "radeon"):
			device = "amd"
		default:
			device = "intel"
		}
	}
	if device == "cpu" {
		device = "intel"
	}
	var specs []DownloadSpec
	b, _ := assets.ReadFile("comfy_runtimes.json")
	if err := json.Unmarshal(b, &specs); err != nil {
		return err
	}
	var spec DownloadSpec
	for _, v := range specs {
		if v.Role == device {
			spec = v
		}
	}
	if spec.URL == "" {
		return errors.New("no pinned portable package for this device")
	}
	dest := filepath.Join(s.root(), "comfy-v0.35.0-"+device)
	if _, err := os.Stat(dest); err == nil {
		return errors.New("this portable installation already exists; use Start ComfyUI. Existing installations are not overwritten")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Hour)
	s.mu.Lock()
	if s.installCancel != nil || s.closed {
		s.mu.Unlock()
		cancel()
		return errors.New("studio is busy")
	}
	s.installCancel = cancel
	s.install = StudioInstall{ID: "comfy-runtime", SetupStatus: SetupStatus{Status: "downloading", Message: "Downloading official ComfyUI portable; includes its own Python", Total: spec.Size}}
	s.wg.Add(1)
	s.mu.Unlock()
	go func() {
		defer s.wg.Done()
		defer cancel()
		archive := filepath.Join(s.root(), "downloads", spec.Name)
		err := downloadPinned(ctx, s.client, spec, archive, func(msg string, n, t int64) {
			s.mu.Lock()
			s.install.Message = msg
			s.install.Done = n
			s.install.Total = t
			s.mu.Unlock()
		})
		staging := ""
		if err == nil {
			staging, err = os.MkdirTemp(s.root(), "comfy-staging-")
		}
		if err == nil {
			s.mu.Lock()
			s.install.Status = "extracting"
			s.install.Message = "Checksum verified. Extracting the portable environment; this can take several minutes."
			s.mu.Unlock()
			err = extractComfyArchive(ctx, archive, staging)
		}
		var py, root string
		if err == nil {
			err = filepath.WalkDir(staging, func(p string, d os.DirEntry, e error) error {
				if e != nil {
					return e
				}
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if !d.IsDir() && d.Name() == "python.exe" && strings.Contains(filepath.ToSlash(p), "/python_embed") {
					py = p
				}
				if !d.IsDir() && d.Name() == "main.py" && filepath.Base(filepath.Dir(p)) == "ComfyUI" {
					root = filepath.Dir(p)
				}
				return nil
			})
			if err == nil && (py == "" || root == "") {
				err = errors.New("portable archive has no recognised embedded Python and ComfyUI host")
			}
		}
		if err == nil {
			pyrel, _ := filepath.Rel(staging, py)
			rrel, _ := filepath.Rel(staging, root)
			err = os.Rename(staging, dest)
			if err == nil {
				s.mu.Lock()
				s.config.ManagedPython = filepath.Join(dest, pyrel)
				s.config.ManagedRoot = filepath.Join(dest, rrel)
				s.mu.Unlock()
			}
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
			s.install.Status = "ready"
			s.install.Message = "Official ComfyUI portable installed. Starting the engine…"
		}
		s.mu.Unlock()
		s.save()
		if err == nil {
			if err = s.startComfy(); err != nil {
				s.mu.Lock()
				s.install.Status = "failed"
				s.install.Message = err.Error()
				s.mu.Unlock()
			}
		}
	}()
	return nil
}
func (s *MediaStudio) startComfy() error {
	s.mu.Lock()
	cfg := s.config
	if s.closed || s.managedCancel != nil {
		s.mu.Unlock()
		return errors.New("managed ComfyUI is already starting/running, or ORIGIN is closing")
	}
	s.mu.Unlock()
	if cfg.ManagedPython == "" || cfg.ManagedRoot == "" {
		return errors.New("install the portable ComfyUI engine first, or connect your existing ComfyUI")
	}
	if _, err := os.Stat(cfg.ManagedPython); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(cfg.ManagedRoot, "main.py")); err != nil {
		return err
	}
	// A dedicated local port avoids hijacking another ComfyUI process.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	yaml := "origin0:\n  base_path: " + fmt.Sprintf("%q", filepath.ToSlash(s.modelRoot())) + "\n  is_default: true\n  checkpoints: checkpoints\n  diffusion_models: diffusion_models\n  text_encoders: text_encoders\n  vae: vae\n  loras: loras\n"
	ypath := filepath.Join(s.root(), "extra_model_paths.yaml")
	if err = atomicWrite(ypath, []byte(yaml)); err != nil {
		return err
	}
	logpath := filepath.Join(s.root(), "comfy-runtime.log")
	log, err := os.OpenFile(logpath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	if s.closed || s.managedCancel != nil {
		s.mu.Unlock()
		cancel()
		log.Close()
		return errors.New("ComfyUI is already running")
	}
	s.managedCancel = cancel
	s.config.ComfyURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	s.install.Message = "Starting ComfyUI. Live startup output: " + logpath
	s.wg.Add(1)
	s.mu.Unlock()
	s.save()
	go func() {
		defer s.wg.Done()
		defer cancel()
		defer log.Close()
		defer func() { s.mu.Lock(); s.managedCancel = nil; s.mu.Unlock() }()
		args := []string{"-s", filepath.Join(cfg.ManagedRoot, "main.py"), "--listen", "127.0.0.1", "--port", fmt.Sprint(port), "--disable-auto-launch", "--extra-model-paths-config", ypath}
		if cfg.Device == "cpu" {
			args = append(args, "--cpu")
		}
		cmd := nativeCommand(ctx, cfg.ManagedPython, args...)
		cmd.Dir = cfg.ManagedRoot
		cmd.Stdout = log
		cmd.Stderr = log
		err := cmd.Run()
		if err != nil && ctx.Err() == nil && cfg.Device != "cpu" {
			_, _ = log.WriteString("\nORIGIN: accelerator launch failed; retrying this portable runtime in CPU mode.\n")
			fallback := nativeCommand(ctx, cfg.ManagedPython, append(args, "--cpu")...)
			fallback.Dir = cfg.ManagedRoot
			fallback.Stdout = log
			fallback.Stderr = log
			err = fallback.Run()
		}
		if ctx.Err() == nil {
			s.mu.Lock()
			s.install.Status = "failed"
			s.install.Message = fmt.Sprintf("ComfyUI exited (%v). Open %s for the exact startup error.", err, logpath)
			s.mu.Unlock()
		}
	}()
	return nil
}
