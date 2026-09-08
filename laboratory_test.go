package main

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func call(e *Engine, method, path string, v any) *httptest.ResponseRecorder {
	var b io.Reader
	if v != nil {
		data, _ := json.Marshal(v)
		b = bytes.NewReader(data)
	}
	r := httptest.NewRequest(method, "http://127.0.0.1:8765"+path, b)
	r.Header.Set("X-Origin-Key", e.sessionKey)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.handler().ServeHTTP(w, r)
	return w
}
func passing(id string) Trial {
	t := Trial{ID: id}
	for _, w := range []int{280, 320, 390, 768, 1440, 2560} {
		t.Checks = append(t.Checks, LayoutCheck{Width: w, Controls: 30})
	}
	t.Pass = checkTrial(t)
	return t
}
func TestCandidatesCannotMutateActiveLayout(t *testing.T) {
	e := NewEngine(t.TempDir())
	before := cloneUI(e.ui)
	for i := 0; i < 20; i++ {
		_ = e.mutateUI(e.ui)
	}
	a, _ := json.Marshal(before)
	b, _ := json.Marshal(e.ui)
	if !bytes.Equal(a, b) {
		t.Fatal("candidate mutated active layout")
	}
}
func TestSnapshotRestoreMergeJoinAndChecksum(t *testing.T) {
	e := NewEngine(t.TempDir())
	e.ingestBytes("a", []byte(strings.Repeat("abcdefghabcdefghi", 20)), 340, "test")
	s, err := e.checkpoint("before")
	if err != nil {
		t.Fatal(err)
	}
	n := len(e.concepts)
	e.ingestBytes("b", []byte(strings.Repeat("zzzzxxxxccccvvvv", 20)), 320, "test")
	if err = e.snapshotAction(s.ID, "restore", ""); err != nil {
		t.Fatal(err)
	}
	if len(e.concepts) != n {
		t.Fatal("state not restored")
	}
	if err = e.snapshotAction(s.ID, "join", ""); err != nil {
		t.Fatal(err)
	}
	if len(e.swarms) != 7 {
		t.Fatal("archived descendant missing")
	}
	if err = e.snapshotAction(s.ID, "branch", "experiment"); err != nil {
		t.Fatal(err)
	}
	if e.lab.Branch != "experiment" {
		t.Fatal("branch name")
	}
	before := e.concepts
	counts := map[string]int{}
	for k, v := range before {
		counts[k] = v.Count
	}
	if err = e.snapshotAction(s.ID, "merge", ""); err != nil {
		t.Fatal(err)
	}
	for k, n := range counts {
		if e.concepts[k].Count != n {
			t.Fatal("replayed evidence inflated counts")
		}
	}
	_ = os.WriteFile(filepath.Join(e.dataDir, "snapshots", s.ID+".json"), []byte("tamper"), 0600)
	if _, err = e.readSnapshot(s.ID); err == nil {
		t.Fatal("checksum not enforced")
	}
}
func TestLocalAPIRejectsUnauthorizedAndCrossOrigin(t *testing.T) {
	e := NewEngine(t.TempDir())
	for _, c := range []struct {
		host, key, origin, method, path string
		status                          int
	}{{"localhost:8765", "", "", "GET", "/api/state", 401}, {"evil.example", e.sessionKey, "", "GET", "/api/state", 403}, {"localhost:8765", e.sessionKey, "https://evil.example", "POST", "/api/observe", 403}, {"localhost:8765", e.sessionKey, "", "GET", "/api/compute", 405}} {
		r := httptest.NewRequest(c.method, "http://"+c.host+c.path, strings.NewReader("{}"))
		r.Header.Set("X-Origin-Key", c.key)
		r.Header.Set("Origin", c.origin)
		w := httptest.NewRecorder()
		e.handler().ServeHTTP(w, r)
		if w.Code != c.status {
			t.Fatalf("%s got %d want %d", c.path, w.Code, c.status)
		}
	}
}
func TestModelCannotCallWithoutConnection(t *testing.T) {
	e := NewEngine(t.TempDir())
	w := call(e, "POST", "/api/model", map[string]any{"kind": "image", "prompt": "test"})
	if w.Code != 400 || !strings.Contains(w.Body.String(), "connect a model") {
		t.Fatal(w.Body.String())
	}
}
func TestProviderAdaptersWithLocalStub(t *testing.T) {
	hits := 0
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		switch r.URL.Path {
		case "/v1/images/generations":
			jsonReply(w, map[string]any{"data": []any{map[string]string{"b64_json": base64.StdEncoding.EncodeToString([]byte("image test bytes"))}}})
		case "/v1/audio/transcriptions":
			if err := r.ParseMultipartForm(2 << 20); err != nil {
				t.Error(err)
			}
			jsonReply(w, map[string]string{"text": "A supplied voice observation"})
		case "/v1/responses":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["store"] != false {
				t.Error("response storage was not disabled")
			}
			jsonReply(w, map[string]any{"output": []any{map[string]any{"content": []any{map[string]string{"type": "output_text", "text": "A test model response"}}}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer mock.Close()
	e := NewEngine(t.TempDir())
	w := call(e, "POST", "/api/provider/connect", map[string]any{"base": mock.URL + "/v1", "key": "test-secret-key", "text_model": "test", "image_model": "test", "audio_model": "test", "enabled": true, "remaining": 3})
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	for _, kind := range []string{"text", "image"} {
		w = call(e, "POST", "/api/model", map[string]any{"kind": kind, "prompt": "test"})
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
	}
	a, err := e.storeObject(strings.NewReader("test audio"), "voice.webm", "audio/webm", "test")
	if err != nil {
		t.Fatal(err)
	}
	w = call(e, "POST", "/api/model", map[string]any{"kind": "transcribe", "asset": a.ID})
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if hits != 3 {
		t.Fatal(hits)
	}
	w = call(e, "POST", "/api/model", map[string]any{"kind": "text", "prompt": "over limit"})
	if w.Code != 400 || hits != 3 {
		t.Fatal("allowance not enforced")
	}
	e.persist()
	b, _ := os.ReadFile(filepath.Join(e.dataDir, "state.json"))
	if bytes.Contains(b, []byte("test-secret-key")) {
		t.Fatal("API key persisted")
	}
	w = call(e, "GET", "/api/provider", nil)
	if strings.Contains(w.Body.String(), "test-secret-key") {
		t.Fatal("API key exposed")
	}
}
func TestProviderRejectsRemoteAndRedirects(t *testing.T) {
	for _, bad := range []string{"http://example.org/v1", "https://api.openai.com.evil/v1", "https://api.openai.com@evil.test/v1", "http://localhost:9999/v1"} {
		if _, err := providerBase(bad); err == nil {
			t.Fatal("accepted", bad)
		}
	}
}
func TestRawStorageDedupAndQuota(t *testing.T) {
	e := NewEngine(t.TempDir())
	e.lab.QuotaBytes = 12
	a, err := e.storeObject(strings.NewReader("abcdef"), "one", "text/plain", "test")
	if err != nil {
		t.Fatal(err)
	}
	b, err := e.storeObject(strings.NewReader("abcdef"), "two", "text/plain", "test")
	if err != nil || a.ID != b.ID || e.lab.StoredBytes != 6 || e.lab.ObjectCount != 1 {
		t.Fatal("dedup failed")
	}
	if _, err = e.storeObject(strings.NewReader("1234567"), "large", "text/plain", "test"); err == nil {
		t.Fatal("quota exceeded")
	}
	entries, _ := os.ReadDir(filepath.Join(e.dataDir, "objects"))
	for _, f := range entries {
		if strings.HasPrefix(f.Name(), "incoming-") {
			t.Fatal("partial object not removed")
		}
	}
}
func TestQuotaUses64BitAndSamplingBeyond8TiB(t *testing.T) {
	e := NewEngine(t.TempDir())
	w := call(e, "POST", "/api/settings", map[string]any{"quota_gb": 16384})
	if w.Code != 204 || e.lab.QuotaBytes != int64(16)<<40 {
		t.Fatal("64 bit quota failed")
	}
	p := filepath.Join(t.TempDir(), "sparse.bin")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	const n = int64(9) << 40
	if err = f.Truncate(n); err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteAt([]byte("START"), 0)
	_, _ = f.WriteAt([]byte("FINISH"), n-6)
	f.Close()
	b, size, err := sampleFile(p)
	if err != nil || size != n || len(b) > 2<<20 || !bytes.HasPrefix(b, []byte("START")) || !bytes.HasSuffix(b, []byte("FINISH")) {
		t.Fatal("large offset sample failed", size, len(b), err)
	}
}
func TestLayoutGateAndPromotionRollback(t *testing.T) {
	e := NewEngine(t.TempDir())
	e.generateUICandidates()
	g := e.uiCandidates[0]
	if err := e.promote(g.ID); err == nil {
		t.Fatal("untested candidate promoted")
	}
	tr := passing(g.ID)
	tr.Checks[0].Overflow = true
	if checkTrial(tr) {
		t.Fatal("overflow allowed")
	}
	e.lab.Trials[g.ID] = passing(g.ID)
	if err := e.promote(g.ID); err != nil {
		t.Fatal(err)
	}
	if e.ui.ID != g.ID || len(e.lab.Snapshots) != 1 {
		t.Fatal("promotion lacks restore point")
	}
}
func TestAutoPromotionRequiresMeasuredImprovementAndIdle(t *testing.T) {
	e := NewEngine(t.TempDir())
	e.generateUICandidates()
	g := e.uiCandidates[0]
	e.lab.Trials[g.ID] = passing(g.ID)
	original := e.ui.ID
	e.autoPromoteUI()
	if e.ui.ID != original {
		t.Fatal("promoted without human task data")
	}
	base := passing(original)
	base.Samples = 5
	base.TotalMS = 50000
	e.lab.Trials[original] = base
	next := passing(g.ID)
	next.Samples = 5
	next.TotalMS = 30000
	e.lab.Trials[g.ID] = next
	e.interactionUntil.Store(time.Now().Add(time.Minute).UnixMilli())
	e.autoPromoteUI()
	if e.ui.ID != original {
		t.Fatal("moved during interaction")
	}
	e.interactionUntil.Store(0)
	e.autoPromoteUI()
	if e.ui.ID != g.ID {
		t.Fatal("valid measured candidate did not promote")
	}
}
func TestInferenceUsesDepthWithoutInventingEvidence(t *testing.T) {
	e := NewEngine(t.TempDir())
	e.engineGenome.InferenceDepth = 3
	ids := []string{"motifaa", "motifbb", "motifcc", "motifdd"}
	for i := 0; i < 3; i++ {
		e.relations[fmt.Sprint(i)] = &Relation{A: ids[i], B: ids[i+1], Count: 20, Confidence: .99}
	}
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 100; i++ {
		e.reasonStep(r)
	}
	found := false
	for _, h := range e.hypotheses {
		if strings.Contains(h.Symbolic, "3 hops") {
			found = true
		}
		if h.Support != 0 {
			t.Fatal("repeated inference invented observations")
		}
	}
	if !found {
		t.Fatal("depth ignored")
	}
}
func TestSwarmDiscoveriesCorrespondToMemory(t *testing.T) {
	e := NewEngine(t.TempDir())
	e.hypotheses["h"] = &Hypothesis{ID: "h", A: "motifaa", B: "motifbb", Confidence: .5}
	for i := 0; i < 4; i++ {
		e.stepSwarms()
	}
	for _, s := range e.swarms {
		if s.Discoveries != 1 || len(s.Memory) != 1 {
			t.Fatal("fake discoveries", s.Discoveries, len(s.Memory))
		}
	}
}
func TestDynamicFieldsAndSubmissionValidation(t *testing.T) {
	e := NewEngine(t.TempDir())
	e.ingestBytes("example", []byte(`{"address":"Ettelbruck","asking_price":250000,"vacant":true}`), 70, "test")
	if len(e.lab.Discovered) != 1 || len(e.lab.Discovered[0].Fields) != 3 {
		t.Fatal("new fields not generated")
	}
	e.lab.Discovered[0].Fields[0].Required = true
	b := e.lab.Discovered[0]
	w := call(e, "POST", "/api/block", map[string]any{"id": b.ID, "values": map[string]string{}})
	if w.Code != 400 {
		t.Fatal("missing required field accepted")
	}
}
func TestCorruptLatestRecoversPrevious(t *testing.T) {
	base := t.TempDir()
	e := NewEngine(base)
	e.lab.Goal = "saved goal"
	e.persist()
	e.lab.Goal = "next goal"
	e.persist()
	_ = os.WriteFile(filepath.Join(e.dataDir, "state.json"), []byte("{invalid"), 0600)
	next := NewEngine(base)
	if next.lab.Goal != "saved goal" {
		t.Fatal("previous checkpoint not recovered")
	}
	files, _ := filepath.Glob(filepath.Join(e.dataDir, "state.corrupt-*.json"))
	if len(files) != 1 {
		t.Fatal("corrupt bytes not preserved")
	}
}
func TestPortableExportExcludesCredentialsAndRawData(t *testing.T) {
	e := NewEngine(t.TempDir())
	e.provider = Provider{Key: "secret-test", Enabled: true}
	w := call(e, "GET", "/api/export", nil)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	z, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range z.File {
		if strings.Contains(f.Name, "instance.lock") {
			t.Fatal("session auth exported")
		}
		if f.Name == "origin0_data/state.json" {
			found = true
			r, _ := f.Open()
			b, _ := io.ReadAll(r)
			r.Close()
			if bytes.Contains(b, []byte("secret-test")) {
				t.Fatal("credential in export")
			}
			var p PersistState
			_ = json.Unmarshal(b, &p)
			if len(p.Lab.Assets) > 0 || len(p.Lab.Snapshots) > 0 {
				t.Fatal("dangling export references")
			}
		}
	}
	if !found {
		t.Fatal("no runnable state")
	}
}
func TestConcurrentViewsAndPersistence(t *testing.T) {
	e := NewEngine(t.TempDir())
	e.ingestBytes("seed", []byte(strings.Repeat("abcdefghxyzabcdefgh", 10)), 180, "test")
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			r := rand.New(rand.NewSource(int64(seed)))
			for j := 0; j < 20; j++ {
				e.reasonStep(r)
				e.stepSwarms()
				v := e.view()
				if _, err := json.Marshal(v); err != nil {
					t.Error(err)
				}
				e.persist()
			}
		}(i)
	}
	wg.Wait()
}
func TestWorkingStateResumesOnNewEngine(t *testing.T) {
	base := t.TempDir()
	e := NewEngine(base)
	e.ingestBytes("seed", []byte(strings.Repeat("abcdefhijkl", 20)), 220, "test")
	e.cycle.Store(42)
	e.persist()
	n := NewEngine(base)
	if n.cycle.Load() != 42 || len(n.concepts) != len(e.concepts) {
		t.Fatal("restart lost state")
	}
}
