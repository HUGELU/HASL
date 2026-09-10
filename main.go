package main

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"math"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

//go:embed web/* worker/* bundled/* model_catalog.json studio_catalog.json comfy_runtimes.json upscale_manifest.json source_snapshot.txt source_bundle.json LOCAL_MODELS.md
var assets embed.FS

type Concept struct {
	ID        string  `json:"id"`
	Code      string  `json:"code"`
	Count     int     `json:"count"`
	Utility   float64 `json:"utility"`
	FirstSeen int64   `json:"first_seen"`
	LastSeen  int64   `json:"last_seen"`
}

type Relation struct {
	A          string  `json:"a"`
	B          string  `json:"b"`
	Count      int     `json:"count"`
	Confidence float64 `json:"confidence"`
	Indirect   bool    `json:"indirect"`
}

type Hypothesis struct {
	Inferences     int     `json:"inferences"`
	ID             string  `json:"id"`
	A              string  `json:"a"`
	B              string  `json:"b"`
	Support        int     `json:"support"`
	Contradictions int     `json:"contradictions"`
	Confidence     float64 `json:"confidence"`
	Indirect       bool    `json:"indirect"`
	LastTest       int64   `json:"last_test"`
	Symbolic       string  `json:"symbolic"`
}

type Question struct {
	ID       string  `json:"id"`
	Symbolic string  `json:"symbolic"`
	Human    string  `json:"human"`
	Priority float64 `json:"priority"`
	Created  int64   `json:"created"`
}

type Experience struct {
	SampleBytes int      `json:"sample_bytes"`
	HashScope   string   `json:"hash_scope"`
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Size        int64    `json:"size"`
	Hash        string   `json:"hash"`
	Seen        int64    `json:"seen"`
	TopConcepts []string `json:"top_concepts"`
	Source      string   `json:"source"`
}

type LanguageToken struct {
	Motif   string  `json:"motif"`
	Code    string  `json:"code"`
	Count   int     `json:"count"`
	Utility float64 `json:"utility"`
}

type Event struct {
	Time   int64  `json:"time"`
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

type SwarmProfile struct {
	Explore     float64 `json:"explore"`
	Skepticism  float64 `json:"skepticism"`
	Compression float64 `json:"compression"`
	Sociality   float64 `json:"sociality"`
	Risk        float64 `json:"risk"`
	Mutation    float64 `json:"mutation"`
}

type Swarm struct {
	Memory          map[string]float64 `json:"memory"`
	EvidenceSeen    int                `json:"evidence_seen"`
	ID              string             `json:"id"`
	Generation      int                `json:"generation"`
	Population      int                `json:"population"`
	Alive           int                `json:"alive"`
	BestFitness     float64            `json:"best_fitness"`
	MeanFitness     float64            `json:"mean_fitness"`
	Diversity       float64            `json:"diversity"`
	Consensus       float64            `json:"consensus"`
	Dissent         float64            `json:"dissent"`
	Entropy         float64            `json:"entropy"`
	LocalHypotheses int                `json:"local_hypotheses"`
	Discoveries     int                `json:"discoveries"`
	Exchanges       int                `json:"exchanges"`
	Coalitions      int                `json:"coalitions"`
	Deaths          int                `json:"deaths"`
	Rebirths        int                `json:"rebirths"`
	Extinctions     int                `json:"extinctions"`
	Focus           string             `json:"focus"`
	AncestralScar   string             `json:"ancestral_scar"`
	Profile         SwarmProfile       `json:"profile"`
}

type UIGenome struct {
	Blocks     []UIBlock `json:"blocks"`
	ID         string    `json:"id"`
	Generation int       `json:"generation"`
	Name       string    `json:"name"`
	Topology   string    `json:"topology"`
	Columns    int       `json:"columns"`
	Density    float64   `json:"density"`
	Radius     int       `json:"radius"`
	MotionMS   int       `json:"motion_ms"`
	RefreshMS  int       `json:"refresh_ms"`
	Order      []string  `json:"order"`
	Score      float64   `json:"score"`
	Rationale  string    `json:"rationale"`
	Preferred  int       `json:"preferred"`
	Rejected   int       `json:"rejected"`
}

type EngineGenome struct {
	ID                  string  `json:"id"`
	Generation          int     `json:"generation"`
	InferenceDepth      int     `json:"inference_depth"`
	NoveltyWeight       float64 `json:"novelty_weight"`
	Skepticism          float64 `json:"skepticism"`
	QuestionSensitivity float64 `json:"question_sensitivity"`
	SwarmMigration      float64 `json:"swarm_migration"`
	CompressionPressure float64 `json:"compression_pressure"`
	Score               float64 `json:"score"`
}

type Reflection struct {
	Time        int64   `json:"time"`
	Observation string  `json:"observation"`
	Proposal    string  `json:"proposal"`
	Confidence  float64 `json:"confidence"`
	SourceLevel bool    `json:"source_level"`
}

type Approval struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Request     string   `json:"request"`
	Why         string   `json:"why"`
	Status      string   `json:"status"`
	Created     int64    `json:"created"`
	Reflections []string `json:"reflections"`
}

type Telemetry struct {
	UptimeSec      int64   `json:"uptime_sec"`
	Cycles         uint64  `json:"cycles"`
	CyclesPerSec   float64 `json:"cycles_per_sec"`
	LogicalCPU     int     `json:"logical_cpu"`
	ActiveWorkers  int     `json:"active_workers"`
	ComputeTarget  int     `json:"compute_target"`
	BusyRatio      float64 `json:"busy_ratio"`
	Goroutines     int     `json:"goroutines"`
	HeapMB         float64 `json:"heap_mb"`
	SysMB          float64 `json:"sys_mb"`
	GPU            string  `json:"gpu"`
	GPUBackend     string  `json:"gpu_backend"`
	Backend        string  `json:"backend"`
	LastCheckpoint int64   `json:"last_checkpoint"`
}

type StateView struct {
	Lab              LabState        `json:"lab"`
	Version          string          `json:"version"`
	Telemetry        Telemetry       `json:"telemetry"`
	Concepts         []*Concept      `json:"concepts"`
	Relations        []*Relation     `json:"relations"`
	Hypotheses       []*Hypothesis   `json:"hypotheses"`
	Questions        []Question      `json:"questions"`
	Experiences      []Experience    `json:"experiences"`
	Language         []LanguageToken `json:"language"`
	Swarms           []*Swarm        `json:"swarms"`
	UIGenome         UIGenome        `json:"ui_genome"`
	UICandidates     []UIGenome      `json:"ui_candidates"`
	EngineGenome     EngineGenome    `json:"engine_genome"`
	EngineCandidates []EngineGenome  `json:"engine_candidates"`
	Reflections      []Reflection    `json:"reflections"`
	Approvals        []Approval      `json:"approvals"`
	Events           []Event         `json:"events"`
	Health           map[string]any  `json:"health"`
}

