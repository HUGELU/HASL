package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func studioEngine(t *testing.T) *Engine {
	t.Helper()
	e := NewEngine(t.TempDir())
	t.Cleanup(e.Stop)
	e.mu.Lock()
	e.lab.Studio.Projects = []ConceptProject{{ID: "abc123abc123abcd", Name: "Position", Subject: "A red square", AutoLearn: true, Examples: []ConceptExample{}}}
	e.mu.Unlock()
	return e
}
func testConceptPNG(x int) []byte {
	im := image.NewRGBA(image.Rect(0, 0, 64, 64))
	draw.Draw(im, im.Bounds(), &image.Uniform{color.RGBA{245, 245, 245, 255}}, image.Point{}, draw.Src)
	draw.Draw(im, image.Rect(x, 20, x+12, 35), &image.Uniform{color.RGBA{230, 30, 45, 255}}, image.Point{}, draw.Src)
	var b bytes.Buffer
	_ = png.Encode(&b, im)
	return b.Bytes()
}
func studioRequest(e *Engine, path string, payload any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "http://127.0.0.1:8765"+path, bytes.NewReader(b))
	req.Header.Set("X-Origin-Key", e.sessionKey)
	w := httptest.NewRecorder()
	e.handler().ServeHTTP(w, req)
	return w
}

type conceptRoundTrip func(*http.Request) (*http.Response, error)

func (f conceptRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func conceptResponse(body []byte) *http.Response {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}}
}

