package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

// Labels are explicit human evidence, never predictions fed back as truth.
type TrainingExample struct {
	Asset    string    `json:"asset"`
	Label    string    `json:"label"`
	Caption  string    `json:"caption"`
	Group    string    `json:"group"`
	Split    string    `json:"split"`
	Features []float64 `json:"features"`
	Kind     string    `json:"kind"`
}
type RecognitionModel struct {
	ID       string            `json:"id"`
	Method   string            `json:"method"`
	Metric   string            `json:"metric"`
	Revision int               `json:"revision"`
	Samples  []TrainingExample `json:"samples"`
	Created  int64             `json:"created"`
}
type ModelScore struct {
	Method           string                    `json:"method"`
	Metric           string                    `json:"metric"`
	Correct          int                       `json:"correct"`
	Total            int                       `json:"total"`
	Accuracy         float64                   `json:"accuracy"`
	BalancedAccuracy float64                   `json:"balanced_accuracy"`
	Confusion        map[string]map[string]int `json:"confusion"`
}
type LearningRun struct {
	ID         string       `json:"id"`
	Revision   int          `json:"revision"`
	Created    int64        `json:"created"`
	Candidates []ModelScore `json:"candidates"`
	Baseline   ModelScore   `json:"baseline"`
	Validation ModelScore   `json:"validation"`
	Audit      ModelScore   `json:"audit"`
	Promoted   bool         `json:"promoted"`
	Reason     string       `json:"reason"`
}
type LearningState struct {
	Version       int               `json:"version"`
	Auto          bool              `json:"auto"`
	Revision      int               `json:"revision"`
	LastEvaluated int               `json:"last_evaluated"`
	Examples      []TrainingExample `json:"examples"`
	Active        RecognitionModel  `json:"active"`
	Previous      RecognitionModel  `json:"previous"`
	Runs          []LearningRun     `json:"runs"`
	Note          string            `json:"note"`
}

func (e *Engine) initLearning() {
	if e.lab.Learning.Version == 0 {
		e.lab.Learning = LearningState{Version: 1, Auto: true, Note: "Teach at least two categories with five independent example groups each: three training, one validation, one audit. More varied examples make the result more useful."}
	}
}