type PersistState struct {
	ComputeTarget int                    `json:"compute_target"`
	Lab           LabState               `json:"lab"`
	Cycles        uint64                 `json:"cycles"`
	Concepts      map[string]*Concept    `json:"concepts"`
	Relations     map[string]*Relation   `json:"relations"`
	Hypotheses    map[string]*Hypothesis `json:"hypotheses"`
	Questions     []Question             `json:"questions"`
	Experiences   []Experience           `json:"experiences"`
	Swarms        []*Swarm               `json:"swarms"`
	UIGenome      UIGenome               `json:"ui_genome"`
	EngineGenome  EngineGenome           `json:"engine_genome"`
	Reflections   []Reflection           `json:"reflections"`
	Approvals     []Approval             `json:"approvals"`
}

type Engine struct {
	mediaStudio         *MediaStudio
	commonVision        *CommonVision
	finishing           *Finishing
	development         *Development
	heavy               chan struct{}
	studio              *ConceptStudio
	images              *NativeImages
	imageBusy           atomic.Bool
	learningMu          sync.Mutex
	jobsMu              sync.Mutex
	jobWG               sync.WaitGroup
	jobCancel           context.CancelFunc
	jobID               string
	jobsClosing         bool
	jobConfig           EvolutionConfig
	lastAutoTraining    int
	wg                  sync.WaitGroup
	lastGraph           uint64
	lab                 LabState
	persistMu           sync.Mutex
	ingestMu            sync.Mutex
	stop                chan struct{}
	stopOnce            sync.Once
	paused              atomic.Bool
	sessionKey          string
	privacy             *PrivacyLock
	instanceAddr        string
	modelMu             sync.Mutex
	provider            Provider
	archiveMu           sync.Mutex
	catalogMu           sync.Mutex
	interactionUntil    atomic.Int64
	mu                  sync.RWMutex
	started             time.Time
	dataDir             string
	dropDir             string
	concepts            map[string]*Concept
	relations           map[string]*Relation
	hypotheses          map[string]*Hypothesis
	questions           []Question
	experiences         []Experience
	swarms              []*Swarm
	ui                  UIGenome
	uiCandidates        []UIGenome
	engineGenome        EngineGenome
	engineCandidates    []EngineGenome
	reflections         []Reflection
	approvals           []Approval
	events              []Event
	seenFiles           map[string]string
	ingestQueue         chan string
	observeQueue        chan string
	cycle               atomic.Uint64
	ops                 atomic.Uint64
	computeTarget       atomic.Int64
	activeWorkers       atomic.Int64
	lastPersist         atomic.Int64
	lastCycleSample     uint64
	lastCycleSampleTime time.Time
	cycleRate           float64
	busyRatio           float64
	gpu                 string
	gpuBackend          string
	sourceSnapshot      string
	rng                 *rand.Rand
}

func now() int64 { return time.Now().Unix() }

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func shortHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])[:12]
}

func motifHash(b []byte) string {
	h := fnv.New64a()
	_, _ = h.Write(b)
	return fmt.Sprintf("m%016x", h.Sum64())
}

func binaryCode(n int) string {
	if n < 1 {
		n = 1
	}
	// Elias-gamma-like prefix-free code for a positive integer.
	bits := strconv.FormatInt(int64(n), 2)
	return strings.Repeat("0", len(bits)-1) + bits
}

func newProfile(r *rand.Rand) SwarmProfile {
	return SwarmProfile{
		Explore:     0.25 + r.Float64()*0.7,
		Skepticism:  0.2 + r.Float64()*0.7,
		Compression: 0.2 + r.Float64()*0.75,
		Sociality:   0.1 + r.Float64()*0.85,
		Risk:        0.1 + r.Float64()*0.8,
		Mutation:    0.01 + r.Float64()*0.09,
	}
}

func newEngineGenome(r *rand.Rand, gen int) EngineGenome {
	return EngineGenome{
		ID:                  fmt.Sprintf("eg-%d-%s", gen, shortHash(fmt.Sprint(r.Int63()))[:6]),
		Generation:          gen,
		InferenceDepth:      2 + r.Intn(3),
		NoveltyWeight:       0.5 + r.Float64(),
		Skepticism:          0.25 + r.Float64()*0.6,
		QuestionSensitivity: 0.35 + r.Float64()*0.6,
		SwarmMigration:      0.05 + r.Float64()*0.35,
		CompressionPressure: 0.3 + r.Float64()*0.7,
	}
}

var panelIDs = []string{"activity", "concepts", "hypotheses", "questions", "language", "swarms", "interface", "self", "approvals", "experiences"}

func newUIGenome(r *rand.Rand, gen int) UIGenome {
	order := append([]string(nil), panelIDs...)
	r.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
	topologies := []string{"grid", "focus", "ribbon", "mosaic"}
	return UIGenome{
		ID:         fmt.Sprintf("ui-%d-%s", gen, shortHash(fmt.Sprint(r.Int63()))[:6]),
		Generation: gen, Name: fmt.Sprintf("Interface %d", gen), Topology: topologies[r.Intn(len(topologies))],
		Columns: 2 + r.Intn(3), Density: 0.78 + r.Float64()*0.45,
		Radius: r.Intn(19), MotionMS: 120 + r.Intn(1000), RefreshMS: 700 + r.Intn(1800),
		Order: order,
	}
}

func NewEngine(base string) *Engine {
	seed := time.Now().UnixNano()
	r := rand.New(rand.NewSource(seed))
	dataDir := filepath.Join(base, "origin0_data")
	dropDir := filepath.Join(base, "DROP_HERE")
	_ = os.MkdirAll(filepath.Join(dataDir, "inbox"), 0755)
	_ = os.MkdirAll(filepath.Join(dataDir, "proposals"), 0755)
	_ = os.MkdirAll(dropDir, 0755)
	src, _ := assets.ReadFile("source_snapshot.txt")
	e := &Engine{
		started: time.Now(), dataDir: dataDir, dropDir: dropDir,
		concepts: map[string]*Concept{}, relations: map[string]*Relation{}, hypotheses: map[string]*Hypothesis{},
		seenFiles: map[string]string{}, ingestQueue: make(chan string, 2048), observeQueue: make(chan string, 512),
		ui: newUIGenome(r, 0), engineGenome: newEngineGenome(r, 0), rng: r, sourceSnapshot: string(src),
	}
	e.computeTarget.Store(35)
	e.stop = make(chan struct{})
	e.sessionKey = randomID()
	e.privacy = newPrivacy(dataDir)
	e.initLab()
	e.ui.Order = append([]string{}, panelIDs...)
	e.ui.Topology = "grid"
	e.ui.Columns = 3
	e.ui.Radius = 10
	e.ui.Density = 1
	for i := 0; i < 6; i++ {
		e.swarms = append(e.swarms, &Swarm{ID: fmt.Sprintf("swarm-%02d", i+1), Population: 64, Alive: 64, Diversity: 0.5, Profile: newProfile(r), Focus: "unassigned"})
	}
	e.load()
	e.repairLoadedState()
	e.initLearning()
	e.initEvolutionJobs()
	e.heavy = make(chan struct{}, 1)
	e.images = newNativeImages(e)
	e.finishing = newFinishing(e)
	e.development = newDevelopment(e)
	e.studio = newConceptStudio(e)
	e.commonVision = newCommonVision(e)
	e.mediaStudio = newMediaStudio(e)
	e.detectGPU()
	e.addEvent("BOOT", "Standalone engine initialized; no external runtime required.")
	return e
}

