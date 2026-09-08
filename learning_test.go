package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func teachFixture(t *testing.T, e *Engine) {
	t.Helper()
	for c := 0; c < 2; c++ {
		for i := 0; i < 5; i++ {
			b := append(bytes.Repeat([]byte{byte(32 + c*192)}, 800+i*20), bytes.Repeat([]byte{100}, i+1)...)
			a, err := e.storeObject(bytes.NewReader(b), fmt.Sprintf("c%d-%d.dat", c, i), "application/octet-stream", "synthetic test")
			if err != nil {
				t.Fatal(err)
			}
			if err = e.labelAsset(a.ID, fmt.Sprintf("class-%d", c), "", ""); err != nil {
				t.Fatal(err)
			}
		}
	}
}
func TestRecognitionTrainingHeldOutAndPersistence(t *testing.T) {
	base := t.TempDir()
	e := NewEngine(base)
	teachFixture(t, e)
	r, err := e.trainRecognition()
	if err != nil {
		t.Fatal(err)
	}
	if !r.Promoted || len(r.Candidates) != 6 || r.Validation.Total != 2 || r.Audit.Total != 2 || r.Audit.Accuracy != 1 {
		t.Fatalf("unexpected actual result %+v", r)
	}
	ids := map[string]bool{}
	for _, x := range e.lab.Learning.Active.Samples {
		if x.Split != "train" {
			t.Fatal("held-out example fitted")
		}
		ids[x.Asset] = true
	}
	for _, x := range e.lab.Learning.Examples {
		if x.Split != "train" && ids[x.Asset] {
			t.Fatal("split leakage")
		}
	}
	before := e.lab.Learning.Active.ID
	r, err = e.trainRecognition()
	if err != nil || len(e.lab.Learning.Runs) != 1 {
		t.Fatal("same evidence evaluated repeatedly")
	}
	n := NewEngine(base)
	if n.lab.Learning.Active.ID != before || len(n.lab.Learning.Examples) != 10 {
		t.Fatal("learning did not survive restart")
	}
}
func TestRecognitionRejectsInsufficientOrCorrelatedEvidence(t *testing.T) {
	e := NewEngine(t.TempDir())
	for i := 0; i < 6; i++ {
		a, err := e.storeObject(bytes.NewReader(bytes.Repeat([]byte{byte(21 + i)}, 100+i)), "sample", "application/octet-stream", "test")
		if err != nil {
			t.Fatal(err)
		}
		if err = e.labelAsset(a.ID, "same-class", "", "one-shoot"); err != nil {
			t.Fatal(err)
		}
	}
	for _, x := range e.lab.Learning.Examples {
		if x.Split != "train" {
			t.Fatal("same subject entered a second split")
		}
	}
	if _, err := e.trainRecognition(); err == nil {
		t.Fatal("unmeasured model promoted")
	}
	if e.lab.Learning.Active.ID != "" {
		t.Fatal("model existed without evaluation")
	}
}
func TestLabelUpdateDeduplicatesAndSnapshotsRestoreLearning(t *testing.T) {
	e := NewEngine(t.TempDir())
	teachFixture(t, e)
	_, err := e.trainRecognition()
	if err != nil {
		t.Fatal(err)
	}
	cp, err := e.checkpoint("trained")
	if err != nil {
		t.Fatal(err)
	}
	x := e.lab.Learning.Examples[0]
	rev := e.lab.Learning.Revision
	if err = e.labelAsset(x.Asset, x.Label, x.Caption, x.Group); err != nil {
		t.Fatal(err)
	}
	if e.lab.Learning.Revision != rev || len(e.lab.Learning.Examples) != 10 {
		t.Fatal("duplicate evidence inflated")
	}
	if err = e.labelAsset(x.Asset, "corrected", x.Caption, x.Group); err != nil {
		t.Fatal(err)
	}
	if e.lab.Learning.Revision != rev+1 {
		t.Fatal("correction did not invalidate evaluation")
	}
	if err = e.snapshotAction(cp.ID, "restore", ""); err != nil {
		t.Fatal(err)
	}
	if e.lab.Learning.Examples[0].Label != x.Label {
		t.Fatal("snapshot lost learning")
	}
}
func TestImageFeaturesAndCompiledKernel(t *testing.T) {
	im := image.NewRGBA(image.Rect(0, 0, 30, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 30; x++ {
			im.Set(x, y, color.RGBA{uint8(100 + x), uint8(y), 30, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, im); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "image.png")
	if err := os.WriteFile(p, b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	f, kind, err := extractFeatures(p)
	if err != nil {
		t.Fatal(err)
	}
	if kind != "image" || len(f) != 64 || f[63] != 1 || f[59] != 0.6 {
		t.Fatal("actual pixel features absent")
	}
	other := append([]float64{}, f...)
	other[45] += 0.25
	got, want := compiledFeatureDistance(f, other), referenceDistance(f, other, compiledRecognitionMetric)
	if math.Abs(got-want) > 1e-12 {
		t.Fatal("compiled and runtime implementations differ")
	}
}
func TestAuditCannotChooseOrPromoteCandidate(t *testing.T) {
	e := NewEngine(t.TempDir())
	teachFixture(t, e)
	for i := range e.lab.Learning.Examples {
		x := &e.lab.Learning.Examples[i]
		if x.Split == "audit" {
			if x.Label == "class-0" {
				x.Label = "class-1"
			} else {
				x.Label = "class-0"
			}
		}
	}
	r, err := e.trainRecognition()
	if err != nil {
		t.Fatal(err)
	}
	if !r.Promoted || r.Validation.Accuracy != 1 || r.Audit.Accuracy != 0 {
		t.Fatal("audit was used for candidate selection or was not measured honestly")
	}
}
func TestLearningAPIHasNoRawFeaturePayloadInLiveState(t *testing.T) {
	e := NewEngine(t.TempDir())
	teachFixture(t, e)
	w := call(e, "POST", "/api/learning/state", map[string]any{})
	var s LearningState
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Examples) != 10 || len(s.Examples[0].Features) != 0 {
		t.Fatal("summary should expose labels, not heavy feature arrays")
	}
	if len(e.view().Lab.Learning.Examples) != 0 {
		t.Fatal("live state grew with the training corpus")
	}
}
