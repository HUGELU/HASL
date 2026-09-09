package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestComputePreferenceSurvivesRestart(t *testing.T) {
	base := t.TempDir()
	e := NewEngine(base)
	w := call(e, "POST", "/api/compute", map[string]any{"target": 65})
	if w.Code != 204 || NewEngine(base).computeTarget.Load() != 65 {
		t.Fatal("saved compute preference lost")
	}
}

func TestLegacyStatePreservedAndLayoutNormalised(t *testing.T) {
	base := t.TempDir()
	e := NewEngine(base)
	p := PersistState{UIGenome: e.ui, Swarms: e.swarms}
	p.UIGenome.Order = []string{"activity", "obsolete", "activity"}
	p.Swarms[0].Discoveries = 999
	p.Swarms[0].Memory = nil
	b, _ := json.Marshal(p)
	_ = os.WriteFile(filepath.Join(e.dataDir, "state.json"), b, 0600)
	n := NewEngine(base)
	backup, err := os.ReadFile(filepath.Join(n.dataDir, "state.before-laboratory.json"))
	if err != nil || string(backup) != string(b) || len(n.ui.Order) != len(panelIDs) || n.swarms[0].Discoveries != 0 {
		t.Fatal("legacy preservation or normalisation failed")
	}
}

func TestMeasurementOffClearsTimingAndRejectsNewTask(t *testing.T) {
	e := NewEngine(t.TempDir())
	tr := passing(e.ui.ID)
	tr.Samples, tr.TotalMS = 5, 12000
	e.lab.Trials[e.ui.ID] = tr
	w := call(e, "POST", "/api/settings", map[string]any{"track": false})
	if w.Code != 204 || e.lab.Trials[e.ui.ID].Samples != 0 {
		t.Fatal("timing retained after measurement disabled")
	}
	w = call(e, "POST", "/api/ui/task", map[string]any{"id": e.ui.ID, "ms": 500, "errors": 0, "task": "find-review"})
	if w.Code != 400 {
		t.Fatal("task timing recorded with measurement disabled")
	}
}