func (e *Engine) addEvent(kind, detail string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, Event{Time: now(), Kind: kind, Detail: detail})
	if len(e.events) > 240 {
		e.events = append([]Event(nil), e.events[len(e.events)-240:]...)
	}
}

func (e *Engine) detectGPU() {
	gpu := "not detected"
	if p, err := exec.LookPath("nvidia-smi"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, p, "--query-gpu=name", "--format=csv,noheader")
		if out, err := cmd.Output(); err == nil {
			s := strings.TrimSpace(string(out))
			if s != "" {
				gpu = s
			}
		}
	}
	e.gpu = gpu
	if gpu != "not detected" {
		e.gpuBackend = "detected / standalone engine currently CPU compute"
	} else {
		e.gpuBackend = "none"
	}
}

func (e *Engine) load() {
	b, err := os.ReadFile(filepath.Join(e.dataDir, "state.json"))
	if err != nil {
		return
	}
	var p PersistState
	if json.Unmarshal(b, &p) != nil {
		_ = atomicWrite(filepath.Join(e.dataDir, "state.corrupt-"+randomID()[:8]+".json"), b)
		backup, err := os.ReadFile(filepath.Join(e.dataDir, "state.previous.json"))
		if err != nil || json.Unmarshal(backup, &p) != nil {
			e.addEvent("LOAD-ERROR", "State could not be read. Corrupt bytes were preserved for recovery.")
			return
		}
		e.addEvent("RECOVERED", "Recovered the previous valid checkpoint.")
	}
	if p.Lab.Version == 0 {
		legacy := filepath.Join(e.dataDir, "state.before-laboratory.json")
		if _, statErr := os.Stat(legacy); os.IsNotExist(statErr) {
			if err := atomicWrite(legacy, b); err != nil {
				e.addEvent("LOAD-ERROR", "Could not preserve legacy state: "+err.Error())
				return
			}
		}
	}
	if p.Concepts != nil {
		e.concepts = p.Concepts
	}
	if p.Relations != nil {
		e.relations = p.Relations
	}
	if p.Hypotheses != nil {
		e.hypotheses = p.Hypotheses
	}
	e.lab = p.Lab
	if p.ComputeTarget >= 5 && p.ComputeTarget <= 100 {
		e.computeTarget.Store(int64(p.ComputeTarget))
	}
	e.cycle.Store(p.Cycles)
	e.questions = p.Questions
	e.experiences = p.Experiences
	if len(p.Swarms) > 0 {
		e.swarms = p.Swarms
	}
	if p.UIGenome.ID != "" {
		e.ui = p.UIGenome
	}
	if p.EngineGenome.ID != "" {
		e.engineGenome = p.EngineGenome
	}
	e.reflections = p.Reflections
	e.approvals = p.Approvals
}

func (e *Engine) persist() {
	e.persistMu.Lock()
	defer e.persistMu.Unlock()
	e.mu.RLock()
	p := PersistState{ComputeTarget: int(e.computeTarget.Load()), Lab: e.lab, Cycles: e.cycle.Load(), Concepts: e.concepts, Relations: e.relations, Hypotheses: e.hypotheses, Questions: e.questions, Experiences: e.experiences, Swarms: e.swarms, UIGenome: e.ui, EngineGenome: e.engineGenome, Reflections: e.reflections, Approvals: e.approvals}
	b, err := json.Marshal(p)
	e.mu.RUnlock()
	if err != nil {
		return
	}
	if previous, readErr := os.ReadFile(filepath.Join(e.dataDir, "state.json")); readErr == nil && json.Valid(previous) {
		if err := atomicWrite(filepath.Join(e.dataDir, "state.previous.json"), previous); err != nil {
			e.addEvent("SAVE-ERROR", "Could not retain recovery copy: "+err.Error())
			return
		}
	}
	if err := atomicWrite(filepath.Join(e.dataDir, "state.json"), b); err != nil {
		e.addEvent("SAVE-ERROR", err.Error())
	} else {
		e.lastPersist.Store(now())
	}
}

func (e *Engine) launch(fn func()) { e.wg.Add(1); go func() { defer e.wg.Done(); fn() }() }
func (e *Engine) Start() {
	e.launch(e.watchDropFolder)
	e.launch(e.ingestLoop)
	e.launch(e.observeLoop)
	e.launch(e.uiEvolutionLoop)
	e.launch(e.swarmLoop)
	e.launch(e.selfReflectionLoop)
	e.launch(e.persistLoop)
	e.launch(e.consoleLoop)
	e.launch(e.learningLoop)
	workers := minInt(runtime.NumCPU(), 8)
	if workers < 1 {
		workers = 1
	}
	for i := 0; i < workers; i++ {
		id := i
		e.launch(func() { e.computeWorker(id, workers) })
	}
	e.launch(e.seedExperiences)
}

func (e *Engine) watchDropFolder() {
	ticker := time.NewTicker(12 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-e.stop:
			return
		case <-ticker.C:
		}
		_ = filepath.WalkDir(e.dropDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			info, err := d.Info()
			if err != nil || time.Since(info.ModTime()) < 3*time.Second {
				return nil
			}
			key := fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano())
			e.mu.RLock()
			old := e.seenFiles[path]
			e.mu.RUnlock()
			if old != key {
				select {
				case e.ingestQueue <- path:
					e.mu.Lock()
					e.seenFiles[path] = key
					e.mu.Unlock()
				default:
					return filepath.SkipAll
				}
			}
			return nil
		})
	}
}

