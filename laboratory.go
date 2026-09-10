package main

import (
	"archive/zip"
	"bytes"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type UIField struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Kind     string `json:"kind"`
	Required bool   `json:"required"`
}
type UIBlock struct {
	ID     string    `json:"id"`
	Title  string    `json:"title"`
	Reason string    `json:"reason"`
	Source string    `json:"source"`
	Fields []UIField `json:"fields"`
}
type LayoutCheck struct {
	Width         int  `json:"width"`
	Overflow      bool `json:"overflow"`
	SmallTargets  int  `json:"small_targets"`
	MissingLabels int  `json:"missing_labels"`
	Controls      int  `json:"controls"`
}
type Trial struct {
	ID      string        `json:"id"`
	Checks  []LayoutCheck `json:"checks"`
	Tested  int64         `json:"tested"`
	Pass    bool          `json:"pass"`
	Samples int           `json:"samples"`
	TotalMS float64       `json:"total_ms"`
	Errors  int           `json:"errors"`
	Task    string        `json:"task"`
}
type Snapshot struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Branch  string `json:"branch"`
	Parent  string `json:"parent"`
	Created int64  `json:"created"`
	Cycles  uint64 `json:"cycles"`
	SHA256  string `json:"sha256"`
}
type Usage struct {
	Kind    string  `json:"kind"`
	Target  string  `json:"target"`
	UI      string  `json:"ui"`
	Device  string  `json:"device"`
	Width   int     `json:"width"`
	Count   int     `json:"count"`
	TotalMS float64 `json:"total_ms"`
	Errors  int     `json:"errors"`
}
type AssetRecord struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	MIME    string `json:"mime"`
	Size    int64  `json:"size"`
	Created int64  `json:"created"`
	Source  string `json:"source"`
}
type LabState struct {
	Studio        ConceptState        `json:"studio"`
	Learning      LearningState       `json:"learning"`
	Jobs          []EvolutionJob      `json:"jobs"`
	ActiveAdapter string              `json:"active_adapter"`
	Version       int                 `json:"version"`
	Goal          string              `json:"goal"`
	Branch        string              `json:"branch"`
	Parent        string              `json:"parent"`
	AutoUI        bool                `json:"auto_ui"`
	Track         bool                `json:"track"`
	Pins          []string            `json:"pins"`
	Trials        map[string]Trial    `json:"trials"`
	Usage         map[string]Usage    `json:"usage"`
	Snapshots     []Snapshot          `json:"snapshots"`
	Assets        []AssetRecord       `json:"assets"`
	Discovered    []UIBlock           `json:"discovered"`
	Answers       []map[string]string `json:"answers"`
	QuotaBytes    int64               `json:"quota_bytes"`
	StoredBytes   int64               `json:"stored_bytes"`
	ObjectCount   int64               `json:"object_count"`
	EngineNote    string              `json:"engine_note"`
	LastPromotion int64               `json:"last_promotion"`
}

