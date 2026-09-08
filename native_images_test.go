package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// A subprocess fixture exercises queue/cancel/transport contracts. It is not a
// diffusion model; real native model generation has a separate integration gate.
func init() {
	if os.Getenv("ORIGIN0_TEST_IMAGE_PROCESS") != "1" {
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--list-devices" {
		println("CPU test fixture")
		os.Exit(0)
	}
	args := map[string]string{}
	for i := 1; i+1 < len(os.Args); i++ {
		if strings.HasPrefix(os.Args[i], "--") {
			args[os.Args[i]] = os.Args[i+1]
		}
	}
	w, _ := strconv.Atoi(args["--width"])
	h, _ := strconv.Atoi(args["--height"])
	if w < 1 || h < 1 || args["--output"] == "" {
		os.Exit(2)
	}
	if args["--prompt"] == "slow fixture" {
		time.Sleep(10 * time.Second)
	}
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			im.SetRGBA(x, y, color.RGBA{uint8(x), uint8(y), 80, 255})
		}
	}
	f, err := os.Create(args["--output"])
	if err != nil {
		os.Exit(3)
	}
	err = png.Encode(f, im)
	f.Close()
	if err != nil {
		os.Exit(4)
	}
	os.Exit(0)
}
func tinySpec(b []byte) DownloadSpec {
	h := sha256.Sum256(b)
	return DownloadSpec{Name: "fixture.bin", Size: int64(len(b)), SHA256: hex.EncodeToString(h[:])}
}
func waitImage(t *testing.T, s *NativeImages, id, status string) {
	t.Helper()
	until := time.Now().Add(12 * time.Second)
	for time.Now().Before(until) {
		s.mu.Lock()
		var found *ImageJob
		for _, j := range s.jobs {
			if j.ID == id {
				x := *j
				found = &x
			}
		}
		s.mu.Unlock()
		if found != nil {
			if found.Status == status {
				return
			}
			if found.Status == "failed" {
				t.Fatal(found.Message, found.Log)
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("job did not reach", status)
}
func fakeImages(t *testing.T) *Engine {
	t.Helper()
	t.Setenv("ORIGIN0_TEST_IMAGE_PROCESS", "1")
	e := NewEngine(t.TempDir())
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	e.images.cli = exe
	e.images.ready = true
	e.images.config.Backend = "cpu"
	t.Cleanup(e.Stop)
	return e
}
func testImageRequest() ImageRequest {
	return ImageRequest{Prompt: "transport test fixture", Width: 256, Height: 256, Steps: 1, Seed: 42}
}

func TestPinnedDownloadResumesAndRejectsCorruption(t *testing.T) {
	data := bytes.Repeat([]byte("model-fixture"), 1000)
	spec := tinySpec(data)
	sawRange := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") == "bytes=40-" {
			sawRange = true
			w.Header().Set("Content-Range", "bytes 40-"+strconv.Itoa(len(data)-1)+"/"+strconv.Itoa(len(data)))
			w.WriteHeader(206)
			w.Write(data[40:])
		} else {
			w.Write(data)
		}
	}))
	defer server.Close()
	spec.URL = server.URL
	dest := filepath.Join(t.TempDir(), "model.bin")
	os.WriteFile(dest+".part", data[:40], 0600)
	progress := func(string, int64, int64) {}
	if err := downloadPinned(context.Background(), server.Client(), spec, dest, progress); err != nil || !sawRange {
		t.Fatal("resume failed", err, sawRange)
	}
	wrong := data
	wrong[0] ^= 1
	os.WriteFile(dest, wrong, 0600)
	if err := checkDownload(context.Background(), dest, spec); err == nil {
		t.Fatal("accepted corrupted full file")
	}
}
func TestResumeWhenServerIgnoresRange(t *testing.T) {
	data := []byte("full model payload")
	spec := tinySpec(data)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(data) }))
	defer srv.Close()
	spec.URL = srv.URL
	dest := filepath.Join(t.TempDir(), "m")
	os.WriteFile(dest+".part", data[:3], 0600)
	if err := downloadPinned(context.Background(), srv.Client(), spec, dest, func(string, int64, int64) {}); err != nil {
		t.Fatal(err)
	}
}
func TestDownloadCancelKeepsPartialAndRejectsWrongRange(t *testing.T) {
	data := bytes.Repeat([]byte("x"), 4096)
	spec := tinySpec(data)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Range", "bytes 2-4095/4096")
		w.WriteHeader(206)
		w.Write(data[2:])
	}))
	defer srv.Close()
	spec.URL = srv.URL
	dest := filepath.Join(t.TempDir(), "m")
	os.WriteFile(dest+".part", data[:20], 0600)
	if err := downloadPinned(context.Background(), srv.Client(), spec, dest, func(string, int64, int64) {}); err == nil {
		t.Fatal("accepted wrong range")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := downloadPinned(ctx, srv.Client(), spec, dest, func(string, int64, int64) {}); err == nil {
		t.Fatal("ignored cancellation")
	}
	if st, err := os.Stat(dest + ".part"); err != nil || st.Size() != 20 {
		t.Fatal("partial was lost")
	}
}
func TestRuntimeExtractionRejectsPathEscape(t *testing.T) {
	for _, name := range []string{"../escape.exe", "C:/escape.exe", `..\escape.exe`} {
		var buf bytes.Buffer
		z := zip.NewWriter(&buf)
		f, _ := z.Create(name)
		f.Write([]byte("bad"))
		z.Close()
		root := t.TempDir()
		p := filepath.Join(root, "r.zip")
		os.WriteFile(p, buf.Bytes(), 0600)
		if err := extractRuntime(p, filepath.Join(root, "runtime")); err == nil {
			t.Fatal("accepted", name)
		}
	}
}
func TestImageJobProducesAssetAndSurvivesRestart(t *testing.T) {
	e := fakeImages(t)
	req := testImageRequest()
	req.Prompt = "a prompt with literal ; and $(echo text)"
	j, err := e.images.submit(req)
	if err != nil {
		t.Fatal(err)
	}
	waitImage(t, e.images, j.ID, "completed")
	e.images.save()
	n := NewEngine(filepath.Dir(e.dataDir))
	defer n.Stop()
	if len(n.images.jobs) != 1 || n.images.jobs[0].Asset == nil {
		t.Fatal("completed asset lost")
	}
	w := call(e, "POST", "/api/images/diagnostics", map[string]any{})
	if w.Code != 200 || strings.Contains(w.Body.String(), req.Prompt) || strings.Contains(w.Body.String(), e.sessionKey) {
		t.Fatal("diagnostics leaked a prompt or key")
	}
}
func TestCancelNativeProcessAndQueuedJob(t *testing.T) {
	e := fakeImages(t)
	req := testImageRequest()
	req.Prompt = "slow fixture"
	j, err := e.images.submit(req)
	if err != nil {
		t.Fatal(err)
	}
	other, err := e.images.submit(testImageRequest())
	if err != nil {
		t.Fatal(err)
	}
	if err = e.images.cancelJob(other.ID); err != nil {
		t.Fatal(err)
	}
	if err = e.images.cancelJob(j.ID); err != nil {
		t.Fatal(err)
	}
	e.images.wg.Wait()
	e.images.mu.Lock()
	defer e.images.mu.Unlock()
	for _, j := range e.images.jobs {
		if j.Status != "cancelled" || j.Asset != nil {
			t.Fatal("cancellation accepted an image", j)
		}
	}
}
func TestPoolAuthenticatesLeasesAndRejectsLateResults(t *testing.T) {
	e := NewEngine(t.TempDir())
	defer e.Stop()
	p := e.images.pool
	if err := p.host("127.0.0.1:0", ""); err != nil {
		t.Fatal(err)
	}
	raw, err := p.invite()
	if err != nil {
		t.Fatal(err)
	}
	inv, err := parsePoolInvite(raw)
	if err != nil {
		t.Fatal(err)
	}
	client := poolClient(inv)
	defer client.CloseIdleConnections()
	req := testImageRequest()
	req.Shared = true
	j, err := e.images.submit(req)
	if err != nil {
		t.Fatal(err)
	}
	b, err := poolPost(context.Background(), client, inv, "/pool/lease", map[string]string{"name": "fixture worker", "pack": e.images.catalog.ID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var lease struct {
		Job   *ImageJob `json:"job"`
		Lease string    `json:"lease"`
	}
	if json.Unmarshal(b, &lease) != nil || lease.Job == nil || lease.Job.ID != j.ID {
		t.Fatal("job was not assigned")
	}
	if err = e.images.cancelJob(j.ID); err != nil {
		t.Fatal(err)
	}
	_, err = poolPost(context.Background(), client, inv, "/pool/result", []byte("not an image"), map[string]string{"X-Job-ID": j.ID, "X-Job-Lease": lease.Lease})
	if err == nil {
		t.Fatal("cancelled lease accepted a result")
	}
	bad := inv
	bad.Token = "wrong-token"
	if _, err = poolPost(context.Background(), client, bad, "/pool/lease", map[string]string{}, nil); err == nil {
		t.Fatal("unauthorized worker admitted")
	}
	bad = inv
	bad.Pin = strings.Repeat("0", 64)
	badClient := poolClient(bad)
	defer badClient.CloseIdleConnections()
	if _, err = poolPost(context.Background(), badClient, bad, "/pool/lease", map[string]string{}, nil); err == nil {
		t.Fatal("bad TLS fingerprint admitted")
	}
}
func TestVolunteerWorkerReturnsImageWithFiniteBudget(t *testing.T) {
	worker := fakeImages(t)
	leader := NewEngine(t.TempDir())
	defer leader.Stop()
	p := leader.images.pool
	if err := p.host("127.0.0.1:0", ""); err != nil {
		t.Fatal(err)
	}
	invite, err := p.invite()
	if err != nil {
		t.Fatal(err)
	}
	req := testImageRequest()
	req.Shared = true
	j, err := leader.images.submit(req)
	if err != nil {
		t.Fatal(err)
	}
	if err = worker.images.pool.join(invite, "contract worker", 1); err != nil {
		t.Fatal(err)
	}
	waitImage(t, leader.images, j.ID, "completed")
	worker.images.pool.wg.Wait()
	worker.images.pool.mu.Lock()
	done := worker.images.pool.joinCompleted
	joined := worker.images.pool.joinCancel != nil
	worker.images.pool.mu.Unlock()
	if done != 1 || joined {
		t.Fatal("worker exceeded or failed its donation budget")
	}
}
func TestExpiredWorkerLeaseIsReassignedOnce(t *testing.T) {
	e := NewEngine(t.TempDir())
	defer e.Stop()
	r := testImageRequest()
	r.Shared = true
	j, err := e.images.submit(r)
	if err != nil {
		t.Fatal(err)
	}
	e.images.mu.Lock()
	defer e.images.mu.Unlock()
	x := e.images.jobs[0]
	x.Status = "running"
	x.ID = j.ID
	x.Lease = "old"
	x.LeaseUntil = 10
	x.Attempts = 1
	e.images.pool.expireLocked(20)
	if x.Status != "queued" || x.Lease != "" {
		t.Fatal("expired job not requeued")
	}
	x.Status = "running"
	x.Attempts = 2
	e.images.pool.expireLocked(20)
	if x.Status != "failed" {
		t.Fatal("unbounded retries")
	}
}
func TestImageLimitsAndCorruptPNGs(t *testing.T) {
	r := testImageRequest()
	r.Width = 1000000
	if validateImageRequest(r) == nil {
		t.Fatal("accepted unbounded image")
	}
	r = testImageRequest()
	if validatePNG([]byte("fake.png"), r) == nil {
		t.Fatal("accepted corrupt PNG")
	}
}