func sampleFile(path string) ([]byte, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, 0, err
	}
	size := st.Size()
	const capBytes int64 = 2 * 1024 * 1024
	if size <= capBytes {
		b, err := io.ReadAll(io.LimitReader(f, capBytes))
		return b, size, err
	}
	// Sample beginning, center, and end without treating file type specially.
	part := capBytes / 3
	out := make([]byte, 0, capBytes)
	offs := []int64{0, max64(0, size/2-part/2), max64(0, size-part)}
	for _, off := range offs {
		_, _ = f.Seek(off, 0)
		buf := make([]byte, part)
		n, _ := io.ReadFull(f, buf)
		out = append(out, buf[:n]...)
	}
	return out, size, nil
}
func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func (e *Engine) ingestLoop() {
	for {
		select {
		case <-e.stop:
			return
		case path := <-e.ingestQueue:
			e.ingestFile(path, "drop")
		}
	}
}
func (e *Engine) observeLoop() {
	for {
		select {
		case <-e.stop:
			return
		case text := <-e.observeQueue:
			e.ingestBytes("human-observation", []byte(text), int64(len(text)), "observation")
		}
	}
}
func (e *Engine) ingestFile(path, source string) {
	b, size, err := sampleFile(path)
	if err != nil {
		e.addEvent("INGEST-ERROR", err.Error())
		return
	}
	e.ingestBytes(filepath.Base(path), b, size, source)
}

func (e *Engine) ingestBytes(name string, b []byte, size int64, source string) {
	e.ingestMu.Lock()
	defer e.ingestMu.Unlock()
	if len(b) == 0 {
		return
	}
	fullHash := sha256.Sum256(b)
	sampleHash := hex.EncodeToString(fullHash[:])
	e.mu.RLock()
	duplicate := false
	for _, x := range e.experiences {
		if x.Hash == sampleHash && x.Size == size {
			duplicate = true
			break
		}
	}
	e.mu.RUnlock()
	if duplicate {
		return
	}
	exp := Experience{SampleBytes: len(b), HashScope: "bounded sample (not whole-file identity)", ID: "x-" + hex.EncodeToString(fullHash[:])[:12], Name: name, Size: size, Hash: hex.EncodeToString(fullHash[:]), Seen: now(), Source: source}
	counts := map[string]int{}
	// Pre-semantic raw-byte motif extraction. No MIME/extension classification is used.
	stride := 4
	if len(b) > 256*1024 {
		stride = 16
	}
	var prev string
	for i := 0; i+8 <= len(b); i += stride {
		id := motifHash(b[i : i+8])
		counts[id]++
		e.mu.Lock()
		c := e.concepts[id]
		if c == nil {
			c = &Concept{ID: id, FirstSeen: now()}
			e.concepts[id] = c
		}
		c.Count++
		c.LastSeen = now()
		c.Utility = math.Log1p(float64(c.Count))
		if prev != "" && prev != id {
			a, bid := prev, id
			if a > bid {
				a, bid = bid, a
			}
			rk := a + "|" + bid
			r := e.relations[rk]
			if r == nil {
				r = &Relation{A: a, B: bid}
				e.relations[rk] = r
			}
			r.Count++
			r.Confidence = 1 - math.Exp(-float64(r.Count)/4)
		}
		e.mu.Unlock()
		prev = id
	}
	type kv struct {
		k string
		v int
	}
	arr := make([]kv, 0, len(counts))
	for k, v := range counts {
		arr = append(arr, kv{k, v})
	}
	sort.Slice(arr, func(i, j int) bool { return arr[i].v > arr[j].v })
	if len(arr) > 32 {
		arr = arr[:32]
	}
	for _, x := range arr {
		exp.TopConcepts = append(exp.TopConcepts, x.k)
	}
	e.mu.Lock()
	e.evaluateEvidenceLocked(counts)
	e.experiences = append(e.experiences, exp)
	if len(e.experiences) > 3000 {
		e.experiences = e.experiences[len(e.experiences)-3000:]
	}
	e.mu.Unlock()
	e.recordExperience(exp)
	e.discoverBlocks(name, b, source)
	e.rebuildCodes()
	e.createHypothesesFromRelations(180)
	e.generateQuestions(12)
	e.compact()
	e.addEvent("INGEST", fmt.Sprintf("%s -> %d sampled bytes; %d recurring motifs", name, len(b), len(counts)))
}

func (e *Engine) rebuildCodes() {
	e.mu.Lock()
	defer e.mu.Unlock()
	list := make([]*Concept, 0, len(e.concepts))
	for _, c := range e.concepts {
		list = append(list, c)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Count > list[j].Count })
	if len(list) > 2048 {
		list = list[:2048]
	}
	for _, c := range e.concepts {
		c.Code = ""
	}
	for i, c := range list {
		c.Code = binaryCode(i + 1)
	}
}

func (e *Engine) createHypothesesFromRelations(limit int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	rels := make([]*Relation, 0, len(e.relations))
	for _, r := range e.relations {
		if r.Count >= 1 {
			rels = append(rels, r)
		}
	}
	sort.Slice(rels, func(i, j int) bool { return rels[i].Confidence > rels[j].Confidence })
	if len(rels) > limit {
		rels = rels[:limit]
	}
	for _, r := range rels {
		id := "h-" + shortHash(r.A+r.B)
		h := e.hypotheses[id]
		if h == nil {
			h = &Hypothesis{ID: id, A: r.A, B: r.B, Symbolic: "?REL " + r.A[:7] + " " + r.B[:7]}
			e.hypotheses[id] = h
		}
		h.Support = maxInt(h.Support, r.Count)
		h.Confidence = float64(h.Support+1) / float64(h.Support+h.Contradictions+2)
		h.LastTest = now()
	}
}

func (e *Engine) generateQuestions(n int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	hs := make([]*Hypothesis, 0, len(e.hypotheses))
	for _, h := range e.hypotheses {
		if h.Confidence > 0.25 {
			hs = append(hs, h)
		}
	}
	e.rng.Shuffle(len(hs), func(i, j int) { hs[i], hs[j] = hs[j], hs[i] })
	if len(hs) > n {
		hs = hs[:n]
	}
	for _, h := range hs {
		id := "q-" + h.ID
		exists := false
		for _, q := range e.questions {
			if q.ID == id {
				exists = true
				break
			}
		}
		if exists {
			continue
		}
		op := []string{"?REL", "?COUNTER", "?TRANSFER", "?CAUSE?"}[e.rng.Intn(4)]
		human := fmt.Sprintf("What observation would most reduce uncertainty about the relation between %s and %s?", h.A[:7], h.B[:7])
		if h.Confidence >= 0.9 {
			op = "?COUNTER"
			human = fmt.Sprintf("Can a counterexample be found that breaks the currently strong relation between %s and %s?", h.A[:7], h.B[:7])
		}
		q := Question{ID: id, Symbolic: fmt.Sprintf("%s %s %s", op, h.A[:7], h.B[:7]), Human: human, Priority: math.Max(0.15, 1-math.Abs(h.Confidence-0.5)*2), Created: now()}
		e.questions = append(e.questions, q)
	}
	if len(e.questions) > 200 {
		e.questions = e.questions[len(e.questions)-200:]
	}
}

