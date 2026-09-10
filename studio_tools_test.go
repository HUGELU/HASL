package main

import (
	"bytes"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHardwareRecommendationsRespectMemoryAndDevice(t *testing.T) {
	low := recommendImage(HardwareProfile{Threads: 2, Memory: 8 << 30}, "detail", true)
	if low.Width != 256 || low.Config.Threads != 1 {
		t.Fatal(low)
	}
	laptop := recommendImage(HardwareProfile{Threads: 16, Memory: 32 << 30}, "detail", false)
	if laptop.Width != 512 || laptop.Steps != 9 {
		t.Fatal(laptop)
	}
	gpu := recommendImage(HardwareProfile{Threads: 16, Memory: 32 << 30}, "detail", true)
	if gpu.Width != 1024 {
		t.Fatal(gpu)
	}
	if parseMemoryKB("MemTotal:       32768 kB\n") != 32<<20 {
		t.Fatal("RAM parser")
	}
}
func TestReferenceRevisionAndFeedbackRoundTrip(t *testing.T) {
	e := fakeImages(t)
	a, err := e.storeObject(bytes.NewReader(testConceptPNG(1)), "ref.png", "image/png", "test")
	if err != nil {
		t.Fatal(err)
	}
	req := testImageRequest()
	req.InitAsset = a.ID
	req.Strength = .5
	req.Steps = 4
	req.Preview = true
	job, err := e.images.submit(req)
	if err != nil {
		t.Fatal(err)
	}
	waitImage(t, e.images, job.ID, "completed")
	b, err := os.ReadFile(filepath.Join(e.images.root(), "runs", job.ID, "input.png"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = png.Decode(bytes.NewReader(b)); err != nil {
		t.Fatal(err)
	}
	ratings := ImageRatings{Match: 4, Quality: 3, Consistency: 4, Speed: 2, Notes: "Try softer lighting"}
	if err = e.images.rateImage(job.ID, ratings); err != nil {
		t.Fatal(err)
	}
	rr := studioRequest(e, "/api/images/metadata", map[string]string{"id": job.ID})
	if rr.Code != 200 {
		t.Fatal(rr.Body.String())
	}
	var v struct {
		Schema  string
		Request ImageRequest
		Ratings ImageRatings
	}
	json.Unmarshal(rr.Body.Bytes(), &v)
	if v.Schema != "origin0.image.v1" || v.Request.InitAsset != a.ID || v.Ratings.Notes != ratings.Notes {
		t.Fatal(v)
	}
	req.Shared = true
	if validateImageRequest(req) == nil {
		t.Fatal("reference would escape to peer without its image")
	}
	req.Shared = false
	req.InitAsset = strings.Repeat("f", 64)
	if _, err = e.images.submit(req); err == nil {
		t.Fatal("missing reference queued")
	}
	if e.images.rateImage(job.ID, ImageRatings{}) == nil {
		t.Fatal("automatic zero scores accepted")
	}
}
func TestUnknownLicenceCanBeExcludedAndExportsStripSecrets(t *testing.T) {
	e := studioEngine(t)
	p := e.lab.Studio.Projects[0]
	x, err := e.addConceptImage(p.ID, testConceptPNG(2), "test", ConceptSource{License: "unknown", URL: "https://user:pass@example.org/image.png?token=secret#private"})
	if err != nil {
		t.Fatal(err)
	}
	if err = e.reviewConcept(p.ID, x.Asset, "test", "An exact caption", "rejected", "unknown"); err != nil {
		t.Fatal(err)
	}
	if err = e.reviewConcept(p.ID, x.Asset, "test", "An exact caption", "accepted", "unknown"); err == nil {
		t.Fatal("unreviewed rights accepted")
	}
	if err = e.reviewConcept(p.ID, x.Asset, "test", "An exact caption", "accepted", "owned"); err != nil {
		t.Fatal(err)
	}
	project, _ := e.conceptProject(p.ID)
	b, _ := canonicalRecipe(project)
	for _, secret := range []string{"user:pass", "token=secret", "#private"} {
		if bytes.Contains(b, []byte(secret)) {
			t.Fatal("export leaked", secret)
		}
	}
}
func TestProgressNeverFabricatesLoadingPercentage(t *testing.T) {
	if imageProgress(ImageJob{Status: "running", Log: "loading weights"})["percent"] != nil {
		t.Fatal("invented progress")
	}
	if imageProgress(ImageJob{Status: "running", Log: "2/8 - 1.20s/it"})["percent"] != 25 {
		t.Fatal("sampling progress")
	}
}

func TestRatingsLearnOnlyFromRepeatedComparableEvidence(t *testing.T) {
	req := testImageRequest()
	var jobs []*ImageJob
	for i := 0; i < 3; i++ {
		r := req
		r.Steps = 8
		jobs = append(jobs, &ImageJob{Status: "completed", Pack: "pack", Request: r, Ratings: &ImageRatings{Match: 4, Quality: 5, Consistency: 4, Speed: 3}})
	}
	if r, n := ratedSettings(jobs[:2], req, "pack"); n != 0 || r.Steps != 1 {
		t.Fatal("too little evidence changed settings")
	}
	if r, n := ratedSettings(jobs, req, "pack"); n != 3 || r.Steps != 8 {
		t.Fatal("feedback did not alter settings", r, n)
	}
	req.Prompt = "a different request"
	if _, n := ratedSettings(jobs, req, "pack"); n != 0 {
		t.Fatal("unrelated prompts counted as evidence")
	}
}
