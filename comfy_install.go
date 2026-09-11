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
func extractComfyArchive(ctx context.Context, extractor, archive, dest string) error {
	// Both the standalone extractor and archive are pinned and verified first.
	// Windows ships different tar builds, some without 7z support.
	listing, err := nativeCommand(ctx, extractor, "l", "-slt", "-ba", "-sccUTF-8", archive).CombinedOutput()
	if err != nil {
		return fmt.Errorf("7-Zip could not inspect the verified portable archive: %w: %s", err, shortText(string(listing), 2500))
	}
	if err := validateComfyListing(string(listing)); err != nil {
		return err
	}
	cmd := nativeCommand(ctx, extractor, "x", "-y", "-bsp0", "-bso0", "-bse1", "-o"+dest, archive)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("portable extraction failed: %w: %s", err, shortText(string(out), 2000))
	}
	return nil
}

func validateComfyListing(listing string) error {
	listing = strings.TrimPrefix(listing, "\ufeff")
	listing = strings.ReplaceAll(listing, "\r\n", "\n")
	if _, entries, ok := strings.Cut(listing, "\n----------\n"); ok {
		listing = entries
	}
	count := 0
	for _, line := range strings.Split(listing, "\n") {
		if name, ok := strings.CutPrefix(line, "Path = "); ok {
			clean := filepath.Clean(filepath.FromSlash(strings.ReplaceAll(name, "\\", "/")))
			if name == "" || !filepath.IsLocal(clean) || strings.Contains(clean, ":") {
				return errors.New("unsafe archive path")
			}
			count++
		}
		for _, prefix := range []string{"Symbolic Link = ", "Hard Link = "} {
			if target, ok := strings.CutPrefix(line, prefix); ok && strings.TrimSpace(target) != "" {
				return errors.New("portable archive links are not supported")
			}
		}
	}
	if count == 0 {
		return errors.New("7-Zip returned no recognised portable file entries")
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
	var spec, helper DownloadSpec
	for _, v := range specs {
		if v.Role == device {
			spec = v
		}
		if v.Role == "extractor" {
			helper = v
		}
	}
	if spec.URL == "" || helper.URL == "" {
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
		extractor := filepath.Join(s.root(), "tools", "7zr-26.03.exe")
		progress := func(msg string, n, t int64) {
			s.mu.Lock()
			s.install.Message = msg
			s.install.Done = n
			s.install.Total = t
			s.mu.Unlock()
		}
		err := downloadPinned(ctx, s.client, helper, extractor, progress)
		if err == nil {
			err = downloadPinned(ctx, s.client, spec, archive, progress)
		}
		staging := ""
		if err == nil {
			staging, err = os.MkdirTemp(s.root(), "comfy-staging-")
		}
		if err == nil {
			s.mu.Lock()
			s.install.Status = "extracting"
			s.install.Message = "Checksum verified. Extracting the portable environment; this can take several minutes."
			s.mu.Unlock()
			err = extractComfyArchive(ctx, extractor, archive, staging)
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
	yaml := s.extraModelPaths()
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