func (e *Engine) compact() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.concepts) > 6000 {
		list := make([]*Concept, 0, len(e.concepts))
		for _, c := range e.concepts {
			list = append(list, c)
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Utility > list[j].Utility })
		keep := map[string]bool{}
		for i := 0; i < 4500 && i < len(list); i++ {
			keep[list[i].ID] = true
		}
		for k := range e.concepts {
			if !keep[k] {
				delete(e.concepts, k)
			}
		}
		for k, r := range e.relations {
			if !keep[r.A] || !keep[r.B] {
				delete(e.relations, k)
			}
		}
	}
	if len(e.relations) > 24000 {
		list := make([]*Relation, 0, len(e.relations))
		for _, r := range e.relations {
			list = append(list, r)
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Count > list[j].Count })
		keep := map[string]bool{}
		for i := 0; i < 18000 && i < len(list); i++ {
			a, b := list[i].A, list[i].B
			if a > b {
				a, b = b, a
			}
			keep[a+"|"+b] = true
		}
		for k := range e.relations {
			if !keep[k] {
				delete(e.relations, k)
			}
		}
	}
}

func (e *Engine) computeWorker(id, total int) {
	r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)*7919))
	for {
		select {
		case <-e.stop:
			return
		default:
		}
		if e.paused.Load() || e.imageBusy.Load() {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		target := int(e.computeTarget.Load())
		active := int(math.Ceil(float64(total) * float64(target) / 100))
		if active < 1 {
			active = 1
		}
		e.activeWorkers.Store(int64(active))
		if id >= active {
			time.Sleep(120 * time.Millisecond)
			continue
		}
		burstStart := time.Now()
		ops := 0
		for time.Since(burstStart) < 25*time.Millisecond {
			e.reasonStep(r)
			ops++
		}
		e.ops.Add(uint64(ops))
		e.cycle.Add(1)
		if target < 100 {
			sleep := time.Duration(float64(25*time.Millisecond) * (100 - float64(target)) / math.Max(float64(target), 1))
			if sleep > 0 {
				time.Sleep(sleep)
			}
		}
	}
}