func TestConceptSpatialFeaturesDistinguishPosition(t *testing.T) {
	_, left, _, err := conceptImage(testConceptPNG(5))
	if err != nil {
		t.Fatal(err)
	}
	_, right, _, err := conceptImage(testConceptPNG(45))
	if err != nil {
		t.Fatal(err)
	}
	if referenceDistance(left, right, "l1") < .01 {
		t.Fatal("spatial positions collapsed")
	}
	if len(left) != 64 {
		t.Fatal(len(left))
	}
}
func TestConceptRejectsNonImagesAndLargeInputs(t *testing.T) {
	e := studioEngine(t)
	for _, b := range [][]byte{[]byte("<svg onload='anything'>"), []byte("not an image"), make([]byte, conceptImageLimit+1)} {
		if _, err := e.addConceptImage("abc123abc123abcd", b, "bad", ConceptSource{}); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
}
func TestConceptDuplicateAndPendingReview(t *testing.T) {
	e := studioEngine(t)
	id := "abc123abc123abcd"
	b := testConceptPNG(4)
	one, err := e.addConceptImage(id, b, "same.png", ConceptSource{License: "cc0"})
	if err != nil {
		t.Fatal(err)
	}
	two, err := e.addConceptImage(id, b, "other.png", ConceptSource{License: "cc0"})
	if err != nil {
		t.Fatal(err)
	}
	p, _ := e.conceptProject(id)
	if one.Asset != two.Asset || len(p.Examples) != 1 || one.Review != "pending" {
		t.Fatal("duplicate or unreviewed input entered training")
	}
	if len(projectLearning(p).Examples) != 0 {
		t.Fatal("pending example entered training")
	}
	if err = e.reviewConcept(id, one.Asset, "red", "a red square", "accepted", "cc0"); err != nil {
		t.Fatal(err)
	}
	p, _ = e.conceptProject(id)
	if len(projectLearning(p).Examples) != 1 {
		t.Fatal("accepted image absent")
	}
}
func TestConceptSameSourceStaysInOneSplit(t *testing.T) {
	e := studioEngine(t)
	id := "abc123abc123abcd"
	a, _ := e.addConceptImage(id, testConceptPNG(3), "v1", ConceptSource{URL: "https://example.com/photo.png", License: "by"})
	b, _ := e.addConceptImage(id, testConceptPNG(40), "v2", ConceptSource{URL: "https://example.com/photo.png", License: "by"})
	if a.Asset == b.Asset || a.Group != b.Group || a.Split != b.Split {
		t.Fatal("source variants leak across splits")
	}
}
func TestConceptSyntheticTrainingAndAudit(t *testing.T) {
	for _, axis := range []string{"horizontal", "vertical", "power", "containment", "colour"} {
		t.Run(axis, func(t *testing.T) {
			e := studioEngine(t)
			n, err := e.seedConcept("abc123abc123abcd", axis)
			if err != nil {
				t.Fatal(err)
			}
			p, _ := e.conceptProject("abc123abc123abcd")
			if n != 20 || len(p.Examples) != 20 || p.Active.ID == "" || len(p.Runs) != 1 {
				t.Fatalf("curriculum did not produce a model: %d %d %+v %s", n, len(p.Examples), p.Runs, p.Note)
			}
			r := p.Runs[0]
			if len(r.Candidates) != 6 || r.Audit.Total != 4 || r.Validation.Total != 4 {
				t.Fatalf("missing independent evaluations: %+v", r)
			}
			training := map[string]bool{}
			for _, x := range p.Active.Samples {
				training[x.Group] = true
			}
			for _, x := range p.Examples {
				if x.Split != "train" && training[x.Group] {
					t.Fatal("held-out group leaked into trained model")
				}
			}
			r2, err := e.trainConcept(p.ID)
			if err != nil || r.ID != r2.ID {
				t.Fatal("same evidence was retrained or audit reused to select a new candidate")
			}
		})
	}
}
func TestConceptProjectIsolationForImageTraining(t *testing.T) {
	e := studioEngine(t)
	_, err := e.seedConcept("abc123abc123abcd", "horizontal")
	if err != nil {
		t.Fatal(err)
	}
	train, val, _, err := e.conceptTrainingRows("abc123abc123abcd")
	if err != nil {
		t.Fatal(err)
	}
	if len(train) != 12 || len(val) != 4 {
		t.Fatalf("wrong selected data: %d %d", len(train), len(val))
	}
	p, _ := e.conceptProject("abc123abc123abcd")
	audit := map[string]bool{}
	for _, x := range p.Examples {
		if x.Split == "audit" {
			audit[x.Asset] = true
		}
	}
	for _, x := range append(train, val...) {
		if audit[x["asset"]] {
			t.Fatal("audit sent to image trainer")
		}
	}
	if _, _, _, err = e.conceptTrainingRows("other"); err == nil {
		t.Fatal("unknown project fell back to global examples")
	}
}
func TestConceptExportDatasetAndPrivateState(t *testing.T) {
	e := studioEngine(t)
	_, _ = e.seedConcept("abc123abc123abcd", "horizontal")
	w := studioRequest(e, "/api/concepts/export", map[string]any{"id": "abc123abc123abcd", "images": true})
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	z, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, f := range z.File {
		parts := strings.Split(f.Name, "/")
		if len(parts) > 2 && parts[1] == "images" {
			counts[parts[0]]++
		}
	}
	if counts["train"] != 12 || counts["validation"] != 4 || counts["audit"] != 4 {
		t.Fatal(counts)
	}
	r := studioRequest(e, "/api/concepts/export", map[string]any{"id": "abc123abc123abcd", "images": false})
	if r.Code != 200 || strings.Contains(r.Body.String(), e.dataDir) || strings.Contains(r.Body.String(), e.sessionKey) || strings.Contains(r.Body.String(), "features") {
		t.Fatal("private runtime state in recipe")
	}
	v := studioRequest(e, "/api/concepts/state", map[string]any{})
	if strings.Contains(v.Body.String(), "\"features\"") {
		t.Fatal("feature vectors sent to UI")
	}
}
func TestConceptSnapshotAndRestore(t *testing.T) {
	e := studioEngine(t)
	_, _ = e.seedConcept("abc123abc123abcd", "horizontal")
	saved, err := e.checkpoint("Concept checkpoint")
	if err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	e.lab.Studio.Projects[0].Name = "changed"
	e.mu.Unlock()
	if err = e.snapshotAction(saved.ID, "restore", ""); err != nil {
		t.Fatal(err)
	}
	p, err := e.conceptProject("abc123abc123abcd")
	if err != nil || p.Name != "Position" || p.Active.ID == "" {
		t.Fatal("project model not restored", err)
	}
}
func TestConceptPublicNetworkBoundary(t *testing.T) {
	for _, raw := range []string{"http://example.com/x", "https://127.0.0.1/x", "https://[::1]/x", "https://10.0.0.1/x", "https://169.254.169.254/x", "https://100.64.0.1/x", "https://example.com:8443/x", "https://user:secret@example.com/x"} {
		if _, err := publicWebURL(raw); err == nil {
			t.Fatal(raw)
		}
	}
	for _, ip := range []string{"192.168.1.2", "::1", "0.0.0.0", "100.64.2.1", "198.19.1.1", "2001:db8::1", "64:ff9b::a00:1"} {
		if publicAddress(net.ParseIP(ip)) {
			t.Fatal(ip)
		}
	}
	if _, err := publicWebURL("https://example.com/photo.jpg"); err != nil {
		t.Fatal(err)
	}
	if err := publicWebClient().CheckRedirect(httptest.NewRequest("GET", "http://127.0.0.1/", nil), nil); err == nil {
		t.Fatal("private redirect allowed")
	}
}
func TestConceptOpenverseCollectionUsesRealReturnedBytes(t *testing.T) {
	e := studioEngine(t)
	pngBytes := testConceptPNG(4)
	e.studio.searchClient = &http.Client{Transport: conceptRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.openverse.org" || r.URL.Query().Get("q") != "red square" {
			t.Fatal(r.URL)
		}
		return conceptResponse([]byte(`{"results":[{"id":"one","title":"Red square","url":"https://example.com/image.png","foreign_landing_url":"https://example.com/page","creator":"Artist","license":"cc0"}]}`)), nil
	})}
	e.studio.imageClient = &http.Client{Transport: conceptRoundTrip(func(r *http.Request) (*http.Response, error) { return conceptResponse(pngBytes), nil })}
	j, err := e.studio.collect("abc123abc123abcd", "openverse", "red square", 1, true)
	if err != nil || j.Status != "searching" {
		t.Fatal(err)
	}
	e.studio.wg.Wait()
	p, _ := e.conceptProject(j.Project)
	if len(p.Examples) != 1 || p.Examples[0].Source.Creator != "Artist" || p.Examples[0].Review != "pending" {
		t.Fatal(p)
	}
	if e.studio.collection.Status != "completed" || e.studio.collection.Downloaded != 1 {
		t.Fatal(e.studio.collection)
	}
}
func TestConceptCollectionReportsFailureAndCancels(t *testing.T) {
	e := studioEngine(t)
	e.studio.searchClient = &http.Client{Transport: conceptRoundTrip(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })}
	_, err := e.studio.collect("abc123abc123abcd", "openverse", "test", 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.studio.collect("abc123abc123abcd", "openverse", "other", 1, true); err == nil {
		t.Fatal("overlapping crawler accepted")
	}
	e.studio.mu.Lock()
	e.studio.cancel()
	e.studio.mu.Unlock()
	done := make(chan struct{})
	go func() { e.studio.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cancellation stuck")
	}
	if e.studio.collection.Status != "stopped" {
		t.Fatal(e.studio.collection)
	}
}
func TestConceptUnknownRightsStayPending(t *testing.T) {
	e := studioEngine(t)
	x, err := e.addConceptImage("abc123abc123abcd", testConceptPNG(4), "photo", ConceptSource{License: "invented"})
	if err != nil {
		t.Fatal(err)
	}
	if err = e.reviewConcept("abc123abc123abcd", x.Asset, "square", "square", "accepted", ""); err == nil {
		t.Fatal("unknown licence silently approved")
	}
	p, _ := e.conceptProject("abc123abc123abcd")
	if p.Examples[0].Review != "pending" {
		t.Fatal("partial invalid mutation")
	}
}
func TestConceptAPIAuthAndConfiguration(t *testing.T) {
	e := studioEngine(t)
	r := httptest.NewRequest("POST", "http://127.0.0.1/api/concepts/state", strings.NewReader("{}"))
	w := httptest.NewRecorder()
	e.handler().ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	for _, raw := range []string{"http://10.0.0.1", "file:///tmp/search", "https://host.test/?key=secret"} {
		w = studioRequest(e, "/api/concepts/searxng", map[string]string{"url": raw})
		if w.Code == 204 {
			t.Fatal("unapproved endpoint", raw)
		}
	}
	w = studioRequest(e, "/api/concepts/searxng", map[string]string{"url": "http://127.0.0.1:8080"})
	if w.Code != 204 {
		t.Fatal(w.Body.String())
	}
}
func TestConceptDownloadBounds(t *testing.T) {
	client := &http.Client{Transport: conceptRoundTrip(func(*http.Request) (*http.Response, error) { return conceptResponse([]byte("123456")), nil })}
	if _, err := webBytes(context.Background(), client, "https://example.com", 5); err == nil {
		t.Fatal("download limit ignored")
	}
}
