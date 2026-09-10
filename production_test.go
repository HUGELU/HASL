package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func finishFixture(t *testing.T, e *Engine) AssetRecord {
	t.Helper()
	im := image.NewNRGBA(image.Rect(0, 0, 24, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 24; x++ {
			im.SetNRGBA(x, y, color.NRGBA{uint8(x * 10), uint8(y * 12), 130, 255})
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, im)
	a, err := e.storeObject(&b, "finish-input.png", "image/png", "test")
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func awaitFinish(t *testing.T, s *Finishing, id string) FinishJob {
	t.Helper()
	end := time.Now().Add(15 * time.Second)
	for time.Now().Before(end) {
		s.mu.Lock()
		var j FinishJob
		for _, v := range s.jobs {
			if v.ID == id {
				j = *v
				break
			}
		}
		s.mu.Unlock()
		if j.Status != "running" && j.Status != "queued" {
			return j
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("finishing did not complete")
	return FinishJob{}
}
func TestFinishPreservesAlphaAndConstantColour(t *testing.T) {
	im := image.NewNRGBA(image.Rect(0, 0, 13, 9))
	for y := 0; y < 9; y++ {
		for x := 0; x < 13; x++ {
			im.SetNRGBA(x, y, color.NRGBA{32, 140, 220, 128})
		}
	}
	out, err := resizeFinish(context.Background(), im, 97, 67, 3)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 67; y++ {
		for x := 0; x < 97; x++ {
			if c := out.NRGBAAt(x, y); c != (color.NRGBA{32, 140, 220, 128}) {
				t.Fatalf("colour changed: %v", c)
			}
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = resizeFinish(cancelled, im, 128, 128, 2); err == nil {
		t.Fatal("cancellation ignored")
	}
	if _, _, err = finishDimensions(1, 1, 9000); err == nil {
		t.Fatal("dimension cap ignored")
	}
}
func TestFinishQueueRecipeCacheAndTamper(t *testing.T) {
	e := NewEngine(t.TempDir())
	defer e.Stop()
	a := finishFixture(t, e)
	req := FinishRequest{Asset: a.ID, FinishOptions: FinishOptions{Mode: "fast", LongEdge: 128, Backend: "cpu", Detail: .75, Sharpness: .2}}
	j, err := e.finishing.submit(req)
	if err != nil {
		t.Fatal(err)
	}
	done := awaitFinish(t, e.finishing, j.ID)
	if done.Status != "completed" || done.Asset == nil || done.Recipe == nil || done.Width != 128 || done.Height != 107 {
		t.Fatalf("bad finish: %+v", done)
	}
	path, _ := e.objectPath(done.Asset.ID)
	f, _ := os.Open(path)
	cfg, err := png.DecodeConfig(f)
	f.Close()
	if err != nil || cfg.Width != 128 || cfg.Height != 107 {
		t.Fatal("invalid output", err)
	}
	j, _ = e.finishing.submit(req)
	cached := awaitFinish(t, e.finishing, j.ID)
	if !cached.Cached || cached.Asset.ID != done.Asset.ID {
		t.Fatal("identical recipe did not use cache")
	}
	p, _ := e.objectPath(a.ID)
	if err = os.WriteFile(p, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	j, _ = e.finishing.submit(req)
	if bad := awaitFinish(t, e.finishing, j.ID); bad.Status != "failed" || !strings.Contains(bad.Message, "checksum") {
		t.Fatalf("bad source accepted: %+v", bad)
	}
}
func TestHeavyWorkCancellation(t *testing.T) {
	e := NewEngine(t.TempDir())
	defer e.Stop()
	if err := e.acquireHeavy(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := e.acquireHeavy(ctx); err == nil {
		t.Fatal("second heavy job ran concurrently")
	}
	e.releaseHeavy()
}
func TestArchitectureMeasuredPlanAndOverlap(t *testing.T) {
	p := ArchitectureProject{Name: "Home <script>", Rooms: []PlanRoom{{Name: "Living", W: 5, H: 4, Door: "bottom", DoorOffset: 1, DoorWidth: .9}, {Name: "Kitchen", X: 5, W: 3, H: 4}}}
	svg, area, err := planSVG(p)
	if err != nil || area != 32 || !strings.Contains(svg, "5.00 × 4.00") || strings.Contains(svg, "<script>") {
		t.Fatal("invalid measured plan", area, err)
	}
	p.Rooms[1].X = 4
	if _, _, err = planSVG(p); err == nil {
		t.Fatal("overlapping rooms accepted")
	}
	p.Rooms = p.Rooms[:1]
	p.Rooms[0].DoorWidth = 10
	if _, _, err = planSVG(p); err == nil {
		t.Fatal("oversized door accepted")
	}
}
func TestWalkthroughEscapesCaptionsAndKeepsPhotos(t *testing.T) {
	e := NewEngine(t.TempDir())
	defer e.Stop()
	a := finishFixture(t, e)
	p := ArchitectureProject{Name: "__FRAMES__ <script>", Views: []ArchitectureView{{Asset: a.ID, Label: "</script><script>alert(1)</script>", Seconds: 2}, {Asset: a.ID, Label: "Second", Seconds: 2}}}
	html, err := e.walkthroughHTML(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, "</script><script>alert") || !strings.Contains(html, "data:image/png;base64,") || !strings.Contains(html, "__FRAMES__ &lt;script&gt;") || !strings.Contains(html, "MediaRecorder") {
		t.Fatal("unsafe or incomplete walkthrough")
	}
	if !strings.Contains(html, `<script type="application/json" id="frames">`) || !strings.Contains(html, "<script>"+walkthroughScript+"</script>") {
		t.Fatal("walkthrough data and fixed executable player must remain separate")
	}
	response := call(e, "GET", "/api/state", nil)
	if !strings.Contains(response.Header().Get("Content-Security-Policy"), "'sha256-"+walkthroughScriptHash()+"'") {
		t.Fatal("the fixed preview player cannot run under the parent CSP")
	}
}
func TestDevelopmentProposalAndSandboxBoundary(t *testing.T) {
	p := CodeProposal{File: "adaptive_kernel.go", Content: "package main\nfunc testValue() int {return 1}\n"}
	if err := validateCodeProposal(p, p.File); err != nil {
		t.Fatal(err)
	}
	p.File = "../main.go"
	if validateCodeProposal(p, "adaptive_kernel.go") == nil {
		t.Fatal("path escape accepted")
	}
	p.File = "adaptive_kernel.go"
	p.Content = "package broken\n"
	if validateCodeProposal(p, p.File) == nil {
		t.Fatal("package change accepted")
	}
	args := strings.Join(sandboxArgs("test", "/tmp/candidate", "golang@sha256:"+strings.Repeat("a", 64)), " ")
	for _, flag := range []string{"--pull=never", "--network none", "--read-only", "--cap-drop ALL", "no-new-privileges", "--memory 2g", "--cpus 2", "dst=/src,readonly"} {
		if !strings.Contains(args, flag) {
			t.Fatal("sandbox missing", flag)
		}
	}
	if sandboxImage.MatchString("golang-evil@sha256:" + strings.Repeat("a", 64)) {
		t.Fatal("unapproved image name accepted")
	}
}
func TestLocalCodingLoopUsesActualResponse(t *testing.T) {
	e := NewEngine(t.TempDir())
	defer e.Stop()
	src, err := embeddedSource()
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v1/chat/completions" {
			t.Error("wrong local endpoint")
		}
		b, _ := json.Marshal(CodeProposal{File: "adaptive_kernel.go", Content: src["adaptive_kernel.go"], Summary: "Retain baseline until benchmark evidence supports a change", Risks: "No measured improvement"})
		jsonReply(w, map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(b)}}}})
	}))
	defer server.Close()
	e.provider = Provider{Base: server.URL + "/v1", Enabled: true, Remaining: 2, TextModel: "fixture-coder"}
	r, err := e.development.start(DevelopmentRequest{Goal: "Inspect correctness", Target: "adaptive_kernel.go", Iterations: 1})
	if err != nil {
		t.Fatal(err)
	}
	e.development.wg.Wait()
	e.development.mu.Lock()
	run := *e.development.runs[0]
	e.development.mu.Unlock()
	if calls != 1 || run.ID != r.ID || run.Status != "completed" || run.Artifact == nil || len(run.Steps) != 1 || !run.Steps[0].Syntax || run.Steps[0].Tested {
		t.Fatalf("incorrect result: %+v, calls %d", run, calls)
	}
	if _, err = os.Stat(filepath.Join(e.dataDir, "development.json")); err != nil {
		t.Fatal("missing durable history")
	}
}
func TestCandidateInRealDockerSandbox(t *testing.T) {
	imageID := os.Getenv("ORIGIN0_TEST_DOCKER_IMAGE")
	docker := os.Getenv("ORIGIN0_TEST_DOCKER")
	if imageID == "" || docker == "" {
		t.Skip("Docker integration gate runs in project CI")
	}
	src, err := embeddedSource()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	p := CodeProposal{File: "adaptive_kernel.go", Content: src["adaptive_kernel.go"]}
	log, err := testCodeCandidate(ctx, DevelopmentConfig{Docker: docker, Image: imageID}, src, p, t.TempDir())
	if err != nil {
		t.Fatalf("Docker gate failed: %v\n%s", err, log)
	}
}

// A closing HTTP client may send its body in a later packet. Replying before
// reading it can reset the Windows TCP connection and lose the response.
func TestProductionControlsConsumeRequestBeforeReply(t *testing.T) {
	e := NewEngine(t.TempDir())
	defer e.Stop()
	server := httptest.NewServer(e.handler())
	defer server.Close()
	for _, path := range []string{"/api/finish/state", "/api/architecture/project", "/api/development/state", "/api/development/cancel"} {
		t.Run(path, func(t *testing.T) {
			conn, err := net.Dial("tcp", strings.TrimPrefix(server.URL, "http://"))
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			_, err = fmt.Fprintf(conn, "POST %s HTTP/1.1\r\nHost: %s\r\nX-Origin-Key: %s\r\nContent-Type: application/json\r\nContent-Length: 2\r\nConnection: close\r\n\r\n", path, strings.TrimPrefix(server.URL, "http://"), e.sessionKey)
			if err != nil {
				t.Fatal(err)
			}
			reader := bufio.NewReader(conn)
			_ = conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
			if _, err = reader.Peek(1); err == nil {
				t.Fatal("responded before consuming POST body")
			} else if n, ok := err.(net.Error); !ok || !n.Timeout() {
				t.Fatal(err)
			}
			_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			if _, err = io.WriteString(conn, "{}"); err != nil {
				t.Fatal(err)
			}
			response, err := http.ReadResponse(reader, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil || response.StatusCode != 200 || !json.Valid(body) {
				t.Fatalf("incomplete response: %d %v %q", response.StatusCode, err, body)
			}
		})
	}
}