func (e *Engine) reasonStep(r *rand.Rand) {
	e.mu.RLock()
	rels := make([]Relation, 0, 500)
	for _, x := range e.relations {
		rels = append(rels, *x)
		if len(rels) >= 500 {
			break
		}
	}
	depth, skeptic := e.engineGenome.InferenceDepth, e.engineGenome.Skepticism
	e.mu.RUnlock()
	if len(rels) < 2 {
		time.Sleep(time.Millisecond)
		return
	}
	first := rels[r.Intn(len(rels))]
	start, node, conf := first.A, first.B, first.Confidence
	visited := map[string]bool{start: true, node: true}
	hops := 1
	for hops < depth {
		candidates := []Relation{}
		for _, edge := range rels {
			if edge.A == node && !visited[edge.B] || edge.B == node && !visited[edge.A] {
				candidates = append(candidates, edge)
			}
		}
		if len(candidates) == 0 {
			break
		}
		next := candidates[r.Intn(len(candidates))]
		if next.A == node {
			node = next.B
		} else {
			node = next.A
		}
		visited[node] = true
		conf *= next.Confidence * (1 - .2*skeptic)
		hops++
	}
	if hops < 2 || conf < .08 {
		return
	}
	x, y := start, node
	if x > y {
		x, y = y, x
	}
	id := "hi-" + shortHash(x+y)
	e.mu.Lock()
	defer e.mu.Unlock()
	h := e.hypotheses[id]
	if h == nil {
		if len(e.hypotheses) >= 6000 {
			return
		}
		h = &Hypothesis{ID: id, A: x, B: y, Indirect: true, Symbolic: fmt.Sprintf("?PATH %d hops %s %s", hops, x[:7], y[:7]), Confidence: conf}
		e.hypotheses[id] = h
	}
	h.Inferences++
	h.LastTest = now()
	// Repeating an inference is not independent evidence and cannot increase support.
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (e *Engine) uiEvolutionLoop() {
	candidateTick := time.NewTicker(20 * time.Second)
	promoteTick := time.NewTicker(13 * time.Second)
	defer candidateTick.Stop()
	defer promoteTick.Stop()
	for {
		select {
		case <-e.stop:
			return
		case <-candidateTick.C:
			if !e.paused.Load() {
				e.generateUICandidates()
			}
		case <-promoteTick.C:
			e.autoPromoteUI()
		}
	}
}

func (e *Engine) mutateUI(parent UIGenome) UIGenome {
	e.mu.Lock()
	defer e.mu.Unlock()
	r := e.rng
	g := cloneUI(parent)
	g.Generation = parent.Generation + 1
	g.ID = fmt.Sprintf("ui-%d-%s", g.Generation, shortHash(fmt.Sprint(r.Int63()))[:6])
	g.Name = fmt.Sprintf("Interface %d.%s", g.Generation, g.ID[len(g.ID)-3:])
	g.Preferred = 0
	g.Rejected = 0
	mutations := []string{}
	if r.Float64() < 0.35 {
		topologies := []string{"grid", "focus", "ribbon", "mosaic"}
		g.Topology = topologies[r.Intn(len(topologies))]
		mutations = append(mutations, "topology")
	}
	if r.Float64() < 0.8 {
		g.Columns = 2 + r.Intn(4)
		mutations = append(mutations, "geometry")
	}
	if r.Float64() < 0.8 {
		g.Density = clamp(g.Density+(r.Float64()-0.5)*0.28, 0.62, 1.35)
		mutations = append(mutations, "density")
	}
	if r.Float64() < 0.6 {
		g.Radius = r.Intn(24)
		mutations = append(mutations, "surface")
	}
	if r.Float64() < 0.6 {
		g.MotionMS = 80 + r.Intn(1200)
		mutations = append(mutations, "motion")
	}
	if r.Float64() < 0.8 && len(g.Order) > 1 {
		i, j := r.Intn(len(g.Order)), r.Intn(len(g.Order))
		g.Order[i], g.Order[j] = g.Order[j], g.Order[i]
		mutations = append(mutations, "hierarchy")
	}
	g.RefreshMS = 550 + r.Intn(1800)
	g.Rationale = "mutated " + strings.Join(mutations, ", ")
	return g
}

func (e *Engine) scoreUI(g UIGenome) float64 {
	e.mu.RLock()
	q := len(e.questions)
	h := len(e.hypotheses)
	s := len(e.swarms)
	c := len(e.concepts)
	e.mu.RUnlock()
	score := 0.0
	index := func(id string) int {
		for i, x := range g.Order {
			if x == id {
				return i
			}
		}
		return len(g.Order)
	}
	score += 2.5 / float64(index("activity")+1)
	if q > 20 {
		score += 2.0 / float64(index("questions")+1)
	}
	if h > 100 {
		score += 2.0 / float64(index("hypotheses")+1)
	}
	if s > 2 {
		score += 1.4 / float64(index("swarms")+1)
	}
	if c > 100 {
		score += 1.2 / float64(index("concepts")+1)
	}
	score += 1.0 - math.Abs(g.Density-0.95)
	score += float64(g.Preferred)*0.5 - float64(g.Rejected)*0.7
	return score
}

func (e *Engine) generateUICandidates() {
	e.mu.RLock()
	parent := cloneUI(e.ui)
	e.mu.RUnlock()
	e.mu.RLock()
	existing := len(e.uiCandidates)
	e.mu.RUnlock()
	if existing > 0 {
		return
	}
	arr := make([]UIGenome, 0, 4)
	for i := 0; i < 4; i++ {
		g := e.mutateUI(parent)
		g.Score = e.scoreUI(g)
		arr = append(arr, g)
	}
	e.mu.Lock()
	e.uiCandidates = arr
	e.mu.Unlock()
	e.addEvent("UI-LAB", fmt.Sprintf("generated %d interface descendants from generation %d", len(arr), parent.Generation))
}
func (e *Engine) autoPromoteUI() { e.promoteMeasuredCandidate() }

func mutateProfile(p SwarmProfile, r *rand.Rand) SwarmProfile {
	d := func(v float64) float64 { return clamp(v+(r.Float64()-0.5)*p.Mutation*3, 0.02, 0.98) }
	p.Explore = d(p.Explore)
	p.Skepticism = d(p.Skepticism)
	p.Compression = d(p.Compression)
	p.Sociality = d(p.Sociality)
	p.Risk = d(p.Risk)
	p.Mutation = clamp(p.Mutation+(r.Float64()-0.5)*0.02, 0.005, 0.18)
	return p
}

func (e *Engine) swarmLoop() {
	tick := time.NewTicker(1500 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-e.stop:
			return
		case <-tick.C:
		}
		if !e.paused.Load() {
			e.stepSwarms()
		}
	}
}
func (e *Engine) stepSwarms() {
	e.mu.Lock()
	defer e.mu.Unlock()
	hs := make([]*Hypothesis, 0, len(e.hypotheses))
	for _, h := range e.hypotheses {
		hs = append(hs, h)
	}
	for _, s := range e.swarms {
		s.Generation++
		if s.Memory == nil {
			s.Memory = map[string]float64{}
		}
		// Mortality is an explicitly simulated population mechanism.
		mortality := minInt(s.Alive, int(float64(s.Population)*.01*s.Profile.Risk))
		s.Alive -= mortality
		s.Deaths += mortality
		if len(hs) > 0 {
			budget := minInt(len(hs), 8+int(24*s.Profile.Explore))
			total := 0.0
			for j := 0; j < budget; j++ {
				h := hs[e.rng.Intn(len(hs))]
				value := h.Confidence - s.Profile.Skepticism*float64(h.Contradictions)/float64(h.Support+h.Contradictions+1)
				if _, seen := s.Memory[h.ID]; !seen && len(s.Memory) < 256 {
					s.Memory[h.ID] = value
					s.Discoveries++
				}
				total += value
				s.Focus = h.Symbolic
			}
			s.MeanFitness = total / float64(budget)
			s.BestFitness = math.Max(s.BestFitness, s.MeanFitness)
			s.LocalHypotheses = len(s.Memory)
		}
		if s.Alive <= maxInt(2, s.Population/12) {
			s.Extinctions++
			s.AncestralScar = shortHash(fmt.Sprintf("%s:%d", s.ID, s.Generation))
			s.Profile = mutateProfile(s.Profile, e.rng)
			s.Alive = maxInt(4, s.Population/3)
			s.Rebirths += s.Alive
		}
		s.Diversity = float64(len(s.Memory)) / 256
		s.Entropy = 0
	}
	if len(e.swarms) > 1 {
		a, b := e.swarms[e.rng.Intn(len(e.swarms))], e.swarms[e.rng.Intn(len(e.swarms))]
		if a != b && e.rng.Float64() < e.engineGenome.SwarmMigration {
			for id, score := range a.Memory {
				if _, ok := b.Memory[id]; !ok && len(b.Memory) < 256 {
					b.Memory[id] = score
					a.Exchanges++
					b.Exchanges++
					break
				}
			}
		}
	}
	for _, a := range e.swarms {
		shared, disagree := 0, 0
		for _, b := range e.swarms {
			if a == b {
				continue
			}
			for id, va := range a.Memory {
				if vb, ok := b.Memory[id]; ok {
					shared++
					if math.Abs(va-vb) > .1 {
						disagree++
					}
				}
			}
		}
		a.Consensus = 0
		a.Dissent = 0
		if shared > 0 {
			a.Dissent = float64(disagree) / float64(shared)
			a.Consensus = 1 - a.Dissent
		}
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func randNorm(r *rand.Rand) float64 { return r.NormFloat64() }

func (e *Engine) selfReflectionLoop() {
	ticker := time.NewTicker(11 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-e.stop:
			return
		case <-ticker.C:
		}
		if e.paused.Load() {
			continue
		}
		e.selfReflect()
		e.evolveEngineGenome()
	}
}
func (e *Engine) selfReflect() {
	src := e.sourceSnapshot
	lines := strings.Count(src, "\n") + 1
	funcs := strings.Count(src, "func ")
	locks := strings.Count(src, ".Lock()") + strings.Count(src, ".RLock()")
	e.mu.RLock()
	h := len(e.hypotheses)
	c := len(e.concepts)
	q := len(e.questions)
	e.mu.RUnlock()
	obs := fmt.Sprintf("Embedded source snapshot: %d lines, %d functions, %d lock sites. Runtime knowledge: %d concepts, %d hypotheses, %d questions.", lines, funcs, locks, c, h, q)
	prop := "Benchmark an engine-profile descendant before changing executable code. Current source-level candidates should remain sandboxed proposals until human promotion."
	if h > 4000 {
		prop = "Hypothesis store is approaching its working bound; candidate descendants should increase contradiction pruning before increasing capacity."
	}
	if c > 5000 {
		prop = "Concept pressure is high; test stronger compression/merging before expanding active memory."
	}
	ref := Reflection{Time: now(), Observation: obs, Proposal: prop, Confidence: 0, SourceLevel: true}
	e.mu.Lock()
	e.reflections = append(e.reflections, ref)
	if len(e.reflections) > 80 {
		e.reflections = e.reflections[len(e.reflections)-80:]
	}
	needEvidenceRequest := q > 8
	alreadyRequested := false
	for _, a := range e.approvals {
		if a.Kind == "read-only-evidence-channel" {
			alreadyRequested = true
			break
		}
	}
	e.mu.Unlock()
	if needEvidenceRequest && !alreadyRequested {
		e.requestApproval("read-only-evidence-channel", "Expose a bounded, read-only external research/evidence channel to test uncertain hypotheses.", "Current internal questions exceed the local evidence base. Local cognition will continue while this remains unapproved.")
	}
	// Write a transparent source-rewrite proposal, but never self-apply or replace the executable.
	name := "current_source_review.md"
	body := fmt.Sprintf("# ORIGIN-0 source reflection\n\n%s\n\n## Candidate change\n%s\n\nThis is a proposal only. The running executable does not self-replace.\n", obs, prop)
	_ = os.WriteFile(filepath.Join(e.dataDir, "proposals", name), []byte(body), 0644)
}

func mutateEngine(g EngineGenome, r *rand.Rand) EngineGenome {
	n := g
	n.Generation++
	n.ID = fmt.Sprintf("eg-%d-%s", n.Generation, shortHash(fmt.Sprint(r.Int63()))[:6])
	n.InferenceDepth = maxInt(1, minInt(6, n.InferenceDepth+r.Intn(3)-1))
	n.NoveltyWeight = clamp(n.NoveltyWeight+(r.Float64()-0.5)*0.3, 0.1, 2)
	n.Skepticism = clamp(n.Skepticism+(r.Float64()-0.5)*0.15, 0.05, 0.95)
	n.QuestionSensitivity = clamp(n.QuestionSensitivity+(r.Float64()-0.5)*0.2, 0.05, 1)
	n.SwarmMigration = clamp(n.SwarmMigration+(r.Float64()-0.5)*0.1, 0, 0.8)
	n.CompressionPressure = clamp(n.CompressionPressure+(r.Float64()-0.5)*0.18, 0.05, 1.5)
	return n
}
func (e *Engine) scoreEngine(g EngineGenome) float64 {
	e.mu.RLock()
	h := len(e.hypotheses)
	q := len(e.questions)
	c := len(e.concepts)
	rels := len(e.relations)
	e.mu.RUnlock()
	capability := math.Log1p(float64(h+rels)) + 0.4*math.Log1p(float64(c))
	uncertainty := math.Log1p(float64(q))
	cost := 0.22*float64(g.InferenceDepth) + 0.18*g.NoveltyWeight + 0.15*g.QuestionSensitivity
	return capability + g.CompressionPressure*0.5 + uncertainty*g.Skepticism*0.15 - cost
}
func (e *Engine) evolveEngineGenome() {
	e.mu.RLock()
	p := e.engineGenome
	e.mu.RUnlock()
	arr := make([]EngineGenome, 0, 5)
	e.mu.Lock()
	for i := 0; i < 5; i++ {
		g := mutateEngine(p, e.rng)
		g.Score = e.scoreEngineUnlocked(g)
		arr = append(arr, g)
	}
	e.engineCandidates = arr
	e.mu.Unlock()
	best := arr[0]
	for _, g := range arr[1:] {
		if g.Score > best.Score {
			best = g
		}
	}
	e.mu.Lock()
	base := e.scoreEngineUnlocked(e.engineGenome)
	if best.Score > base*1.002 {
		// This score is a hand-written cost heuristic, not a measured improvement.
		e.lab.EngineNote = "Candidate profiles generated; automatic promotion awaits an external benchmark."
		// e.engineGenome is retained until benchmarked.
		e.mu.Unlock()
		e.addEvent("ENGINE-CANDIDATE", fmt.Sprintf("heuristic candidate %s score %.3f", best.ID, best.Score))
	} else {
		e.mu.Unlock()
	}
}
func (e *Engine) scoreEngineUnlocked(g EngineGenome) float64 {
	capability := math.Log1p(float64(len(e.hypotheses)+len(e.relations))) + 0.4*math.Log1p(float64(len(e.concepts)))
	uncertainty := math.Log1p(float64(len(e.questions)))
	cost := 0.22*float64(g.InferenceDepth) + 0.18*g.NoveltyWeight + 0.15*g.QuestionSensitivity
	return capability + g.CompressionPressure*0.5 + uncertainty*g.Skepticism*0.15 - cost
}

func (e *Engine) persistLoop() {
	ticker := time.NewTicker(8 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-e.stop:
			return
		case <-ticker.C:
		}
		e.persist()
	}
}

