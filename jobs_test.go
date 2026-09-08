package main

import (
	"context"
	"encoding/json"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGeneratedSourceRestrictedAndReconstructable(t *testing.T) {
	for _, metric := range []string{"l1", "l2"} {
		source, err := generatedKernel(metric)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), "adaptive_kernel.go", source, 0)
		if err != nil || len(file.Imports) != 0 {
			t.Fatal("generated kernel must be standalone numeric code", err)
		}
	}
	if _, err := generatedKernel("l1; os.RemoveAll"); err == nil {
		t.Fatal("arbitrary source input accepted")
	}
	src, err := embeddedSource()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err = writeSourceTree(src, dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range append(rebuildFiles, "source_bundle.json", "source_snapshot.txt") {
		if _, err = os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
}
func TestMissingRuntimeDoesNotLaunchOrPretendGPU(t *testing.T) {
	e := NewEngine(t.TempDir())
	if _, err := e.startEvolutionJob(JobRequest{Kind: "image", Prompt: "test"}); err == nil {
		t.Fatal("missing interpreter was ignored")
	}
	if len(e.lab.Jobs) != 0 {
		t.Fatal("job recorded as running without a runtime")
	}
	if w := call(e, "POST", "/api/evolution/config", map[string]any{"auto_train": true}); w.Code != 400 {
		t.Fatal("incomplete auto training enabled")
	}
}
func TestFailedBuildRetainsIncumbent(t *testing.T) {
	e := NewEngine(t.TempDir())
	teachFixture(t, e)
	if _, err := e.trainRecognition(); err != nil {
		t.Fatal(err)
	}
	before := e.lab.Learning.Active.ID
	dir := t.TempDir()
	// An existing non-executable file is an intentionally failing compiler fixture.
	fake := filepath.Join(dir, "compiler.exe")
	_ = os.WriteFile(fake, []byte("not an executable"), 0700)
	e.jobConfig = EvolutionConfig{GoCompiler: fake, MaxMinutes: 1}
	job, err := e.startEvolutionJob(JobRequest{Kind: "rebuild"})
	if err != nil {
		t.Fatal(err)
	}
	e.jobWG.Wait()
	if e.lab.Learning.Active.ID != before {
		t.Fatal("failed build changed the active model")
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.lab.Jobs[0].ID != job.ID || e.lab.Jobs[0].Status != "failed" || len(e.lab.Jobs[0].Artifacts) != 0 {
		t.Fatal("failed build looked successful")
	}
}
func TestJobCancellationOnStopAndInterruptedRecovery(t *testing.T) {
	e := NewEngine(t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	e.jobCancel = cancel
	e.jobWG.Add(1)
	go func() { defer e.jobWG.Done(); <-ctx.Done() }()
	e.Stop()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("stop did not cancel own worker")
	}
	if !e.jobsClosing {
		t.Fatal("late job starts still allowed")
	}
	n := NewEngine(t.TempDir())
	n.lab.Jobs = []EvolutionJob{{ID: "unfinished", Status: "running"}}
	n.persist()
	recovered := NewEngine(filepath.Dir(n.dataDir))
	if recovered.lab.Jobs[0].Status != "interrupted" {
		t.Fatal("stale running status survived restart")
	}
}
func TestBuildEnvironmentOfflineAndControlled(t *testing.T) {
	t.Setenv("GOFLAGS", "-toolexec=unexpected")
	t.Setenv("GOTOOLCHAIN", "auto")
	t.Setenv("GOOS", "darwin")
	env := strings.Join(buildEnvironment("windows"), "\n")
	for _, want := range []string{"GOTOOLCHAIN=local", "GOPROXY=off", "CGO_ENABLED=0", "GOOS=windows"} {
		if !strings.Contains(env, want) {
			t.Fatal(want)
		}
	}
	if strings.Contains(env, "unexpected") || strings.Contains(env, "GOOS=darwin") {
		t.Fatal("ambient build flags leaked")
	}
}
func TestAdapterRequiresCompletedJobAndVisualReview(t *testing.T) {
	e := NewEngine(t.TempDir())
	if w := call(e, "POST", "/api/evolution/adapter", map[string]any{"id": "absent", "preferred": true, "reason": "looks better"}); w.Code != 400 {
		t.Fatal("nonexistent adapter activated")
	}
	e.lab.Jobs = []EvolutionJob{{ID: "trained", Kind: "train_lora", Status: "completed"}}
	if w := call(e, "POST", "/api/evolution/adapter", map[string]any{"id": "trained", "preferred": true, "reason": ""}); w.Code != 400 {
		t.Fatal("missing review accepted")
	}
	if w := call(e, "POST", "/api/evolution/adapter", map[string]any{"id": "trained", "preferred": true, "reason": "Sharper masonry and more accurate window spacing"}); w.Code != 204 {
		t.Fatal(w.Body.String())
	}
	if e.lab.ActiveAdapter != "trained" {
		t.Fatal("reviewed adapter did not activate")
	}
}
func TestModelWorkerManifestValidationBeforeExecution(t *testing.T) {
	e := NewEngine(t.TempDir())
	req := JobRequest{Kind: "train_lora", Prompt: "test", Steps: 20}
	dir := t.TempDir()
	err := e.runLocalModel(context.Background(), EvolutionConfig{}, req, "unused", dir, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "captioned") {
		t.Fatal("training started without labelled image evidence", err)
	}
	b, _ := json.Marshal(e.jobConfig)
	if strings.Contains(string(b), "Key") {
		t.Fatal("provider key unexpectedly entered local runtime config")
	}
}
