package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStudioCatalogueAndRecipes(t *testing.T) {
	c := studioCatalog()
	if len(c) != 7 {
		t.Fatalf("expected seven complete packs, got %d", len(c))
	}
	if err := validateStudioCatalogue(c); err != nil {
		t.Fatal(err)
	}
	for _, m := range c {
		if m.Engine == "native" {
			continue
		}
		q := StudioRequest{Model: m.ID, Prompt: "A timber pavilion in daylight", Width: m.Width, Height: m.Width, Steps: m.Steps, Guidance: &m.Guidance, Seed: 0}
		g, err := studioGraph(q, m, "")
		if err != nil {
			t.Fatal(m.ID, err)
		}
		b, _ := json.Marshal(g)
		if !bytes.Contains(b, []byte(q.Prompt)) || !bytes.Contains(b, []byte("SaveImage")) {
			t.Fatal("recipe lost the prompt or output")
		}
		if m.Recipe == "flux2" {
			if !bytes.Contains(b, []byte("Flux2Scheduler")) || !bytes.Contains(b, []byte("EmptyFlux2LatentImage")) {
				t.Fatal("incorrect FLUX.2 graph")
			}
		} else if !bytes.Contains(b, []byte(`"seed":0`)) {
			t.Fatal("seed zero was changed")
		}
		q.Width = 777
		if _, err = studioGraph(q, m, ""); err == nil {
			t.Fatal("unaligned resolution accepted")
		}
	}
}

func TestStudioControlBodiesValidatedBeforeSideEffects(t *testing.T) {
	e := NewEngine(t.TempDir())
	defer e.Stop()
	for _, route := range []string{"state", "comfy", "comfy-setup", "comfy-start", "stop-download"} {
		r := httptest.NewRequest("POST", "http://localhost/api/studio/"+route, strings.NewReader(`{"unexpected":true}`))
		r.Header.Set("X-Origin-Key", e.sessionKey)
		w := httptest.NewRecorder()
		e.handler().ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "unknown field") {
			t.Fatalf("%s did not validate its complete request before responding: %d %s", route, w.Code, w.Body.String())
		}
	}
}

func TestComfyArchiveEntriesStayInsideStaging(t *testing.T) {
	valid := "Path = ComfyUI_windows_portable\\ComfyUI\\main.py\r\nSize = 20\r\n\r\nPath = ComfyUI_windows_portable\\python_embeded\\python.exe\r\nSize = 100\r\n"
	if err := validateComfyListing(valid); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		"Path = ..\\outside.exe\n",
		"Path = C:\\outside.exe\n",
		"Path = /outside.exe\n",
		"Path = safe.txt:stream\n",
		"Path = safe\nSymbolic Link = ..\\outside\n",
		"Path = safe\nHard Link = ..\\outside\n",
		"No entries were listed",
	} {
		if err := validateComfyListing(bad); err == nil {
			t.Fatalf("unsafe or unrecognised listing accepted: %q", bad)
		}
	}
}
func TestComfyWorkflowOutputAndErrorPersistence(t *testing.T) {
	// This is an HTTP protocol fixture, not evidence of neural image quality.
	var im bytes.Buffer
	_ = png.Encode(&im, image.NewRGBA(image.Rect(0, 0, 16, 16)))
	var mu sync.Mutex
	accepted := false
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/prompt":
			var q map[string]any
			if json.NewDecoder(r.Body).Decode(&q) != nil || q["prompt"] == nil {
				t.Error("workflow missing")
			}
			mu.Lock()
			accepted = true
			mu.Unlock()
			jsonReply(w, map[string]any{"prompt_id": "fixture-output"})
		case "/history/fixture-output":
			jsonReply(w, map[string]any{"fixture-output": map[string]any{"status": map[string]any{"completed": true, "status_str": "success"}, "outputs": map[string]any{"7": map[string]any{"images": []any{map[string]any{"filename": "result.png", "type": "output", "subfolder": ""}}}}}})
		case "/view":
			if r.URL.Query().Get("filename") != "result.png" {
				t.Error("output filename lost")
			}
			w.Header().Set("Content-Type", "image/png")
			w.Write(im.Bytes())
		default:
			http.NotFound(w, r)
		}
	}))
	defer mock.Close()
	e := NewEngine(t.TempDir())
	defer e.Stop()
	s := e.mediaStudio
	s.config.ComfyURL = mock.URL
	graph := map[string]any{"7": node("SaveImage", map[string]any{"images": link("6", 0), "filename_prefix": "test"})}
	j, err := s.submitGraph(StudioRequest{Prompt: "Protocol fixture"}, graph)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		copy := *s.jobs[0]
		s.mu.Unlock()
		if copy.Status == "completed" {
			if len(copy.Assets) != 1 {
				t.Fatal("output not saved")
			}
			path, err := e.objectPath(copy.Assets[0].ID)
			if err != nil || path == "" {
				t.Fatal("asset is not recoverable")
			}
			mu.Lock()
			ok := accepted
			mu.Unlock()
			if !ok {
				t.Fatal("workflow never reached ComfyUI")
			}
			return
		}
		if copy.Status == "failed" {
			t.Fatal(copy.Message)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("job did not finish", j.ID)
}
func TestStudioBrandingPersistenceAndBoundaries(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)
	s := e.mediaStudio
	s.config.Brand = "Example Architecture"
	s.config.Accent = "#55aa99"
	s.save()
	e.Stop()
	other := NewEngine(dir)
	defer other.Stop()
	if other.mediaStudio.config.Brand != "Example Architecture" {
		t.Fatal("brand did not survive restart")
	}
	for _, raw := range []string{"https://example.com", "http://127.0.0.1:8188/path", "http://user:secret@localhost:8188", "http://10.0.0.1:8188"} {
		if validComfyURL(raw) == nil {
			t.Fatal("accepted unsupported address", raw)
		}
	}
	req := httptest.NewRequest("GET", "http://localhost/studio/ws?clientId=unknown", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Origin", "http://external.example")
	w := httptest.NewRecorder()
	other.handler().ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatal("cross-origin live access accepted")
	}
	if _, err := other.mediaStudio.submitGraph(StudioRequest{}, map[string]any{"nodes": []any{}}); err == nil {
		t.Fatal("visual JSON accepted as API workflow")
	}
}
func TestComfyRejectsInvalidWorkflowWithoutFalseCompletion(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"missing model","node_errors":{"1":"checkpoint absent"}}`, 400)
	}))
	defer mock.Close()
	e := NewEngine(t.TempDir())
	defer e.Stop()
	s := e.mediaStudio
	s.config.ComfyURL = mock.URL
	_, err := s.submitGraph(StudioRequest{}, map[string]any{"1": node("MissingNode", map[string]any{})})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			t.Fatal("failure was not recorded")
		default:
			s.mu.Lock()
			j := *s.jobs[0]
			s.mu.Unlock()
			if j.Status == "failed" {
				if !strings.Contains(j.Message, "missing model") || len(j.Assets) != 0 {
					t.Fatal("failure was misrepresented")
				}
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}