func (e *Engine) consoleLoop() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	fmt.Println("============================================================")
	fmt.Println("ORIGIN-0 v1.8.0 - OPEN CREATIVE STUDIO")
	fmt.Println("Open Models & engines for local model packs and portable ComfyUI.")
	fmt.Println("============================================================")
	for {
		select {
		case <-e.stop:
			return
		case <-ticker.C:
		}
		tel := e.telemetry()
		e.mu.RLock()
		c, h, q := len(e.concepts), len(e.hypotheses), len(e.questions)
		ui := e.ui.Generation
		sw := len(e.swarms)
		e.mu.RUnlock()
		fmt.Printf("cycle=%d | %.1f cyc/s | workers=%d/%d | target=%d%% | mem=%.1fMB | concepts=%d | hypotheses=%d | questions=%d | swarms=%d | ui-gen=%d\n", tel.Cycles, tel.CyclesPerSec, tel.ActiveWorkers, tel.LogicalCPU, tel.ComputeTarget, tel.HeapMB, c, h, q, sw, ui)
	}
}

func (e *Engine) telemetry() Telemetry {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	cy := e.cycle.Load()
	t := time.Now()
	e.mu.Lock()
	dt := t.Sub(e.lastCycleSampleTime).Seconds()
	if e.lastCycleSampleTime.IsZero() {
		dt = t.Sub(e.started).Seconds()
	}
	if dt > 0 {
		e.cycleRate = float64(cy-e.lastCycleSample) / dt
	}
	e.lastCycleSample = cy
	e.lastCycleSampleTime = t
	// Busy ratio is a direct work-budget metric, not a fake system CPU percentage.
	e.busyRatio = float64(e.computeTarget.Load()) / 100
	rate, busy := e.cycleRate, e.busyRatio
	e.mu.Unlock()
	return Telemetry{UptimeSec: int64(time.Since(e.started).Seconds()), Cycles: cy, CyclesPerSec: rate, LogicalCPU: runtime.NumCPU(), ActiveWorkers: int(e.activeWorkers.Load()), ComputeTarget: int(e.computeTarget.Load()), BusyRatio: busy, Goroutines: runtime.NumGoroutine(), HeapMB: float64(m.HeapAlloc) / 1024 / 1024, SysMB: float64(m.Sys) / 1024 / 1024, GPU: e.gpu, GPUBackend: e.gpuBackend, Backend: "native-go/cpu", LastCheckpoint: e.lastPersist.Load()}
}

