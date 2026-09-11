package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type sourceTestTransport func(*http.Request) (*http.Response, error)

func (f sourceTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func sourceResponse(r *http.Request, value any) *http.Response {
	b, _ := json.Marshal(value)
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(b))), Header: http.Header{"Content-Type": []string{"application/json"}}, Request: r}
}
func TestProviderImportDownloadsVerifiedBytesAndSurvivesRestart(t *testing.T) {
	// Tiny protocol fixture: checks source identity, credentials, persistence and setup, not neural quality.
	home := t.TempDir()
	e := NewEngine(home)
	s := e.mediaStudio
	payload := []byte("test checkpoint fixture")
	h := sha256.Sum256(payload)
	hash := hex.EncodeToString(h[:])
	revision := strings.Repeat("a", 40)
	transport := sourceTestTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "huggingface.co" {
			if r.Header.Get("Authorization") != "Bearer hf-fixture" {
				t.Error("provider credential missing")
			}
			if strings.Contains(r.URL.Path, "/resolve/") {
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(payload))), Header: http.Header{}, Request: r}, nil
			}
			return sourceResponse(r, map[string]any{"id": "test/checkpoint", "sha": revision, "cardData": map[string]any{"license": "apache-2.0", "base_model": "stable-diffusion-v1-5/stable-diffusion-v1-5"}, "siblings": []any{map[string]any{"rfilename": "model.safetensors", "lfs": map[string]any{"sha256": hash, "size": len(payload)}}}}), nil
		}
		return nil, fmt.Errorf("unexpected fixture host %s", r.URL.Host)
	})
	s.client = &http.Client{Transport: transport}
	if err := s.saveProviderKeys(ProviderKeys{HF: "hf-fixture"}); err != nil {
		t.Fatal(err)
	}
	m, err := s.registerSource(context.Background(), SourceQuery{Provider: "huggingface", ID: "test/checkpoint", Revision: revision, File: "model.safetensors", Profile: "sd15", Role: "checkpoints"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Recipe != "checkpoint" || m.Width != 512 || !strings.HasPrefix(m.Files[0].Name, hash[:12]) {
		t.Fatal("recipe or collision protection missing")
	}
	if err = s.installModel(m.ID); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(4 * time.Second)
	for !s.modelInstalled(m) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !s.modelInstalled(m) {
		t.Fatal("verified import never became installed", s.state())
	}
	got, err := os.ReadFile(s.modelFile(m.Files[0]))
	if err != nil || string(got) != string(payload) {
		t.Fatal("download changed")
	}
	d, _ := json.Marshal(s.studioDiagnostics())
	state, _ := json.Marshal(s.state())
	if strings.Contains(string(d)+string(state), "hf-fixture") {
		t.Fatal("credential leaked")
	}
	e.Stop()
	again := NewEngine(home)
	defer again.Stop()
	restored, err := again.mediaStudio.model(m.ID)
	if err != nil || !again.mediaStudio.modelInstalled(restored) {
		t.Fatal("import lost after restart", err)
	}
}
func TestProviderCredentialsNeverFollowCDNRedirects(t *testing.T) {
	seen := map[string]string{}
	next := sourceTestTransport(func(r *http.Request) (*http.Response, error) {
		seen[r.URL.Host] = r.Header.Get("Authorization")
		return sourceResponse(r, map[string]bool{"ok": true}), nil
	})
	tr := providerTransport{next, ProviderKeys{HF: "secret-hf", Civitai: "secret-civ"}}
	for _, host := range []string{"huggingface.co", "civitai.com", "cdn-lfs.hf.co", "example.org"} {
		r, _ := http.NewRequest("GET", "https://"+host+"/file", nil)
		r.Header.Set("Authorization", "Bearer inherited-secret")
		res, err := tr.RoundTrip(r)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
	}
	if seen["huggingface.co"] != "Bearer secret-hf" || seen["civitai.com"] != "Bearer secret-civ" || seen["cdn-lfs.hf.co"] != "" || seen["example.org"] != "" {
		t.Fatal(seen)
	}
}
func TestCivitaiVersionsAndModelTermsRetained(t *testing.T) {
	e := NewEngine(t.TempDir())
	defer e.Stop()
	s := e.mediaStudio
	s.client = &http.Client{Transport: sourceTestTransport(func(r *http.Request) (*http.Response, error) {
		return sourceResponse(r, map[string]any{"id": 42, "name": "Architecture fixture", "type": "Checkpoint", "modelVersions": []any{map[string]any{"id": 9, "name": "Version nine", "baseModel": "SDXL 1.0", "files": []any{map[string]any{"name": "architecture.safetensors", "sizeKB": 1.125, "downloadUrl": "https://civitai.com/api/download/models/9", "hashes": map[string]any{"SHA256": strings.Repeat("B", 64)}}}}}}), nil
	})}
	d, err := s.inspectSource(context.Background(), SourceQuery{Provider: "civitai", ID: "42"})
	if err != nil || len(d.Files) != 1 {
		t.Fatal(d, err)
	}
	f := d.Files[0]
	if f.Version != "9" || f.Profile != "sdxl" || f.Spec.Size != 1152 || f.Spec.Source != "https://civitai.com/models/42" {
		t.Fatal(f)
	}
	if _, err = s.registerSource(context.Background(), SourceQuery{Provider: "civitai", ID: "42", Revision: "10", File: f.Name}); err == nil {
		t.Fatal("changed revision accepted")
	}
}
func TestMatrixLinkKeepsExistingFilesAndAddsSharedPaths(t *testing.T) {
	e := NewEngine(t.TempDir())
	defer e.Stop()
	s := e.mediaStudio
	root := t.TempDir()
	models := filepath.Join(root, "custom models")
	os.MkdirAll(models, 0700)
	original := fmt.Sprintf(`{"ModelDirectoryOverride":%q,"InstalledPackages":[{"Id":"fixture-id","PackageName":"ComfyUI","DisplayName":"My ComfyUI","LibraryPath":"Packages/ComfyUI"}]}`, models)
	os.WriteFile(filepath.Join(root, "settings.json"), []byte(original), 0600)
	if err := s.linkMatrix(root, ""); err != nil {
		t.Fatal(err)
	}
	yaml := s.extraModelPaths()
	if !strings.Contains(yaml, "stability_matrix:") || !strings.Contains(yaml, filepath.ToSlash(models)) || !strings.Contains(yaml, "checkpoints: StableDiffusion") {
		t.Fatal(yaml)
	}
	got, _ := os.ReadFile(filepath.Join(root, "settings.json"))
	if string(got) != original {
		t.Fatal("Matrix settings overwritten")
	}
	state, _ := json.Marshal(s.matrixState())
	if !strings.Contains(string(state), "My ComfyUI") {
		t.Fatal(string(state))
	}
}
func TestPreparationChecksActualEngineModelAvailability(t *testing.T) {
	e := NewEngine(t.TempDir())
	defer e.Stop()
	s := e.mediaStudio
	name := "my-shared-model.safetensors"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/system_stats":
			jsonReply(w, map[string]any{"devices": []any{map[string]any{"name": "fixture CPU"}}})
		case "/object_info", "/object_info/CheckpointLoaderSimple":
			jsonReply(w, map[string]any{"CheckpointLoaderSimple": map[string]any{"input": map[string]any{"required": map[string]any{"ckpt_name": []any{[]string{name}}}}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	s.mu.Lock()
	s.config.ComfyURL = server.URL
	s.mu.Unlock()
	m, err := s.registerCheckpoint(context.Background(), name, "sdxl")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.prepareModel(m.ID); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		p := s.preparation
		s.mu.Unlock()
		if p.Status == "ready" {
			break
		}
		if p.Status == "failed" {
			t.Fatal(p)
		}
		time.Sleep(10 * time.Millisecond)
	}
	s.mu.Lock()
	p := s.preparation
	s.mu.Unlock()
	if p.Status != "ready" {
		t.Fatal(p)
	}
	if _, err = s.registerCheckpoint(context.Background(), "missing.safetensors", "sdxl"); err == nil {
		t.Fatal("unlisted model accepted")
	}
}
func TestSourceRoutesValidateInputAndPrivacy(t *testing.T) {
	e := NewEngine(t.TempDir())
	defer e.Stop()
	for _, p := range []string{"prepare", "readiness", "matrix", "matrix/install", "matrix/open", "model-paths", "diagnostics", "source/search", "source/inspect", "source/import", "source/keys", "checkpoint"} {
		r := httptest.NewRequest("POST", "http://localhost/api/studio/"+p, strings.NewReader(`{"unexpected":true}`))
		r.Header.Set("X-Origin-Key", e.sessionKey)
		w := httptest.NewRecorder()
		e.handler().ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatalf("%s allowed invalid control request: %d", p, w.Code)
		}
		r = httptest.NewRequest("POST", "http://localhost/api/studio/"+p, strings.NewReader(`{}`))
		w = httptest.NewRecorder()
		e.handler().ServeHTTP(w, r)
		if w.Code == 200 {
			t.Fatalf("%s allowed unauthenticated access", p)
		}
	}
}
