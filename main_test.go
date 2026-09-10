package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBinaryCodePrefixFree(t *testing.T) {
	codes := []string{}
	for i := 1; i < 80; i++ {
		codes = append(codes, binaryCode(i))
	}
	for i, a := range codes {
		for j, b := range codes {
			if i != j && strings.HasPrefix(b, a) {
				t.Fatalf("not prefix free: %s prefixes %s", a, b)
			}
		}
	}
}

func TestSelfIngestionProducesStructure(t *testing.T) {
	d := t.TempDir()
	e := NewEngine(d)
	e.ingestBytes("self-test", []byte(strings.Repeat("alpha beta gamma alpha beta delta ", 40)), 1200, "test")
	e.mu.RLock()
	defer e.mu.RUnlock()
	if len(e.concepts) == 0 {
		t.Fatal("no concepts")
	}
	if len(e.relations) == 0 {
		t.Fatal("no relations")
	}
	if len(e.hypotheses) == 0 {
		t.Fatal("no hypotheses")
	}
	if len(e.questions) == 0 {
		t.Fatal("no questions")
	}
}

func TestSwarmExtinctionRebirthBounded(t *testing.T) {
	e := NewEngine(t.TempDir())
	e.mu.Lock()
	e.swarms[0].Alive = 1
	e.swarms[0].Population = 64
	e.mu.Unlock()
	e.stepSwarms()
	e.mu.RLock()
	s := e.swarms[0]
	defer e.mu.RUnlock()
	if s.Alive > s.Population {
		t.Fatal("population escaped bound")
	}
	if s.Extinctions < 1 {
		t.Fatal("expected simulated extinction/rebirth")
	}
	if s.AncestralScar == "" {
		t.Fatal("missing ancestral scar")
	}
}

func TestUICandidates(t *testing.T) {
	e := NewEngine(t.TempDir())
	e.generateUICandidates()
	e.mu.RLock()
	defer e.mu.RUnlock()
	if len(e.uiCandidates) != 4 {
		t.Fatalf("got %d candidates", len(e.uiCandidates))
	}
	for _, g := range e.uiCandidates {
		if g.Topology == "" {
			t.Fatal("missing topology")
		}
	}
}

func TestHTTPState(t *testing.T) {
	e := NewEngine(t.TempDir())
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "http://127.0.0.1:8765/api/state", nil)
	req.Header.Set("X-Origin-Key", e.sessionKey)
	e.handler().ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("code %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "1.5-native-images") {
		t.Fatal("state missing version")
	}
}

func TestDropFolderExists(t *testing.T) {
	e := NewEngine(t.TempDir())
	if st, err := os.Stat(e.dropDir); err != nil || !st.IsDir() {
		t.Fatal("drop folder not created")
	}
	if _, err := os.Stat(filepath.Join(e.dataDir, "proposals")); err != nil {
		t.Fatal("proposal folder not created")
	}
}

func TestWorkersAdvance(t *testing.T) {
	e := NewEngine(t.TempDir())
	e.launch(func() { e.computeWorker(0, 1) })
	defer e.Stop()
	time.Sleep(80 * time.Millisecond)
	if e.cycle.Load() == 0 {
		t.Fatal("worker did not advance")
	}
}