func randomID() string {
	b := make([]byte, 24)
	if _, err := crand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func cloneUI(g UIGenome) UIGenome {
	b, _ := json.Marshal(g)
	var v UIGenome
	_ = json.Unmarshal(b, &v)
	return v
}
func cloneLab(g LabState) LabState {
	b, _ := json.Marshal(g)
	var v LabState
	_ = json.Unmarshal(b, &v)
	return v
}
func (e *Engine) initLab() {
	e.lab = LabState{Version: 1, Goal: "Generate useful images locally, organise supplied evidence, and improve the steps I use most.", Branch: "main", Track: true, AutoUI: true, QuotaBytes: 100 << 30, Trials: map[string]Trial{}, Usage: map[string]Usage{}, Pins: []string{}, Snapshots: []Snapshot{}, Assets: []AssetRecord{}, Discovered: []UIBlock{}}
	for _, d := range []string{"snapshots", "objects", "catalog", "logs", "outputs"} {
		_ = os.MkdirAll(filepath.Join(e.dataDir, d), 0700)
	}
}
func (e *Engine) repairLoadedState() {
	if e.lab.Version == 0 {
		e.initLab()
	}
	if e.lab.Trials == nil {
		e.lab.Trials = map[string]Trial{}
	}
	if e.lab.Usage == nil {
		e.lab.Usage = map[string]Usage{}
	}
	order, seen := []string{}, map[string]bool{}
	for _, id := range append(e.ui.Order, panelIDs...) {
		for _, known := range panelIDs {
			if id == known && !seen[id] {
				order = append(order, id)
				seen[id] = true
			}
		}
	}
	e.ui.Order = order
	// Clear inherited unmeasured scores; only browser-measured trials can promote a layout.
	e.ui.Score = 0
	for _, s := range e.swarms {
		if s.Memory == nil {
			s.Memory = map[string]float64{}
			s.Discoveries, s.LocalHypotheses, s.Exchanges = 0, 0, 0
		}
	}
	for i := range e.approvals {
		if e.approvals[i].Status == "approved" {
			e.approvals[i].Status = "recorded (no active grant)"
		}
	}
}
func (e *Engine) Stop() {
	e.stopOnce.Do(func() {
		close(e.stop)
		e.paused.Store(true)
		e.images.close()
		if e.studio != nil {
			e.studio.close()
		}
		e.stopEvolutionJobs()
		e.wg.Wait()
		e.persist()
	})
}
func atomicWrite(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".write-")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
func (e *Engine) stateBytes() ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return json.Marshal(PersistState{ComputeTarget: int(e.computeTarget.Load()), Lab: e.lab, Cycles: e.cycle.Load(), Concepts: e.concepts, Relations: e.relations, Hypotheses: e.hypotheses, Questions: e.questions, Experiences: e.experiences, Swarms: e.swarms, UIGenome: e.ui, EngineGenome: e.engineGenome, Reflections: e.reflections})
}
func validID(s string) bool {
	if s == "" || len(s) > 100 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
func (e *Engine) checkpoint(name string) (Snapshot, error) {
	e.archiveMu.Lock()
	defer e.archiveMu.Unlock()
	if len(name) > 100 {
		name = name[:100]
	}
	if name == "" {
		name = "Saved state"
	}
	b, err := e.stateBytes()
	if err != nil {
		return Snapshot{}, err
	}
	h := sha256.Sum256(b)
	e.mu.RLock()
	branch, parent := e.lab.Branch, e.lab.Parent
	e.mu.RUnlock()
	s := Snapshot{ID: "state-" + randomID()[:16], Name: name, Branch: branch, Parent: parent, Created: now(), Cycles: e.cycle.Load(), SHA256: hex.EncodeToString(h[:])}
	if err = atomicWrite(filepath.Join(e.dataDir, "snapshots", s.ID+".json"), b); err != nil {
		return s, err
	}
	meta, _ := json.Marshal(s)
	if err = atomicWrite(filepath.Join(e.dataDir, "snapshots", s.ID+".meta.json"), meta); err != nil {
		return s, err
	}
	e.mu.Lock()
	e.lab.Snapshots = append(e.lab.Snapshots, s)
	e.lab.Parent = s.ID
	e.mu.Unlock()
	e.persist()
	e.addEvent("SNAPSHOT", "Saved "+name)
	return s, nil
}
func (e *Engine) readSnapshot(id string) (PersistState, error) {
	var p PersistState
	if !validID(id) {
		return p, errors.New("invalid snapshot ID")
	}
	var meta Snapshot
	b, err := os.ReadFile(filepath.Join(e.dataDir, "snapshots", id+".meta.json"))
	if err != nil {
		return p, err
	}
	if err = json.Unmarshal(b, &meta); err != nil {
		return p, err
	}
	b, err = os.ReadFile(filepath.Join(e.dataDir, "snapshots", id+".json"))
	if err != nil {
		return p, err
	}
	h := sha256.Sum256(b)
	if hex.EncodeToString(h[:]) != meta.SHA256 {
		return p, errors.New("snapshot checksum mismatch")
	}
	err = json.Unmarshal(b, &p)
	return p, err
}
func (e *Engine) snapshotAction(id, action, name string) error {
	p, err := e.readSnapshot(id)
	if err != nil {
		return err
	}
	if action != "restore" && action != "branch" && action != "merge" && action != "join" {
		return errors.New("unknown snapshot action")
	}
	if _, err = e.checkpoint("Before " + action); err != nil {
		return err
	}
	e.learningMu.Lock()
	defer e.learningMu.Unlock()
	e.ingestMu.Lock()
	defer e.ingestMu.Unlock()
	e.mu.Lock()
	switch action {
	case "restore", "branch":
		// Keep the current archive and grants. Historical permissions are never replayed.
		old := e.lab
		e.concepts = p.Concepts
		e.relations = p.Relations
		e.hypotheses = p.Hypotheses
		e.questions = p.Questions
		e.experiences = p.Experiences
		e.swarms = p.Swarms
		e.ui = cloneUI(p.UIGenome)
		e.engineGenome = p.EngineGenome
		e.reflections = p.Reflections
		e.lab = p.Lab
		e.lab.Jobs = old.Jobs
		e.lab.ActiveAdapter = old.ActiveAdapter
		e.initLearning()
		e.lab.Snapshots = old.Snapshots
		e.lab.StoredBytes = old.StoredBytes
		e.lab.ObjectCount = old.ObjectCount
		e.lab.Assets = old.Assets
		e.lab.QuotaBytes = old.QuotaBytes
		e.lab.Track = old.Track
		e.lab.Pins = old.Pins
		e.lab.Parent = id
		if action == "branch" {
			if name == "" {
				name = "branch-" + randomID()[:6]
			}
			e.lab.Branch = name
		}
		e.uiCandidates = nil
		e.cycle.Store(p.Cycles)
	case "merge":
		e.mergeLearningLocked(p.Lab.Learning)
		// Idempotent union. Replayed evidence is not added to counts.
		for k, v := range p.Concepts {
			if e.concepts[k] == nil && len(e.concepts) < 6000 {
				e.concepts[k] = v
			}
		}
		for k, v := range p.Relations {
			if e.relations[k] == nil && len(e.relations) < 18000 {
				if e.concepts[v.A] != nil && e.concepts[v.B] != nil {
					e.relations[k] = v
				}
			}
		}
		for k, v := range p.Hypotheses {
			if e.hypotheses[k] == nil && len(e.hypotheses) < 6000 {
				e.hypotheses[k] = v
			}
		}
	case "join":
		if len(e.swarms) >= 12 {
			e.mu.Unlock()
			return errors.New("12 internal descendants active; save a branch before replacing one")
		}
		if len(p.Swarms) > 0 {
			s := *p.Swarms[0]
			s.ID = "archive-" + randomID()[:8]
			e.swarms = append(e.swarms, &s)
		}
	}
	e.lastCycleSample = e.cycle.Load()
	e.lastCycleSampleTime = time.Now()
	e.mu.Unlock()
	e.persist()
	e.addEvent("BRANCH", action+" "+id)
	return nil
}
func (e *Engine) evaluateEvidenceLocked(motifs map[string]int) {
	for _, h := range e.hypotheses {
		if motifs[h.A] > 0 {
			if motifs[h.B] > 0 {
				h.Support++
			} else {
				h.Contradictions++
			}
			h.Confidence = float64(h.Support+1) / float64(h.Support+h.Contradictions+2)
			h.LastTest = now()
		}
	}
}
func (e *Engine) recordExperience(x Experience) {
	e.catalogMu.Lock()
	defer e.catalogMu.Unlock()
	b, _ := json.Marshal(x)
	dir := filepath.Join(e.dataDir, "catalog", x.Hash[:2])
	_ = atomicWrite(filepath.Join(dir, x.ID+".json"), b)
}
func (e *Engine) discoverBlocks(name string, b []byte, source string) {
	// Raw motif learning is type-blind. This separate human aid uses explicit field names.
	var fields []UIField
	var obj map[string]any
	if len(b) < 256<<10 && json.Unmarshal(b, &obj) == nil && obj != nil {
		keys := []string{}
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if len(fields) >= 8 {
				break
			}
			kind := "text"
			switch obj[k].(type) {
			case float64:
				kind = "number"
			case bool:
				kind = "checkbox"
			case map[string]any, []any:
				continue
			}
			if len(k) > 60 {
				continue
			}
			fields = append(fields, UIField{ID: "f" + shortHash(k), Label: k, Kind: kind})
		}
	}
	if len(fields) == 0 {
		return
	}
	block := UIBlock{ID: "block-" + shortHash(fmt.Sprint(fields)), Title: "Capture another example", Reason: "Fields inferred from keys in " + name + ". Human-facing aid; not proof of semantic understanding.", Source: source, Fields: fields}
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, x := range e.lab.Discovered {
		if x.ID == block.ID {
			return
		}
	}
	if len(e.lab.Discovered) < 24 {
		e.lab.Discovered = append(e.lab.Discovered, block)
	}
}
func (e *Engine) seedExperiences() {
	e.ingestBytes("embedded-source-snapshot", []byte(e.sourceSnapshot), int64(len(e.sourceSnapshot)), "self-source")
	e.mu.Lock()
	if len(e.lab.Discovered) == 0 {
		e.lab.Discovered = []UIBlock{{ID: "huge-lead", Title: "HUGE · property observation", Reason: "Starter supplied for your immediate real-estate work. Confirm each observation yourself.", Source: "human-seeded", Fields: []UIField{{ID: "location", Label: "Location / address", Kind: "text", Required: true}, {ID: "observation", Label: "What did you observe?", Kind: "textarea", Required: true}, {ID: "next_step", Label: "Next action", Kind: "text"}}}, {ID: "reality", Title: "Compare an expectation with reality", Reason: "Supply an ordinary safe observation, its prediction, and actual outcome. Pain reports are human reports, not machine sensation.", Source: "human-seeded", Fields: []UIField{{ID: "prediction", Label: "What did you expect?", Kind: "text"}, {ID: "observed", Label: "What actually happened?", Kind: "textarea"}, {ID: "source", Label: "Source / conditions", Kind: "text"}}}}
	}
	e.mu.Unlock()
	e.generateUICandidates()
}
func checkTrial(t Trial) bool {
	required := map[int]bool{280: false, 320: false, 390: false, 768: false, 1440: false, 2560: false}
	for _, c := range t.Checks {
		if _, ok := required[c.Width]; ok && !c.Overflow && c.SmallTargets == 0 && c.MissingLabels == 0 && c.Controls > 0 {
			required[c.Width] = true
		}
	}
	for _, ok := range required {
		if !ok {
			return false
		}
	}
	return true
}
func (e *Engine) promoteMeasuredCandidate() {
	e.mu.RLock()
	if !e.lab.AutoUI || e.paused.Load() || time.Now().UnixMilli() < e.interactionUntil.Load() || now()-e.lab.LastPromotion < 120 {
		e.mu.RUnlock()
		return
	}
	base := e.lab.Trials[e.ui.ID]
	candidate := ""
	// Same scripted find-control task; only observed trials count, never synthetic scale tests.
	if base.Samples >= 5 {
		for _, g := range e.uiCandidates {
			t := e.lab.Trials[g.ID]
			if t.Pass && t.Samples >= 5 && t.Errors <= base.Errors && t.TotalMS/float64(t.Samples) < .9*base.TotalMS/float64(base.Samples) && g.Rejected == 0 {
				candidate = g.ID
				break
			}
		}
	}
	e.mu.RUnlock()
	if candidate != "" {
		_ = e.promote(candidate)
	}
}
func (e *Engine) promote(id string) error {
	e.mu.RLock()
	var choice *UIGenome
	for _, g := range e.uiCandidates {
		if g.ID == id {
			c := cloneUI(g)
			choice = &c
		}
	}
	trial := e.lab.Trials[id]
	e.mu.RUnlock()
	if choice == nil {
		return errors.New("candidate expired; create another experiment")
	}
	if !trial.Pass {
		return errors.New("run the responsive layout checks before applying")
	}
	if _, err := e.checkpoint("Before interface promotion"); err != nil {
		return err
	}
	e.mu.Lock()
	e.ui = *choice
	e.uiCandidates = nil
	e.lab.LastPromotion = now()
	e.mu.Unlock()
	e.persist()
	e.addEvent("UI-PROMOTE", "Applied measured layout "+id)
	return nil
}
func (e *Engine) storeObject(r io.Reader, name, mime, source string) (AssetRecord, error) {
	e.catalogMu.Lock()
	defer e.catalogMu.Unlock()
	e.mu.RLock()
	remaining := e.lab.QuotaBytes - e.lab.StoredBytes
	e.mu.RUnlock()
	if remaining <= 0 {
		return AssetRecord{}, errors.New("archive quota reached; raise it in Settings after checking disk space")
	}
	f, err := os.CreateTemp(filepath.Join(e.dataDir, "objects"), "incoming-")
	if err != nil {
		return AssetRecord{}, err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(r, remaining+1))
	syncErr := f.Sync()
	closeErr := f.Close()
	if err != nil {
		return AssetRecord{}, err
	}
	if syncErr != nil {
		return AssetRecord{}, syncErr
	}
	if closeErr != nil {
		return AssetRecord{}, closeErr
	}
	if n > remaining {
		return AssetRecord{}, errors.New("object exceeds remaining archive quota")
	}
	id := hex.EncodeToString(h.Sum(nil))
	dir := filepath.Join(e.dataDir, "objects", id[:2])
	if err = os.MkdirAll(dir, 0700); err != nil {
		return AssetRecord{}, err
	}
	dst := filepath.Join(dir, id)
	exists := false
	if _, err = os.Stat(dst); err == nil {
		exists = true
	} else if err = os.Rename(tmp, dst); err != nil {
		return AssetRecord{}, err
	}
	if len(name) > 180 {
		name = name[:180]
	}
	a := AssetRecord{ID: id, Name: name, MIME: mime, Size: n, Created: now(), Source: source}
	meta, _ := json.Marshal(a)
	if err = atomicWrite(dst+".json", meta); err != nil {
		return a, err
	}
	e.mu.Lock()
	if !exists {
		e.lab.StoredBytes += n
		e.lab.ObjectCount++
	}
	found := false
	for _, x := range e.lab.Assets {
		if x.ID == id {
			found = true
			break
		}
	}
	if !found {
		e.lab.Assets = append(e.lab.Assets, a)
		if len(e.lab.Assets) > 100 {
			e.lab.Assets = e.lab.Assets[len(e.lab.Assets)-100:]
		}
	}
	e.mu.Unlock()
	return a, nil
}
func (e *Engine) objectPath(id string) (string, error) {
	if len(id) != 64 {
		return "", errors.New("invalid object ID")
	}
	if _, err := hex.DecodeString(id); err != nil {
		return "", err
	}
	return filepath.Join(e.dataDir, "objects", id[:2], id), nil
}
func jsonReply(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
func decode(r *http.Request, v any) error {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return errors.New("one JSON object required")
	}
	return nil
}
func apiError(w http.ResponseWriter, err error) { http.Error(w, err.Error(), 400) }
func (e *Engine) laboratoryHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		b, _ := assets.ReadFile("web/index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(b)
	})
	for _, file := range []string{"app.js", "style.css", "evolution.js", "generator.js", "generator.css", "concepts.js", "concepts.css"} {
		f := file
		mux.HandleFunc("/"+f, func(w http.ResponseWriter, r *http.Request) {
			b, _ := assets.ReadFile("web/" + f)
			if strings.HasSuffix(f, "js") {
				w.Header().Set("Content-Type", "text/javascript")
			} else {
				w.Header().Set("Content-Type", "text/css")
			}
			w.Write(b)
		})
	}
	mux.HandleFunc("/local-models", func(w http.ResponseWriter, r *http.Request) {
		b, _ := assets.ReadFile("LOCAL_MODELS.md")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(b)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ORIGIN-0 OK") })
	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		v := e.view()
		v.Health["paused"] = e.paused.Load()
		v.Health["total_concepts"] = lenCopy(e, "concepts")
		v.Health["total_hypotheses"] = lenCopy(e, "hypotheses")
		jsonReply(w, v)
	})
	mux.HandleFunc("/api/observe", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			Text string `json:"text"`
		}
		if err := decode(r, &v); err != nil {
			apiError(w, err)
			return
		}
		if strings.TrimSpace(v.Text) == "" {
			apiError(w, errors.New("enter an observation"))
			return
		}
		select {
		case e.observeQueue <- v.Text:
			w.WriteHeader(202)
		default:
			http.Error(w, "input queue full; try again", 429)
		}
	})
	mux.HandleFunc("/api/compute", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			Target int   `json:"target"`
			Paused *bool `json:"paused"`
		}
		if err := decode(r, &v); err != nil {
			apiError(w, err)
			return
		}
		if v.Paused != nil {
			e.paused.Store(*v.Paused)
		}
		if v.Target > 0 {
			e.computeTarget.Store(int64(maxInt(5, minInt(100, v.Target))))
		}
		e.persist()
		w.WriteHeader(204)
	})
	mux.HandleFunc("/api/stop", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204); go e.Stop() })
	mux.HandleFunc("/api/upload", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name == "" {
			name = "input.bin"
		}
		mime := r.Header.Get("Content-Type")
		a, err := e.storeObject(r.Body, name, mime, "user upload")
		if err != nil {
			apiError(w, err)
			return
		}
		p, _ := e.objectPath(a.ID)
		select {
		case e.ingestQueue <- p:
		default:
			e.addEvent("QUEUE", "Saved input; ingestion queue is full")
		}
		e.persist()
		jsonReply(w, a)
	})
	mux.HandleFunc("/api/asset", func(w http.ResponseWriter, r *http.Request) {
		p, err := e.objectPath(r.URL.Query().Get("id"))
		if err != nil {
			apiError(w, err)
			return
		}
		w.Header().Set("Content-Disposition", "attachment")
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, p)
	})
	mux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			Goal    *string  `json:"goal"`
			AutoUI  *bool    `json:"auto_ui"`
			Track   *bool    `json:"track"`
			QuotaGB *int64   `json:"quota_gb"`
			Pins    []string `json:"pins"`
		}
		if err := decode(r, &v); err != nil {
			apiError(w, err)
			return
		}
		if v.QuotaGB != nil && (*v.QuotaGB < 1 || *v.QuotaGB > 65536) {
			apiError(w, errors.New("quota must be 1–65536 GiB"))
			return
		}
		e.mu.Lock()
		if v.Goal != nil && len(*v.Goal) < 2000 {
			e.lab.Goal = *v.Goal
		}
		if v.AutoUI != nil {
			e.lab.AutoUI = *v.AutoUI
		}
		if v.Track != nil {
			e.lab.Track = *v.Track
			if !*v.Track {
				e.lab.Usage = map[string]Usage{}
				for id, trial := range e.lab.Trials {
					trial.Samples, trial.TotalMS, trial.Errors = 0, 0, 0
					e.lab.Trials[id] = trial
				}
			}
		}
		if v.QuotaGB != nil {
			e.lab.QuotaBytes = *v.QuotaGB << 30
		}
		if v.Pins != nil {
			e.lab.Pins = []string{}
			for _, id := range v.Pins {
				for _, known := range panelIDs {
					if id == known {
						e.lab.Pins = append(e.lab.Pins, id)
						break
					}
				}
			}
		}
		e.mu.Unlock()
		e.persist()
		w.WriteHeader(204)
	})
	mux.HandleFunc("/api/snapshot", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			Name   string `json:"name"`
			ID     string `json:"id"`
			Action string `json:"action"`
		}
		if err := decode(r, &v); err != nil {
			apiError(w, err)
			return
		}
		if v.Action == "save" {
			s, err := e.checkpoint(v.Name)
			if err != nil {
				apiError(w, err)
				return
			}
			jsonReply(w, s)
			return
		}
		if err := e.snapshotAction(v.ID, v.Action, v.Name); err != nil {
			apiError(w, err)
			return
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("/api/ui/new", func(w http.ResponseWriter, r *http.Request) {
		e.mu.Lock()
		e.uiCandidates = nil
		e.mu.Unlock()
		e.generateUICandidates()
		w.WriteHeader(204)
	})
	mux.HandleFunc("/api/ui/action", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			ID     string `json:"id"`
			Action string `json:"action"`
		}
		if err := decode(r, &v); err != nil {
			apiError(w, err)
			return
		}
		if v.Action == "promote" {
			if err := e.promote(v.ID); err != nil {
				apiError(w, err)
				return
			}
		} else {
			e.mu.Lock()
			for i := range e.uiCandidates {
				if e.uiCandidates[i].ID == v.ID {
					if v.Action == "prefer" {
						e.uiCandidates[i].Preferred++
					} else if v.Action == "reject" {
						e.uiCandidates[i].Rejected++
					}
				}
			}
			e.mu.Unlock()
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("/api/ui/reorder", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			Order []string `json:"order"`
		}
		if err := decode(r, &v); err != nil {
			apiError(w, err)
			return
		}
		seen := map[string]bool{}
		for _, id := range v.Order {
			for _, known := range panelIDs {
				if id == known {
					seen[id] = true
				}
			}
		}
		if len(seen) != len(panelIDs) || len(v.Order) != len(panelIDs) {
			apiError(w, errors.New("layout must contain every panel once"))
			return
		}
		e.mu.Lock()
		e.ui.Order = append([]string{}, v.Order...)
		e.ui.ID = "manual-" + randomID()[:8]
		e.uiCandidates = nil
		e.mu.Unlock()
		e.persist()
		w.WriteHeader(204)
	})
	mux.HandleFunc("/api/ui/test", func(w http.ResponseWriter, r *http.Request) {
		var t Trial
		if err := decode(r, &t); err != nil {
			apiError(w, err)
			return
		}
		e.mu.Lock()
		valid := t.ID == e.ui.ID
		for _, g := range e.uiCandidates {
			if g.ID == t.ID {
				valid = true
			}
		}
		if !valid {
			e.mu.Unlock()
			http.Error(w, "candidate expired", 409)
			return
		}
		prev := e.lab.Trials[t.ID]
		t.Samples = prev.Samples
		t.TotalMS = prev.TotalMS
		t.Errors = prev.Errors
		t.Pass = checkTrial(t)
		t.Tested = now()
		e.lab.Trials[t.ID] = t
		e.mu.Unlock()
		jsonReply(w, t)
	})
	mux.HandleFunc("/api/ui/task", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			ID     string  `json:"id"`
			MS     float64 `json:"ms"`
			Errors int     `json:"errors"`
			Task   string  `json:"task"`
		}
		if err := decode(r, &v); err != nil {
			apiError(w, err)
			return
		}
		if v.MS < 100 || v.MS > 120000 || v.Errors < 0 || v.Task != "find-review" {
			apiError(w, errors.New("invalid task measurement"))
			return
		}
		e.mu.Lock()
		if !e.lab.Track {
			e.mu.Unlock()
			apiError(w, errors.New("interaction measurement is disabled in Settings"))
			return
		}
		t, ok := e.lab.Trials[v.ID]
		if ok {
			t.Samples++
			t.TotalMS += v.MS
			t.Errors += v.Errors
			t.Task = v.Task
			e.lab.Trials[v.ID] = t
		}
		e.mu.Unlock()
		if !ok {
			apiError(w, errors.New("run layout checks first"))
			return
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("/api/usage", func(w http.ResponseWriter, r *http.Request) {
		var list []Usage
		if err := decode(r, &list); err != nil {
			apiError(w, err)
			return
		}
		if len(list) > 100 {
			apiError(w, errors.New("too many aggregates"))
			return
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.lab.Track {
			for _, u := range list {
				if !validID(u.Kind) || !validID(u.Target) || len(u.Device) > 20 || u.Count < 0 || u.Count > 10000 || u.TotalMS < 0 || u.TotalMS > 3600000 {
					continue
				}
				key := u.UI + ":" + u.Target + ":" + u.Kind + ":" + u.Device + fmt.Sprint(u.Width/200)
				if len(e.lab.Usage) >= 1000 {
					if _, ok := e.lab.Usage[key]; !ok {
						continue
					}
				}
				old := e.lab.Usage[key]
				u.Count += old.Count
				u.TotalMS += old.TotalMS
				u.Errors += old.Errors
				e.lab.Usage[key] = u
			}
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("/api/active", func(w http.ResponseWriter, r *http.Request) {
		e.interactionUntil.Store(time.Now().Add(8 * time.Second).UnixMilli())
		w.WriteHeader(204)
	})
	mux.HandleFunc("/api/block", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			ID     string            `json:"id"`
			Values map[string]string `json:"values"`
		}
		if err := decode(r, &v); err != nil {
			apiError(w, err)
			return
		}
		e.mu.RLock()
		var block *UIBlock
		for _, b := range e.lab.Discovered {
			if b.ID == v.ID {
				c := b
				block = &c
			}
		}
		e.mu.RUnlock()
		if block == nil {
			apiError(w, errors.New("unknown block"))
			return
		}
		clean := map[string]string{"block": block.ID}
		for _, f := range block.Fields {
			value := v.Values[f.ID]
			if len(value) > 8000 {
				apiError(w, errors.New("field too long"))
				return
			}
			if f.Required && strings.TrimSpace(value) == "" {
				apiError(w, errors.New(f.Label+" is required"))
				return
			}
			clean[f.Label] = value
		}
		b, _ := json.Marshal(clean)
		select {
		case e.observeQueue <- string(b):
		default:
			http.Error(w, "queue full", 429)
			return
		}
		e.mu.Lock()
		e.lab.Answers = append(e.lab.Answers, clean)
		if len(e.lab.Answers) > 300 {
			e.lab.Answers = e.lab.Answers[len(e.lab.Answers)-300:]
		}
		e.mu.Unlock()
		e.persist()
		w.WriteHeader(204)
	})
	mux.HandleFunc("/api/approval/action", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			ID     string `json:"id"`
			Action string `json:"action"`
		}
		if err := decode(r, &v); err != nil {
			apiError(w, err)
			return
		}
		if v.Action != "approve" && v.Action != "reject" {
			apiError(w, errors.New("invalid decision"))
			return
		}
		e.mu.Lock()
		for i := range e.approvals {
			if e.approvals[i].ID == v.ID && e.approvals[i].Status == "waiting" {
				if v.Action == "approve" {
					e.approvals[i].Status = "recorded (configure provider to connect)"
				} else {
					e.approvals[i].Status = "rejected"
				}
			}
		}
		e.mu.Unlock()
		e.persist()
		w.WriteHeader(204)
	})
	mux.HandleFunc("/api/export", e.exportBundle)
	e.mediaRoutes(mux)
	e.learningRoutes(mux)
	e.evolutionRoutes(mux)
	e.nativeImageRoutes(mux)
	e.conceptRoutes(mux)
	e.contributionRoutes(mux)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' blob: data:; media-src 'self' blob:; connect-src 'self'; frame-src 'self' blob:; object-src 'none'; base-uri 'none'; frame-ancestors 'self'")
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		if host != "127.0.0.1" && host != "localhost" && host != "[::1]" && host != "::1" {
			http.Error(w, "local host required", 403)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			if r.Header.Get("X-Origin-Key") != e.sessionKey {
				http.Error(w, "Open the address printed by ORIGIN-0 to unlock this local session.", 401)
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" {
				u, err := url.Parse(origin)
				if err != nil || u.Host != r.Host {
					http.Error(w, "same origin required", 403)
					return
				}
			}
			read := r.URL.Path == "/api/state" || r.URL.Path == "/api/asset" || r.URL.Path == "/api/export" || r.URL.Path == "/api/provider" && r.Method == "GET"
			if !read && r.Method != "POST" {
				http.Error(w, "POST required", 405)
				return
			}
			if read && r.Method != "GET" {
				http.Error(w, "GET required", 405)
				return
			}
			limit := int64(4 << 20)
			if r.URL.Path == "/api/upload" {
				limit = 16 << 30
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
		}
		mux.ServeHTTP(w, r)
	})
}
func lenCopy(e *Engine, k string) int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if k == "concepts" {
		return len(e.concepts)
	}
	return len(e.hypotheses)
}
func (e *Engine) exportBundle(w http.ResponseWriter, r *http.Request) {
	b, err := e.stateBytes()
	var exported PersistState
	_ = json.Unmarshal(b, &exported)
	exported.Lab.Assets = nil
	exported.Lab.Snapshots = nil
	exported.Lab.StoredBytes = 0
	exported.Lab.ObjectCount = 0
	b, _ = json.Marshal(exported)
	if err != nil {
		apiError(w, err)
		return
	}
	exe, err := os.Executable()
	if err != nil {
		apiError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=ORIGIN0_SAVED_BRANCH.zip")
	z := zip.NewWriter(w)
	defer z.Close()
	add := func(name string, r io.Reader) error {
		dst, err := z.Create(name)
		if err != nil {
			return err
		}
		_, err = io.Copy(dst, r)
		return err
	}
	f, err := os.Open(exe)
	if err == nil {
		defer f.Close()
		name := filepath.Base(exe)
		_ = add(name, f)
	}
	_ = add("origin0_data/state.json", bytes.NewReader(b))
	_ = add("SOURCE_SNAPSHOT.txt", strings.NewReader(e.sourceSnapshot))
	_ = add("READ_FIRST.txt", strings.NewReader("ORIGIN-0 v1.5 saved branch\nRun the included program from an extracted folder.\nThis contains the current executable and bounded working state.\nLarge raw objects, previous snapshots, credentials and grants are excluded.\nFull recovery requires a separate copy of your original origin0_data folder.\nThis export preserves the same executable; it is not a newly compiled intelligence.\n"))
}

func (e *Engine) acquireInstance() (func(), bool) {
	path := filepath.Join(e.dataDir, "instance.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		b, _ := os.ReadFile(path)
		var old struct {
			Address string `json:"address"`
			Key     string `json:"key"`
		}
		_ = json.Unmarshal(b, &old)
		host, _, parseErr := net.SplitHostPort(old.Address)
		if parseErr == nil && host == "127.0.0.1" {
			client := http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("no redirects") }}
			if res, err := client.Get("http://" + old.Address + "/health"); err == nil {
				res.Body.Close()
				if res.StatusCode == 200 {
					if os.Getenv("ORIGIN0_NO_BROWSER") == "" {
						openBrowser("http://" + old.Address + "/#" + old.Key)
					}
					fmt.Println("ORIGIN-0 is already running for this data folder.")
					return func() {}, false
				}
			}
		}
		info, statErr := os.Stat(path)
		if statErr == nil && time.Since(info.ModTime()) < 5*time.Second {
			fmt.Println("Another ORIGIN-0 instance is starting. Try again in a moment.")
			return func() {}, false
		}
		if err = os.Remove(path); err != nil {
			fmt.Println("Cannot acquire data-folder lock:", err)
			return func() {}, false
		}
		f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return func() {}, false
		}
	}
	data, _ := json.Marshal(map[string]string{"address": e.instanceAddr, "key": e.sessionKey})
	if _, err = f.Write(data); err != nil {
		f.Close()
		return func() {}, false
	}
	f.Close()
	return func() { _ = os.Remove(path) }, true
}