// Fixed 64 features: bounded byte statistics, RGB distributions and coarse geometry.
// This is a small learnable classifier, not a pretrained semantic vision model.
func extractFeatures(path string) ([]float64, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 1<<20))
	if err != nil {
		return nil, "", err
	}
	if len(b) == 0 {
		return nil, "", errors.New("empty files have no training signal")
	}
	v := make([]float64, 64)
	for _, x := range b {
		v[int(x)/8]++
	}
	for i := 0; i < 32; i++ {
		v[i] /= float64(len(b))
	}
	cfg, _, de := image.DecodeConfig(bytes.NewReader(b))
	if de != nil {
		v[63] = 0
		return v, "bytes", nil
	}
	if cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width)*int64(cfg.Height) > 16_000_000 {
		return nil, "", errors.New("training images must have at most 16 million pixels; supply a resized example")
	}
	if _, err = f.Seek(0, 0); err != nil {
		return nil, "", err
	}
	im, _, err := image.Decode(io.LimitReader(f, 32<<20))
	if err != nil {
		return nil, "", fmt.Errorf("image decode: %w", err)
	}
	bounds := im.Bounds()
	count := 0.0
	edge := 0.0
	sx, sy := maxInt(1, bounds.Dx()/96), maxInt(1, bounds.Dy()/96)
	for y := bounds.Min.Y; y < bounds.Max.Y; y += sy {
		last := -1.0
		for x := bounds.Min.X; x < bounds.Max.X; x += sx {
			r, g, bl, _ := im.At(x, y).RGBA()
			vals := []uint32{r, g, bl}
			for c, z := range vals {
				v[32+c*8+minInt(7, int(z>>13))]++
			}
			lum := (float64(r) + float64(g) + float64(bl)) / (3 * 65535)
			v[56] += lum
			v[57] += lum * lum
			if last >= 0 {
				edge += math.Abs(lum - last)
			}
			last = lum
			count++
		}
	}
	for i := 32; i <= 57; i++ {
		v[i] /= count
	}
	v[58] = edge / count
	v[59] = float64(bounds.Dx()) / float64(bounds.Dx()+bounds.Dy())
	v[60] = math.Min(1, float64(bounds.Dx())/4096)
	v[61] = math.Min(1, float64(bounds.Dy())/4096)
	v[62] = math.Sqrt(math.Max(0, v[57]-v[56]*v[56]))
	v[63] = 1
	return v, "image", nil
}
func cloneLearning(x LearningState) LearningState {
	b, _ := json.Marshal(x)
	var n LearningState
	_ = json.Unmarshal(b, &n)
	return n
}
func (e *Engine) labelAsset(asset, label, caption, group string) error {
	label, caption, group = strings.TrimSpace(label), strings.TrimSpace(caption), strings.TrimSpace(group)
	if label == "" || len(label) > 80 || len(caption) > 2000 || len(group) > 100 {
		return errors.New("enter a category (1–80 characters), optional caption (up to 2000), and optional example-group name")
	}
	path, err := e.objectPath(asset)
	if err != nil {
		return err
	}
	v, kind, err := extractFeatures(path)
	if err != nil {
		return err
	}
	if group == "" {
		group = asset
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	ls := &e.lab.Learning
	idx := -1
	role := ""
	groups := map[string]bool{}
	for i, x := range ls.Examples {
		if x.Asset != asset && featureFingerprint(x.Features) == featureFingerprint(v) {
			if x.Label != label {
				return errors.New("this example has the same measured features as a different category; supply a more distinguishable example")
			}
			group = x.Group
			role = x.Split
		}
		if x.Asset == asset {
			idx = i
			role = x.Split
			group = x.Group
		}
		if x.Label == label {
			groups[x.Group] = true
		}
		if x.Group == group && x.Asset != asset {
			if x.Label != label {
				return errors.New("examples from one group must share a category")
			}
			role = x.Split
		}
	}
	if idx < 0 && len(ls.Examples) >= 2000 {
		return errors.New("active teaching set is limited to 2000 examples; save a branch before beginning a new teaching set")
	}
	if role == "" {
		role = []string{"train", "train", "train", "validation", "audit"}[len(groups)%5]
	}
	example := TrainingExample{Asset: asset, Label: label, Caption: caption, Group: group, Split: role, Features: v, Kind: kind}
	if idx >= 0 {
		old := ls.Examples[idx]
		if old.Label == label && old.Caption == caption {
			return nil
		}
		ls.Examples[idx] = example
	} else {
		ls.Examples = append(ls.Examples, example)
	}
	ls.Revision++
	ls.Note = "New teaching evidence received. The next learning cycle will compare candidates."
	return nil
}
func featureDistance(a, b []float64, metric string) float64 {
	if len(a) != 64 || len(b) != 64 {
		return math.Inf(1)
	}
	if metric == compiledRecognitionMetric {
		return compiledFeatureDistance(a, b)
	}
	return referenceDistance(a, b, metric)
}
func referenceDistance(a, b []float64, metric string) float64 {
	sum := 0.0
	for i := range a {
		weight := 1.0
		// Use appearance for decoded images; compressed bytes do not describe objects.
		if a[63] > 0.5 && b[63] > 0.5 && i < 32 {
			weight = 0.02
		}
		d := math.Abs(a[i] - b[i])
		if metric == "l1" {
			sum += weight * d
		} else {
			sum += weight * d * d
		}
	}
	return sum
}
func modelPredict(m RecognitionModel, v []float64) (string, float64) {
	if len(m.Samples) == 0 {
		return "", 0
	}
	labels := map[string]float64{}
	if m.Method == "centroid" {
		sums := map[string][]float64{}
		counts := map[string]int{}
		for _, s := range m.Samples {
			if len(s.Features) != len(v) {
				continue
			}
			if sums[s.Label] == nil {
				sums[s.Label] = make([]float64, len(v))
			}
			counts[s.Label]++
			for i, x := range s.Features {
				sums[s.Label][i] += x
			}
		}
		for label, a := range sums {
			for i := range a {
				a[i] /= float64(counts[label])
			}
			labels[label] = featureDistance(a, v, m.Metric)
		}
	} else {
		type neighbour struct {
			label string
			d     float64
			id    string
		}
		ns := []neighbour{}
		for _, s := range m.Samples {
			ns = append(ns, neighbour{s.Label, featureDistance(s.Features, v, m.Metric), s.Asset})
		}
		sort.Slice(ns, func(i, j int) bool {
			if ns[i].d == ns[j].d {
				return ns[i].id < ns[j].id
			}
			return ns[i].d < ns[j].d
		})
		k := 1
		if m.Method == "knn3" {
			k = 3
		}
		k = minInt(k, len(ns))
		votes := map[string]float64{}
		for _, n := range ns[:k] {
			votes[n.label] += 1 / (1 + n.d)
		}
		best := ""
		score := -1.0
		for l, s := range votes {
			if s > score || s == score && l < best {
				best, score = l, s
			}
		}
		return best, ns[0].d
	}
	best := ""
	d := math.Inf(1)
	for label, dist := range labels {
		if dist < d || dist == d && label < best {
			best, d = label, dist
		}
	}
	return best, d
}
func scoreRecognition(m RecognitionModel, examples []TrainingExample) ModelScore {
	s := ModelScore{Method: m.Method, Metric: m.Metric, Confusion: map[string]map[string]int{}}
	counts, hits := map[string]int{}, map[string]int{}
	for _, x := range examples {
		predicted, _ := modelPredict(m, x.Features)
		s.Total++
		counts[x.Label]++
		if s.Confusion[x.Label] == nil {
			s.Confusion[x.Label] = map[string]int{}
		}
		s.Confusion[x.Label][predicted]++
		if predicted == x.Label {
			s.Correct++
			hits[x.Label]++
		}
	}
	if s.Total > 0 {
		s.Accuracy = float64(s.Correct) / float64(s.Total)
	}
	for l, n := range counts {
		s.BalancedAccuracy += float64(hits[l]) / float64(n)
	}
	if len(counts) > 0 {
		s.BalancedAccuracy /= float64(len(counts))
	}
	return s
}
func learningSets(ls LearningState) ([]TrainingExample, []TrainingExample, []TrainingExample, error) {
	train, val, audit := []TrainingExample{}, []TrainingExample{}, []TrainingExample{}
	groups := map[string]map[string]map[string]bool{}
	for _, x := range ls.Examples {
		if len(x.Features) != 64 {
			return nil, nil, nil, errors.New("invalid feature vector in teaching state")
		}
		if groups[x.Label] == nil {
			groups[x.Label] = map[string]map[string]bool{"train": {}, "validation": {}, "audit": {}}
		}
		if groups[x.Label][x.Split] == nil {
			return nil, nil, nil, errors.New("invalid teaching split")
		}
		groups[x.Label][x.Split][x.Group] = true
		switch x.Split {
		case "train":
			train = append(train, x)
		case "validation":
			val = append(val, x)
		case "audit":
			audit = append(audit, x)
		}
	}
	if len(groups) < 2 {
		return nil, nil, nil, errors.New("teach at least two different categories")
	}
	needs := []string{}
	for label, g := range groups {
		if len(g["train"]) < 3 || len(g["validation"]) < 1 || len(g["audit"]) < 1 {
			needs = append(needs, fmt.Sprintf("%s: %d/3 training, %d/1 validation, %d/1 audit groups", label, len(g["train"]), len(g["validation"]), len(g["audit"])))
		}
	}
	sort.Strings(needs)
	if len(needs) > 0 {
		return nil, nil, nil, errors.New("more independent examples needed — " + strings.Join(needs, "; "))
	}
	return train, val, audit, nil
}
func (e *Engine) trainRecognition() (LearningRun, error) {
	e.learningMu.Lock()
	defer e.learningMu.Unlock()
	e.mu.RLock()
	ls := cloneLearning(e.lab.Learning)
	e.mu.RUnlock()
	var run LearningRun
	if e.paused.Load() {
		return run, errors.New("resume the engine to run learning")
	}
	if ls.LastEvaluated == ls.Revision && len(ls.Runs) > 0 {
		return ls.Runs[len(ls.Runs)-1], nil
	}
	train, val, audit, err := learningSets(ls)
	if err != nil {
		e.mu.Lock()
		e.lab.Learning.Note = err.Error()
		e.mu.Unlock()
		return run, err
	}
	run = LearningRun{ID: randomID()[:16], Revision: ls.Revision, Created: now(), Baseline: scoreRecognition(ls.Active, val)}
	var winner RecognitionModel
	best := -1.0
	for _, method := range []string{"centroid", "knn1", "knn3"} {
		for _, metric := range []string{"l1", "l2"} {
			m := RecognitionModel{ID: randomID()[:16], Method: method, Metric: metric, Revision: ls.Revision, Samples: train, Created: now()}
			s := scoreRecognition(m, val)
			run.Candidates = append(run.Candidates, s)
			if s.BalancedAccuracy > best {
				best = s.BalancedAccuracy
				winner = m
				run.Validation = s
			}
		}
	}
	// Audit is reported after candidate selection, never used to select/promote a model.
	run.Audit = scoreRecognition(winner, audit)
	run.Promoted = best > run.Baseline.BalancedAccuracy+1e-9 && best >= 0.6
	run.Reason = "Incumbent retained: no validation improvement. Supply fresh, varied examples."
	if run.Promoted {
		run.Reason = "Candidate improved balanced validation accuracy. Prior model retained for rollback. Audit results are a small-sample estimate, not proof of general understanding."
	}
	e.mu.Lock()
	if e.lab.Learning.Revision != ls.Revision {
		e.mu.Unlock()
		return run, errors.New("teaching changed during evaluation; rerun on the new revision")
	}
	e.lab.Learning.LastEvaluated = ls.Revision
	e.lab.Learning.Note = run.Reason
	if run.Promoted {
		e.lab.Learning.Previous = e.lab.Learning.Active
		e.lab.Learning.Active = winner
	}
	e.lab.Learning.Runs = append(e.lab.Learning.Runs, run)
	if len(e.lab.Learning.Runs) > 40 {
		e.lab.Learning.Runs = e.lab.Learning.Runs[len(e.lab.Learning.Runs)-40:]
	}
	e.mu.Unlock()
	e.persist()
	e.addEvent("LEARNING", fmt.Sprintf("revision %d: %d candidates, validation %.0f%%, audit %.0f%%, promoted=%t", ls.Revision, len(run.Candidates), 100*run.Validation.BalancedAccuracy, 100*run.Audit.BalancedAccuracy, run.Promoted))
	if run.Promoted {
		e.maybeAutoBuild()
	}
	return run, nil
}
func (e *Engine) learningLoop() {
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-e.stop:
			return
		case <-tick.C:
			e.mu.RLock()
			wanted := e.lab.Learning.Auto && e.lab.Learning.Revision > e.lab.Learning.LastEvaluated
			e.mu.RUnlock()
			if wanted && !e.paused.Load() {
				_, _ = e.trainRecognition()
			}
			if !e.paused.Load() {
				e.maybeAutoTrain()
			}
		}
	}
}
func (e *Engine) learningRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/learning/state", func(w http.ResponseWriter, r *http.Request) {
		e.mu.RLock()
		ls := cloneLearning(e.lab.Learning)
		e.mu.RUnlock()
		for i := range ls.Examples {
			ls.Examples[i].Features = nil
		}
		ls.Active.Samples = nil
		ls.Previous.Samples = nil
		jsonReply(w, ls)
	})
	mux.HandleFunc("/api/learning/label", func(w http.ResponseWriter, r *http.Request) {
		var x struct{ Asset, Label, Caption, Group string }
		if err := decode(r, &x); err != nil {
			apiError(w, err)
			return
		}
		if err := e.labelAsset(x.Asset, x.Label, x.Caption, x.Group); err != nil {
			apiError(w, err)
			return
		}
		e.persist()
		w.WriteHeader(204)
	})
	mux.HandleFunc("/api/learning/train", func(w http.ResponseWriter, r *http.Request) {
		out, err := e.trainRecognition()
		if err != nil {
			apiError(w, err)
			return
		}
		jsonReply(w, out)
	})
	mux.HandleFunc("/api/learning/predict", func(w http.ResponseWriter, r *http.Request) {
		var x struct{ Asset string }
		if err := decode(r, &x); err != nil {
			apiError(w, err)
			return
		}
		p, err := e.objectPath(x.Asset)
		if err != nil {
			apiError(w, err)
			return
		}
		v, kind, err := extractFeatures(p)
		if err != nil {
			apiError(w, err)
			return
		}
		e.mu.RLock()
		m := e.lab.Learning.Active
		label, distance := modelPredict(m, v)
		e.mu.RUnlock()
		if label == "" {
			apiError(w, errors.New("no validated recognition model yet; teach examples first"))
			return
		}
		jsonReply(w, map[string]any{"label": label, "distance": distance, "model": m.ID, "kind": kind, "note": "Feature similarity prediction; distance is not calibrated confidence. Confirm or correct it."})
	})
	mux.HandleFunc("/api/learning/settings", func(w http.ResponseWriter, r *http.Request) {
		var x struct{ Auto bool }
		if err := decode(r, &x); err != nil {
			apiError(w, err)
			return
		}
		e.mu.Lock()
		e.lab.Learning.Auto = x.Auto
		e.mu.Unlock()
		e.persist()
		w.WriteHeader(204)
	})
	mux.HandleFunc("/api/learning/rollback", func(w http.ResponseWriter, r *http.Request) {
		e.learningMu.Lock()
		defer e.learningMu.Unlock()
		e.mu.Lock()
		if e.lab.Learning.Previous.ID == "" {
			e.mu.Unlock()
			apiError(w, errors.New("no previous trained model to restore"))
			return
		}
		e.lab.Learning.Active, e.lab.Learning.Previous = e.lab.Learning.Previous, e.lab.Learning.Active
		e.lab.Learning.Auto = false
		e.lab.Learning.Note = "Prior model restored. Automatic learning paused until you enable it."
		e.mu.Unlock()
		e.persist()
		w.WriteHeader(204)
	})
}

func featureFingerprint(v []float64) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Called under the state lock. Keep current labels on conflicts; never replay a
// source branch's trained weights or count one asset twice.
func (e *Engine) mergeLearningLocked(incoming LearningState) {
	known := map[string]bool{}
	groups := map[string]TrainingExample{}
	for _, x := range e.lab.Learning.Examples {
		known[x.Asset] = true
		groups[x.Group] = x
	}
	added := 0
	for _, x := range incoming.Examples {
		if known[x.Asset] || len(x.Features) != 64 || len(e.lab.Learning.Examples) >= 2000 {
			continue
		}
		if old, ok := groups[x.Group]; ok {
			if old.Label != x.Label {
				continue
			}
			x.Split = old.Split
		}
		if x.Split != "train" && x.Split != "validation" && x.Split != "audit" {
			continue
		}
		e.lab.Learning.Examples = append(e.lab.Learning.Examples, x)
		known[x.Asset] = true
		groups[x.Group] = x
		added++
	}
	if added > 0 {
		e.lab.Learning.Revision++
		e.lab.Learning.Note = fmt.Sprintf("Merged %d new teaching examples. Evaluate them before changing the active model.", added)
	}
}