func topConcepts(m map[string]*Concept, n int) []*Concept {
	a := make([]*Concept, 0, len(m))
	for _, c := range m {
		copy := *c
		a = append(a, &copy)
	}
	sort.Slice(a, func(i, j int) bool { return a[i].Count > a[j].Count })
	if len(a) > n {
		a = a[:n]
	}
	return a
}
func topRelations(m map[string]*Relation, n int) []*Relation {
	a := make([]*Relation, 0, len(m))
	for _, r := range m {
		copy := *r
		a = append(a, &copy)
	}
	sort.Slice(a, func(i, j int) bool { return a[i].Confidence > a[j].Confidence })
	if len(a) > n {
		a = a[:n]
	}
	return a
}
func topHypotheses(m map[string]*Hypothesis, n int) []*Hypothesis {
	a := make([]*Hypothesis, 0, len(m))
	for _, h := range m {
		copy := *h
		a = append(a, &copy)
	}
	sort.Slice(a, func(i, j int) bool { return a[i].Confidence > a[j].Confidence })
	if len(a) > n {
		a = a[:n]
	}
	return a
}
func languageView(m map[string]*Concept, n int) []LanguageToken {
	cs := topConcepts(m, n)
	out := make([]LanguageToken, 0, len(cs))
	for _, c := range cs {
		out = append(out, LanguageToken{Motif: c.ID, Code: c.Code, Count: c.Count, Utility: c.Utility})
	}
	return out
}

func (e *Engine) view() StateView {
	tel := e.telemetry()
	e.mu.RLock()
	defer e.mu.RUnlock()
	ex := append([]Experience{}, e.experiences...)
	if len(ex) > 30 {
		ex = ex[len(ex)-30:]
	}
	qs := append([]Question{}, e.questions...)
	if len(qs) > 40 {
		qs = qs[len(qs)-40:]
	}
	ev := append([]Event{}, e.events...)
	if len(ev) > 80 {
		ev = ev[len(ev)-80:]
	}
	refs := append([]Reflection{}, e.reflections...)
	if len(refs) > 20 {
		refs = refs[len(refs)-20:]
	}
	health := map[string]any{"status": "running", "state_dir": e.dataDir, "drop_dir": e.dropDir, "bounded_working_memory": true, "network_access": "explicit setup, provider actions and worker invitations", "self_replace_executable": false, "source_reflection": true, "internal_clone_swarms": len(e.swarms)}
	uiCandidates := []UIGenome{}
	for _, g := range e.uiCandidates {
		uiCandidates = append(uiCandidates, cloneUI(g))
	}
	engineCandidates := append([]EngineGenome{}, e.engineCandidates...)
	approvals := append([]Approval{}, e.approvals...)
	swarms := []*Swarm{}
	for _, s := range e.swarms {
		copy := *s
		copy.Memory = nil
		swarms = append(swarms, &copy)
	}
	labSummary := e.lab
	labSummary.Learning = LearningState{}
	labSummary.Studio = ConceptState{}
	labSummary.Jobs = nil
	return StateView{Lab: cloneLab(labSummary), Version: "1.8.0-open-studio", Telemetry: tel, Concepts: topConcepts(e.concepts, 80), Relations: topRelations(e.relations, 80), Hypotheses: topHypotheses(e.hypotheses, 80), Questions: qs, Experiences: ex, Language: languageView(e.concepts, 80), Swarms: swarms, UIGenome: cloneUI(e.ui), UICandidates: uiCandidates, EngineGenome: e.engineGenome, EngineCandidates: engineCandidates, Reflections: refs, Approvals: approvals, Events: ev, Health: health}
}

func (e *Engine) requestApproval(kind, request, why string) {
	a := Approval{ID: "a-" + shortHash(fmt.Sprintf("%d:%s", time.Now().UnixNano(), request)), Kind: kind, Request: request, Why: why, Status: "waiting", Created: now(), Reflections: []string{
		"Why is this capability necessary rather than merely convenient?",
		"What useful inference can continue locally while approval is pending?",
		"Can existing evidence or simulation answer part of the question?",
		"What narrower authorized capability would be sufficient?",
		"What information would reduce the need for this capability?",
		"What measurable information gain is expected?",
	}}
	e.mu.Lock()
	e.approvals = append(e.approvals, a)
	if len(e.approvals) > 60 {
		e.approvals = e.approvals[len(e.approvals)-60:]
	}
	e.mu.Unlock()
	e.addEvent("APPROVAL", request)
}

func (e *Engine) handler() http.Handler { return e.laboratoryHandler() }

func baseDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func openBrowser(url string) {
	switch runtime.GOOS {
	case "windows":
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		_ = exec.Command("open", url).Start()
	default:
		_ = exec.Command("xdg-open", url).Start()
	}
}

func writableBase() string {
	if v := os.Getenv("ORIGIN0_HOME"); v != "" {
		return v
	}
	base := baseDir()
	test := filepath.Join(base, ".origin0_write_test")
	if err := os.WriteFile(test, []byte("ok"), 0600); err == nil {
		_ = os.Remove(test)
		return base
	}
	if cfg, err := os.UserConfigDir(); err == nil && cfg != "" {
		fallback := filepath.Join(cfg, "ORIGIN0")
		_ = os.MkdirAll(fallback, 0755)
		return fallback
	}
	return "."
}

func listenLocal() (net.Listener, string, error) {
	for p := 8765; p <= 8785; p++ {
		addr := fmt.Sprintf("127.0.0.1:%d", p)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			return ln, addr, nil
		}
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", err
	}
	return ln, ln.Addr().String(), nil
}

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	if os.Getenv("ORIGIN0_RELAY_MODE") == "1" {
		if err := runInternetRelay(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	base := writableBase()
	e := NewEngine(base)
	ln, addr, err := listenLocal()
	if err != nil {
		fmt.Println("FATAL: cannot open local interface port:", err)
		fmt.Println("Press Enter to close.")
		_, _ = fmt.Scanln()
		os.Exit(1)
	}
	e.instanceAddr = addr
	releaseLock, ok := e.acquireInstance()
	if !ok {
		ln.Close()
		return
	}
	defer releaseLock()
	logfile, logErr := os.OpenFile(filepath.Join(e.dataDir, "logs", "runtime.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if logErr == nil {
		defer logfile.Close()
		log.SetOutput(io.MultiWriter(os.Stdout, logfile))
	}
	e.Start()
	e.images.resumeReady()
	srv := &http.Server{Handler: e.handler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	go func() {
		select {
		case <-signals:
		case <-e.stop:
		}
		e.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()
	url := "http://" + addr + "/#" + e.sessionKey
	fmt.Println("ORIGIN-0 is starting at " + url)
	fmt.Println("DROP_HERE folder: " + e.dropDir)
	fmt.Println("State folder: " + e.dataDir)
	if os.Getenv("ORIGIN0_NO_BROWSER") == "" {
		go func() { time.Sleep(900 * time.Millisecond); openBrowser(url) }()
	}
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		fmt.Println("FATAL:", err)
		fmt.Println("Press Enter to close.")
		_, _ = fmt.Scanln()
		os.Exit(1)
	}
}

// Binary helper retained for future bit-level genome operators.
func u64(b []byte) uint64 {
	if len(b) < 8 {
		return 0
	}
	return binary.LittleEndian.Uint64(b[:8])
}
