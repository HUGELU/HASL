package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math/bits"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const conceptImageLimit = 12 << 20

type ConceptAttribute struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type ConceptSource struct {
	URL        string `json:"url"`
	Page       string `json:"page"`
	Creator    string `json:"creator"`
	License    string `json:"license"`
	LicenseURL string `json:"license_url"`
	Query      string `json:"query"`
}
type ConceptExample struct {
	Asset    string        `json:"asset"`
	Name     string        `json:"name"`
	Label    string        `json:"label"`
	Caption  string        `json:"caption"`
	Group    string        `json:"group"`
	Split    string        `json:"split"`
	Review   string        `json:"review"`
	Source   ConceptSource `json:"source"`
	Width    int           `json:"width"`
	Height   int           `json:"height"`
	DHash    uint64        `json:"dhash"`
	Features []float64     `json:"features,omitempty"`
}
type ConceptProject struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	Subject    string             `json:"subject"`
	Attributes []ConceptAttribute `json:"attributes"`
	AutoLearn  bool               `json:"auto_learn"`
	Created    int64              `json:"created"`
	Revision   int                `json:"revision"`
	Evaluated  int                `json:"evaluated"`
	Examples   []ConceptExample   `json:"examples"`
	Active     RecognitionModel   `json:"active"`
	Previous   RecognitionModel   `json:"previous"`
	Runs       []LearningRun      `json:"runs"`
	Note       string             `json:"note"`
}
type ConceptState struct {
	Projects []ConceptProject `json:"projects"`
}
type ImageSearchResult struct {
	ID      string        `json:"id"`
	Title   string        `json:"title"`
	Source  ConceptSource `json:"source"`
	Asset   string        `json:"asset,omitempty"`
	Message string        `json:"message,omitempty"`
}
type ConceptCollection struct {
	ID         string              `json:"id"`
	Project    string              `json:"project"`
	Query      string              `json:"query"`
	Status     string              `json:"status"`
	Message    string              `json:"message"`
	Results    []ImageSearchResult `json:"results"`
	Downloaded int                 `json:"downloaded"`
}
type ConceptStudio struct {
	e            *Engine
	mu           sync.Mutex
	wg           sync.WaitGroup
	cancel       context.CancelFunc
	closed       bool
	collection   ConceptCollection
	searchClient *http.Client
	imageClient  *http.Client
	searxURL     string
}

func newConceptStudio(e *Engine) *ConceptStudio {
	return &ConceptStudio{e: e, searchClient: publicWebClient(), imageClient: publicWebClient()}
}
func (s *ConceptStudio) close() {
	s.mu.Lock()
	s.closed = true
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()
	s.wg.Wait()
}

// Resolve and dial the same validated address, including every redirect. A search
// result must not turn the local application into a private-network proxy.
func publicAddress(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, block := range []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "2001:db8::/32", "64:ff9b::/96", "2002::/16", "2001::/32"} {
		_, n, _ := net.ParseCIDR(block)
		if n.Contains(ip) {
			return false
		}
	}
	return true
}
func publicWebURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || (u.Port() != "" && u.Port() != "443") {
		return nil, errors.New("image sources must use a public HTTPS URL")
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && !publicAddress(ip) {
		return nil, errors.New("private image-source addresses are not allowed")
	}
	return u, nil
}
func publicWebClient() *http.Client {
	t := &http.Transport{ResponseHeaderTimeout: 20 * time.Second, TLSHandshakeTimeout: 10 * time.Second, IdleConnTimeout: 30 * time.Second}
	t.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if port != "443" {
			return nil, errors.New("public HTTPS port required")
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if !publicAddress(ip.IP) {
				return nil, errors.New("source resolves to a private address")
			}
		}
		for _, ip := range ips {
			c, er := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
			if er == nil {
				return c, nil
			}
			err = er
		}
		if err == nil {
			err = errors.New("source has no reachable address")
		}
		return nil, err
	}
	return &http.Client{Transport: t, Timeout: 45 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) > 3 {
			return errors.New("too many source redirects")
		}
		_, err := publicWebURL(r.URL.String())
		return err
	}}
}
func webBytes(ctx context.Context, client *http.Client, raw string, limit int64) ([]byte, error) {
	r, err := http.NewRequestWithContext(ctx, "GET", raw, nil)
	if err != nil {
		return nil, err
	}
	r.Header.Set("User-Agent", "ORIGIN0/1.6 (+https://github.com/HUGELU/HASL)")
	r.Header.Set("Accept", "application/json,image/png,image/jpeg;q=0.9,*/*;q=0.1")
	res, err := client.Do(r)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("source returned HTTP %d; try later or use your own image files", res.StatusCode)
	}
	if res.ContentLength > limit {
		return nil, errors.New("source exceeds the download size limit")
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if int64(len(b)) > limit {
		return nil, errors.New("source exceeds the download size limit")
	}
	return b, err
}
func cloneProject(p ConceptProject) ConceptProject {
	b, _ := json.Marshal(p)
	var v ConceptProject
	_ = json.Unmarshal(b, &v)
	return v
}
func (e *Engine) projectLocked(id string) *ConceptProject {
	for i := range e.lab.Studio.Projects {
		if e.lab.Studio.Projects[i].ID == id {
			return &e.lab.Studio.Projects[i]
		}
	}
	return nil
}
func (e *Engine) conceptProject(id string) (ConceptProject, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	p := e.projectLocked(id)
	if p == nil {
		return ConceptProject{}, errors.New("concept project not found")
	}
	return cloneProject(*p), nil
}
func shortText(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) > max {
		r = r[:max]
	}
	return string(r)
}
func cleanAttributes(in []ConceptAttribute) ([]ConceptAttribute, error) {
	if len(in) > 20 {
		return nil, errors.New("use at most 20 concept attributes")
	}
	out := []ConceptAttribute{}
	seen := map[string]bool{}
	for _, a := range in {
		a.Name = shortText(a.Name, 60)
		a.Value = shortText(a.Value, 200)
		if a.Name == "" || a.Value == "" {
			continue
		}
		if seen[a.Name] {
			return nil, errors.New("attribute names must be distinct")
		}
		seen[a.Name] = true
		out = append(out, a)
	}
	return out, nil
}
func conceptPrompt(p ConceptProject) string {
	parts := []string{p.Subject}
	for _, a := range p.Attributes {
		parts = append(parts, a.Name+": "+a.Value)
	}
	return strings.Join(parts, ", ")
}

// Explicit spatial features, unlike the legacy byte/histogram classifier. This
// remains a small classifier and is not a semantic foundation vision model.
func conceptImage(b []byte) (image.Image, []float64, uint64, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return nil, nil, 0, errors.New("use a decodable PNG or JPEG image")
	}
	if (format != "png" && format != "jpeg") || cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width)*int64(cfg.Height) > 16_000_000 {
		return nil, nil, 0, errors.New("use PNG/JPEG images of at most 16 million pixels")
	}
	im, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, nil, 0, err
	}
	v := make([]float64, 64)
	counts := make([]float64, 16)
	bounds := im.Bounds()
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			r, g, bl, _ := im.At(bounds.Min.X+x*bounds.Dx()/64, bounds.Min.Y+y*bounds.Dy()/64).RGBA()
			cell := (y/16)*4 + x/16
			v[cell*3] += float64(r) / 65535
			v[cell*3+1] += float64(g) / 65535
			v[cell*3+2] += float64(bl) / 65535
			counts[cell]++
			lum := (r + g + bl) / 3
			v[48+minInt(15, int(lum>>12))] += 1.0 / 4096
		}
	}
	for i := 0; i < 48; i++ {
		v[i] /= counts[i/3]
	}
	lum := func(x, y int) uint32 {
		r, g, b, _ := im.At(bounds.Min.X+x*bounds.Dx()/9, bounds.Min.Y+y*bounds.Dy()/8).RGBA()
		return (r + g + b) / 3
	}
	var dh uint64
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if lum(x, y) > lum(x+1, y) {
				dh |= 1 << uint(y*8+x)
			}
		}
	}
	return im, v, dh, nil
}
func conceptGroups(p *ConceptProject) map[string]string {
	m := map[string]string{}
	for _, x := range p.Examples {
		m[x.Group] = x.Split
	}
	return m
}
func (e *Engine) addConceptImage(id string, b []byte, name string, source ConceptSource) (ConceptExample, error) {
	var empty ConceptExample
	source.URL = shortText(source.URL, 2048)
	source.Page = shortText(source.Page, 2048)
	source.Creator = shortText(source.Creator, 200)
	source.LicenseURL = shortText(source.LicenseURL, 2048)
	source.Query = shortText(source.Query, 240)
	if !openLicense(source.License) && source.License != "owned" && source.License != "permission" {
		source.License = "unknown"
	}
	if len(b) > conceptImageLimit {
		return empty, errors.New("concept images must be at most 12 MB")
	}
	im, v, dh, err := conceptImage(b)
	if err != nil {
		return empty, err
	}
	p, err := e.conceptProject(id)
	if err != nil {
		return empty, err
	}
	if len(p.Examples) >= 500 {
		return empty, errors.New("project limit reached: export or create a new project")
	}
	mime := http.DetectContentType(b)
	a, err := e.storeObject(bytes.NewReader(b), shortText(name, 120), mime, "concept:"+id)
	if err != nil {
		return empty, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	live := e.projectLocked(id)
	if live == nil {
		return empty, errors.New("project was removed")
	}
	for _, x := range live.Examples {
		if x.Asset == a.ID {
			return x, nil
		}
	}
	if len(live.Examples) >= 500 {
		return empty, errors.New("project limit reached")
	}
	group := a.ID
	split := ""
	for _, x := range live.Examples { // dHash and spatial distance reduce false matches across solid colours.
		if (source.URL != "" && x.Source.URL == source.URL) || (bits.OnesCount64(x.DHash^dh) <= 2 && referenceDistance(x.Features, v, "l1") < 0.025) {
			group = x.Group
			split = x.Split
			break
		}
	}
	if split == "" {
		split = []string{"train", "train", "train", "validation", "audit"}[len(conceptGroups(live))%5]
	}
	x := ConceptExample{Asset: a.ID, Name: shortText(name, 120), Caption: shortText(name, 1000), Group: group, Split: split, Review: "pending", Source: source, Width: im.Bounds().Dx(), Height: im.Bounds().Dy(), DHash: dh, Features: v}
	live.Examples = append(live.Examples, x)
	live.Revision++
	live.Note = "New examples await caption and relevance review. Collection alone does not train a generator."
	return x, nil
}
func openLicense(s string) bool { return s == "cc0" || s == "pdm" || s == "by" || s == "by-sa" }
func (s *ConceptStudio) search(ctx context.Context, provider, query string, limit int) ([]ImageSearchResult, error) {
	var rows []ImageSearchResult
	if provider == "openverse" {
		u := "https://api.openverse.org/v1/images/?" + url.Values{"q": {query}, "page_size": {strconv.Itoa(limit)}, "license": {"cc0,pdm,by,by-sa"}}.Encode()
		b, err := webBytes(ctx, s.searchClient, u, 2<<20)
		if err != nil {
			return nil, err
		}
		var res struct {
			Results []struct {
				ID, Title, URL, Creator, License string
				Page                             string `json:"foreign_landing_url"`
				LicenseURL                       string `json:"license_url"`
			}
		}
		if err = json.Unmarshal(b, &res); err != nil {
			return nil, err
		}
		for _, x := range res.Results {
			if len(rows) >= limit {
				break
			}
			if _, err = publicWebURL(x.URL); err != nil {
				continue
			}
			rows = append(rows, ImageSearchResult{ID: shortText(x.ID, 100), Title: shortText(x.Title, 300), Source: ConceptSource{URL: x.URL, Page: shortText(x.Page, 2000), Creator: shortText(x.Creator, 200), License: strings.ToLower(x.License), LicenseURL: shortText(x.LicenseURL, 2000), Query: query}})
		}
	} else if provider == "searxng" {
		s.mu.Lock()
		base := s.searxURL
		s.mu.Unlock()
		if base == "" {
			return nil, errors.New("configure your SearXNG URL first; its JSON search API must be enabled")
		}
		client := s.searchClient
		u, _ := url.Parse(base)
		if net.ParseIP(u.Hostname()).IsLoopback() {
			client = &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("local search redirects disabled") }}
		}
		b, err := webBytes(ctx, client, base+"/search?"+url.Values{"q": {query}, "categories": {"images"}, "format": {"json"}}.Encode(), 2<<20)
		if err != nil {
			return nil, err
		}
		var res struct {
			Results []struct {
				Title, URL string
				Image      string `json:"img_src"`
			}
		}
		if err = json.Unmarshal(b, &res); err != nil {
			return nil, err
		}
		for _, x := range res.Results {
			if len(rows) >= limit {
				break
			}
			if _, err = publicWebURL(x.Image); err != nil {
				continue
			}
			rows = append(rows, ImageSearchResult{ID: shortHash(x.Image), Title: shortText(x.Title, 300), Source: ConceptSource{URL: x.Image, Page: shortText(x.URL, 2000), License: "unknown", Query: query}, Message: "Check the source licence before importing; general search supplies no reuse permission."})
		}
	} else {
		return nil, errors.New("choose Openverse or your SearXNG instance")
	}
	return rows, nil
}
func (s *ConceptStudio) collect(project, provider, query string, limit int, download bool) (ConceptCollection, error) {
	if _, err := s.e.conceptProject(project); err != nil {
		return ConceptCollection{}, err
	}
	query = shortText(query, 240)
	if query == "" || limit < 1 || limit > 24 {
		return ConceptCollection{}, errors.New("enter a query and a limit of 1–24 images")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.cancel != nil {
		return ConceptCollection{}, errors.New("stop or finish the current collection first")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	s.cancel = cancel
	s.collection = ConceptCollection{ID: randomID()[:16], Project: project, Query: query, Status: "searching", Message: "Searching the selected image index."}
	initial := s.collection
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer cancel()
		rows, err := s.search(ctx, provider, query, limit)
		if err == nil {
			s.mu.Lock()
			s.collection.Results = rows
			s.collection.Status = "collecting"
			s.mu.Unlock()
			for i, row := range rows {
				if ctx.Err() != nil {
					break
				}
				if !download || !openLicense(row.Source.License) {
					continue
				}
				if i > 0 {
					select {
					case <-ctx.Done():
					case <-time.After(700 * time.Millisecond):
					}
				}
				if ctx.Err() != nil {
					break
				}
				b, er := webBytes(ctx, s.imageClient, row.Source.URL, conceptImageLimit)
				var x ConceptExample
				if er == nil {
					x, er = s.e.addConceptImage(project, b, row.Title, row.Source)
				}
				s.mu.Lock()
				if er != nil {
					s.collection.Results[i].Message = shortText(er.Error(), 400)
				} else {
					s.collection.Results[i].Asset = x.Asset
					s.collection.Results[i].Message = "Saved locally; review the content and caption before training."
					s.collection.Downloaded++
				}
				s.mu.Unlock()
			}
		}
		s.mu.Lock()
		s.collection.Status = "completed"
		s.collection.Message = "Collection finished. Review examples, labels and captions in the dataset."
		if err != nil {
			s.collection.Status = "failed"
			s.collection.Message = shortText(err.Error(), 600)
		}
		if ctx.Err() != nil {
			s.collection.Status = "stopped"
			s.collection.Message = "Collection stopped. Successfully saved examples remain available."
		}
		s.cancel = nil
		s.mu.Unlock()
		s.e.persist()
	}()
	return initial, nil
}
func projectLearning(p ConceptProject) LearningState {
	ls := LearningState{Revision: p.Revision, Active: p.Active}
	for _, x := range p.Examples {
		if x.Review != "accepted" {
			continue
		}
		ls.Examples = append(ls.Examples, TrainingExample{Asset: x.Asset, Label: x.Label, Caption: x.Caption, Group: x.Group, Split: x.Split, Kind: "image", Features: x.Features})
	}
	return ls
}
func (e *Engine) trainConcept(id string) (LearningRun, error) {
	e.learningMu.Lock()
	defer e.learningMu.Unlock()
	p, err := e.conceptProject(id)
	if err != nil {
		return LearningRun{}, err
	}
	if e.paused.Load() {
		return LearningRun{}, errors.New("resume the engine to train")
	}
	if p.Evaluated == p.Revision && len(p.Runs) > 0 {
		return p.Runs[len(p.Runs)-1], nil
	}
	ls := projectLearning(p)
	train, val, audit, err := learningSets(ls)
	if err != nil {
		e.mu.Lock()
		if x := e.projectLocked(id); x != nil {
			x.Note = err.Error()
		}
		e.mu.Unlock()
		return LearningRun{}, err
	}
	run := LearningRun{ID: randomID()[:16], Revision: p.Revision, Created: now(), Baseline: scoreRecognition(p.Active, val)}
	best := -1.0
	var winner RecognitionModel
	for _, method := range []string{"centroid", "knn1", "knn3"} {
		for _, metric := range []string{"l1", "l2"} {
			m := RecognitionModel{ID: randomID()[:16], Method: method, Metric: metric, Revision: p.Revision, Samples: train, Created: now()}
			score := scoreRecognition(m, val)
			run.Candidates = append(run.Candidates, score)
			if score.BalancedAccuracy > best {
				best = score.BalancedAccuracy
				winner = m
				run.Validation = score
			}
		}
	}
	run.Audit = scoreRecognition(winner, audit)
	run.Promoted = best > run.Baseline.BalancedAccuracy+1e-9 && best >= 0.6
	run.Reason = "Kept the prior classifier: no measured validation improvement. Add varied examples."
	if run.Promoted {
		run.Reason = "Promoted a small spatial-feature classifier using validation accuracy. Audit examples were not used for selection. This does not train diffusion weights."
	}
	e.mu.Lock()
	live := e.projectLocked(id)
	if live == nil || live.Revision != p.Revision {
		e.mu.Unlock()
		return run, errors.New("dataset changed during training; evaluate its new revision")
	}
	if run.Promoted {
		live.Previous = live.Active
		live.Active = winner
	}
	live.Evaluated = p.Revision
	live.Runs = append(live.Runs, run)
	if len(live.Runs) > 20 {
		live.Runs = live.Runs[len(live.Runs)-20:]
	}
	live.Note = run.Reason
	e.mu.Unlock()
	e.persist()
	return run, nil
}
func (e *Engine) reviewConcept(project, asset, label, caption, review, license string) error {
	label = shortText(label, 100)
	caption = shortText(caption, 1000)
	if license != "" && !openLicense(license) && license != "owned" && license != "permission" && license != "unknown" {
		return errors.New("invalid source licence or permission")
	}
	if review != "accepted" && review != "rejected" && review != "pending" {
		return errors.New("invalid review state")
	}
	if review == "accepted" && (label == "" || caption == "") {
		return errors.New("accepted examples need a category and an accurate caption")
	}
	e.mu.Lock()
	p := e.projectLocked(project)
	if p == nil {
		e.mu.Unlock()
		return errors.New("project not found")
	}
	found := false
	for i := range p.Examples {
		if p.Examples[i].Asset != asset {
			continue
		}
		x := p.Examples[i]
		if license != "" {
			x.Source.License = license
		}
		if review == "accepted" && !openLicense(x.Source.License) && x.Source.License != "owned" && x.Source.License != "permission" {
			e.mu.Unlock()
			return errors.New("record the image's licence or your permission before accepting it")
		}
		x.Label = label
		x.Caption = caption
		x.Review = review
		p.Examples[i] = x
		found = true
		break
	}
	if !found {
		e.mu.Unlock()
		return errors.New("example not found")
	}
	p.Revision++
	auto := p.AutoLearn
	e.mu.Unlock()
	if auto {
		_, _ = e.trainConcept(project)
	}
	e.persist()
	return nil
}

// Exact synthetic curricula are useful labelled exercises, never counted as
// evidence that the same classifier understands photographs or human attributes.
func (e *Engine) seedConcept(id, axis string) (int, error) {
	labels := map[string][]string{"horizontal": {"left", "right"}, "vertical": {"above", "below"}, "power": {"on", "off"}, "containment": {"inside", "outside"}, "colour": {"red", "blue"}}[axis]
	if labels == nil {
		return 0, errors.New("choose horizontal, vertical, power, containment or colour")
	}
	if _, err := e.conceptProject(id); err != nil {
		return 0, err
	}
	batch := randomID()[:16]
	count := 0
	for class, label := range labels {
		for i := 0; i < 10; i++ {
			r := rand.New(rand.NewSource(int64(i*37 + class*991 + len(axis)*131)))
			im := image.NewRGBA(image.Rect(0, 0, 128, 128))
			bg := color.RGBA{238, 240, 244, 255}
			draw.Draw(im, im.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
			x, y := 24+r.Intn(16), 42+r.Intn(30)
			col := color.RGBA{220, 35, 55, 255}
			size := 18 + r.Intn(14)
			switch axis {
			case "horizontal":
				if class == 1 {
					x = 78 + r.Intn(12)
				}
			case "vertical":
				x = 42 + r.Intn(28)
				y = 14 + r.Intn(12)
				if class == 1 {
					y = 82 + r.Intn(10)
				}
			case "power":
				x = 35 + r.Intn(25)
				y = 35 + r.Intn(25)
				if class == 0 {
					col = color.RGBA{252, 200, 30, 255}
				} else {
					col = color.RGBA{45, 48, 54, 255}
				}
			case "colour":
				x = 22 + r.Intn(60)
				y = 22 + r.Intn(60)
				if class == 1 {
					col = color.RGBA{35, 75, 220, 255}
				}
			case "containment":
				draw.Draw(im, image.Rect(30, 30, 100, 100), &image.Uniform{color.RGBA{40, 45, 50, 255}}, image.Point{}, draw.Src)
				draw.Draw(im, image.Rect(34, 34, 96, 96), &image.Uniform{bg}, image.Point{}, draw.Src)
				size = 12 + r.Intn(8)
				x = 42 + r.Intn(28)
				y = 42 + r.Intn(28)
				if class == 1 {
					x = 4 + r.Intn(7)
					y = 5 + r.Intn(75)
				}
			}
			draw.Draw(im, image.Rect(x, y, x+size, y+size), &image.Uniform{col}, image.Point{}, draw.Src)
			var b bytes.Buffer
			_ = png.Encode(&b, im)
			ex, err := e.addConceptImage(id, b.Bytes(), "Synthetic "+axis+" "+label+" "+strconv.Itoa(i)+".png", ConceptSource{License: "cc0", Creator: "ORIGIN-0 synthetic curriculum"})
			if err != nil {
				return count, err
			}
			e.mu.Lock()
			p := e.projectLocked(id)
			for j := range p.Examples {
				v := &p.Examples[j]
				if v.Asset == ex.Asset {
					v.Label = label
					v.Caption = "A synthetic diagram illustrating " + axis + ": " + label + "."
					v.Review = "accepted"
					v.Group = "synthetic-" + batch + "-" + strconv.Itoa(class) + "-" + strconv.Itoa(i)
					v.Split = []string{"train", "train", "train", "validation", "audit"}[i%5]
				}
			}
			p.Revision++
			e.mu.Unlock()
			count++
		}
	}
	_, _ = e.trainConcept(id)
	e.persist()
	return count, nil
}

func (e *Engine) conceptTrainingRows(id string) ([]map[string]string, []map[string]string, int, error) {
	p, err := e.conceptProject(id)
	if err != nil {
		return nil, nil, 0, err
	}
	train, val := []map[string]string{}, []map[string]string{}
	for _, x := range p.Examples {
		if x.Review != "accepted" || x.Caption == "" || x.Split == "audit" {
			continue
		}
		path, er := e.objectPath(x.Asset)
		if er != nil {
			return nil, nil, 0, er
		}
		if _, er = os.Stat(path); er != nil {
			return nil, nil, 0, er
		}
		row := map[string]string{"path": path, "caption": x.Caption, "asset": x.Asset, "group": x.Group}
		if x.Split == "train" {
			train = append(train, row)
		} else if x.Split == "validation" {
			val = append(val, row)
		}
	}
	if len(train) < 3 || len(val) < 1 {
		return nil, nil, 0, errors.New("image adapter training needs three accepted training images and one separate validation image")
	}
	return train, val, p.Revision, nil
}
func conceptRecipe(p ConceptProject) map[string]any {
	examples := []map[string]any{}
	for _, x := range p.Examples {
		if x.Review != "accepted" {
			continue
		}
		examples = append(examples, map[string]any{"sha256": x.Asset, "label": x.Label, "caption": x.Caption, "group": x.Group, "split": x.Split, "source": x.Source})
	}
	return map[string]any{"schema": "origin0.concept.v1", "name": p.Name, "subject": p.Subject, "attributes": p.Attributes, "prompt": conceptPrompt(p), "examples": examples, "evaluation": p.Runs, "note": "Recipe metadata only. Image bytes and model weights are not included. Source licences and permissions apply; inspect before publishing."}
}
func (e *Engine) exportConcept(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string
		Images bool
	}
	if err := decode(r, &req); err != nil {
		apiError(w, err)
		return
	}
	p, err := e.conceptProject(req.ID)
	if err != nil {
		apiError(w, err)
		return
	}
	if !req.Images {
		w.Header().Set("Content-Disposition", "attachment; filename=ORIGIN0_CONCEPT_RECIPE.json")
		jsonReply(w, conceptRecipe(p))
		return
	}
	for _, x := range p.Examples {
		if x.Review != "accepted" {
			continue
		}
		path, er := e.objectPath(x.Asset)
		if er != nil {
			apiError(w, er)
			return
		}
		if _, er = os.Stat(path); er != nil {
			apiError(w, er)
			return
		}
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=ORIGIN0_CONCEPT_DATASET.zip")
	z := zip.NewWriter(w)
	defer z.Close()
	recipe, _ := json.MarshalIndent(conceptRecipe(p), "", "  ")
	f, err := z.Create("concept.json")
	if err != nil {
		return
	}
	if _, err = f.Write(recipe); err != nil {
		return
	}
	meta := map[string]*bytes.Buffer{}
	for _, split := range []string{"train", "validation", "audit"} {
		meta[split] = new(bytes.Buffer)
	}
	for _, x := range p.Examples {
		if x.Review != "accepted" {
			continue
		}
		path, _ := e.objectPath(x.Asset)
		file, er := os.Open(path)
		if er != nil {
			return
		}
		cfg, format, er := image.DecodeConfig(file)
		_ = cfg
		if er != nil {
			file.Close()
			return
		}
		_, _ = file.Seek(0, 0)
		ext := ".png"
		if format == "jpeg" {
			ext = ".jpg"
		}
		name := "images/" + x.Asset + ext
		f, er := z.Create(x.Split + "/" + name)
		if er != nil {
			file.Close()
			return
		}
		_, er = io.Copy(f, file)
		file.Close()
		if er != nil {
			return
		}
		_ = json.NewEncoder(meta[x.Split]).Encode(map[string]string{"file_name": name, "text": x.Caption, "label": x.Label, "group": x.Group, "sha256": x.Asset})
	}
	for _, split := range []string{"train", "validation", "audit"} {
		f, er := z.Create(split + "/metadata.jsonl")
		if er != nil {
			return
		}
		if _, er = f.Write(meta[split].Bytes()); er != nil {
			return
		}
	}
}

func (e *Engine) conceptRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/concepts/state", func(w http.ResponseWriter, r *http.Request) {
		e.mu.RLock()
		b, _ := json.Marshal(e.lab.Studio)
		e.mu.RUnlock()
		var state ConceptState
		_ = json.Unmarshal(b, &state)
		for i := range state.Projects {
			p := &state.Projects[i]
			for j := range p.Examples {
				p.Examples[j].Features = nil
			}
			p.Active.Samples = nil
			p.Previous.Samples = nil
		}
		e.studio.mu.Lock()
		job := e.studio.collection
		job.Results = append([]ImageSearchResult{}, job.Results...)
		searx := e.studio.searxURL
		e.studio.mu.Unlock()
		jsonReply(w, map[string]any{"projects": state.Projects, "collection": job, "searxng": searx, "feature_model": "spatial-rgb-v1", "image_training": "Optional SDXL LoRA worker; needs configured Python, compatible model weights and CUDA. Native Z-Image weights are not trained by this classifier."})
	})
	mux.HandleFunc("/api/concepts/save", func(w http.ResponseWriter, r *http.Request) {
		var x struct {
			ID, Name, Subject string
			Attributes        []ConceptAttribute
			AutoLearn         bool `json:"auto_learn"`
		}
		if err := decode(r, &x); err != nil {
			apiError(w, err)
			return
		}
		x.Name = shortText(x.Name, 100)
		x.Subject = shortText(x.Subject, 1500)
		attrs, err := cleanAttributes(x.Attributes)
		if err != nil || x.Name == "" || x.Subject == "" {
			apiError(w, errors.New("enter a name, subject and at most 20 distinct attributes"))
			return
		}
		e.mu.Lock()
		p := e.projectLocked(x.ID)
		if p == nil {
			if x.ID != "" || len(e.lab.Studio.Projects) >= 24 {
				e.mu.Unlock()
				apiError(w, errors.New("project not found or 24-project limit reached"))
				return
			}
			e.lab.Studio.Projects = append(e.lab.Studio.Projects, ConceptProject{ID: randomID()[:16], Created: now(), Examples: []ConceptExample{}, Note: "Add and review varied examples. Prompt controls do not retrain model weights."})
			p = &e.lab.Studio.Projects[len(e.lab.Studio.Projects)-1]
		}
		p.Name = x.Name
		p.Subject = x.Subject
		p.Attributes = attrs
		p.AutoLearn = x.AutoLearn
		id := p.ID
		prompt := conceptPrompt(*p)
		e.mu.Unlock()
		e.persist()
		jsonReply(w, map[string]string{"id": id, "prompt": prompt})
	})
	mux.HandleFunc("/api/concepts/search", func(w http.ResponseWriter, r *http.Request) {
		var x struct {
			ID, Provider, Query string
			Limit               int
			Download            bool
		}
		if err := decode(r, &x); err != nil {
			apiError(w, err)
			return
		}
		j, err := e.studio.collect(x.ID, x.Provider, x.Query, x.Limit, x.Download)
		if err != nil {
			apiError(w, err)
			return
		}
		jsonReply(w, j)
	})
	mux.HandleFunc("/api/concepts/stop", func(w http.ResponseWriter, r *http.Request) {
		e.studio.mu.Lock()
		if e.studio.cancel != nil {
			e.studio.cancel()
		}
		e.studio.mu.Unlock()
		w.WriteHeader(204)
	})
	mux.HandleFunc("/api/concepts/searxng", func(w http.ResponseWriter, r *http.Request) {
		var x struct{ URL string }
		if err := decode(r, &x); err != nil {
			apiError(w, err)
			return
		}
		raw := strings.TrimRight(strings.TrimSpace(x.URL), "/")
		if raw != "" {
			u, err := url.Parse(raw)
			if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
				apiError(w, errors.New("invalid SearXNG base URL"))
				return
			}
			local := net.ParseIP(u.Hostname()).IsLoopback()
			if !(local && (u.Scheme == "http" || u.Scheme == "https")) {
				if _, err = publicWebURL(raw); err != nil {
					apiError(w, err)
					return
				}
			}
		}
		e.studio.mu.Lock()
		e.studio.searxURL = raw
		e.studio.mu.Unlock()
		w.WriteHeader(204)
	})
	mux.HandleFunc("/api/concepts/add", func(w http.ResponseWriter, r *http.Request) {
		var x struct {
			ID, Asset, Name string
			Source          ConceptSource
		}
		if err := decode(r, &x); err != nil {
			apiError(w, err)
			return
		}
		path, err := e.objectPath(x.Asset)
		if err != nil {
			apiError(w, err)
			return
		}
		f, err := os.Open(path)
		if err != nil {
			apiError(w, err)
			return
		}
		b, err := io.ReadAll(io.LimitReader(f, conceptImageLimit+1))
		f.Close()
		if err != nil {
			apiError(w, err)
			return
		}
		if x.Source.License == "" {
			x.Source.License = "unknown"
		}
		row, err := e.addConceptImage(x.ID, b, x.Name, x.Source)
		if err != nil {
			apiError(w, err)
			return
		}
		e.persist()
		jsonReply(w, row)
	})
	mux.HandleFunc("/api/concepts/import-url", func(w http.ResponseWriter, r *http.Request) {
		var x struct {
			ID, Name string
			Source   ConceptSource
		}
		if err := decode(r, &x); err != nil {
			apiError(w, err)
			return
		}
		if !openLicense(x.Source.License) && x.Source.License != "owned" && x.Source.License != "permission" {
			apiError(w, errors.New("record a source licence or permission first"))
			return
		}
		if _, err := publicWebURL(x.Source.URL); err != nil {
			apiError(w, err)
			return
		}
		b, err := webBytes(r.Context(), e.studio.imageClient, x.Source.URL, conceptImageLimit)
		if err != nil {
			apiError(w, err)
			return
		}
		row, err := e.addConceptImage(x.ID, b, x.Name, x.Source)
		if err != nil {
			apiError(w, err)
			return
		}
		e.persist()
		jsonReply(w, row)
	})
	mux.HandleFunc("/api/concepts/review", func(w http.ResponseWriter, r *http.Request) {
		var x struct{ ID, Asset, Label, Caption, Review, License string }
		if err := decode(r, &x); err != nil {
			apiError(w, err)
			return
		}
		if err := e.reviewConcept(x.ID, x.Asset, x.Label, x.Caption, x.Review, x.License); err != nil {
			apiError(w, err)
			return
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("/api/concepts/train", func(w http.ResponseWriter, r *http.Request) {
		var x struct{ ID string }
		if err := decode(r, &x); err != nil {
			apiError(w, err)
			return
		}
		run, err := e.trainConcept(x.ID)
		if err != nil {
			apiError(w, err)
			return
		}
		jsonReply(w, run)
	})
	mux.HandleFunc("/api/concepts/rollback", func(w http.ResponseWriter, r *http.Request) {
		var x struct{ ID string }
		if err := decode(r, &x); err != nil {
			apiError(w, err)
			return
		}
		e.mu.Lock()
		p := e.projectLocked(x.ID)
		if p == nil || p.Previous.ID == "" {
			e.mu.Unlock()
			apiError(w, errors.New("no prior project classifier"))
			return
		}
		p.Active, p.Previous = p.Previous, p.Active
		p.AutoLearn = false
		p.Note = "Previous classifier restored; automatic training paused."
		e.mu.Unlock()
		e.persist()
		w.WriteHeader(204)
	})
	mux.HandleFunc("/api/concepts/predict", func(w http.ResponseWriter, r *http.Request) {
		var x struct{ ID, Asset string }
		if err := decode(r, &x); err != nil {
			apiError(w, err)
			return
		}
		p, err := e.conceptProject(x.ID)
		if err != nil {
			apiError(w, err)
			return
		}
		if p.Active.ID == "" {
			apiError(w, errors.New("train this project's classifier first"))
			return
		}
		path, err := e.objectPath(x.Asset)
		if err != nil {
			apiError(w, err)
			return
		}
		f, err := os.Open(path)
		if err != nil {
			apiError(w, err)
			return
		}
		b, err := io.ReadAll(io.LimitReader(f, conceptImageLimit+1))
		f.Close()
		if err != nil {
			apiError(w, err)
			return
		}
		_, v, _, err := conceptImage(b)
		if err != nil {
			apiError(w, err)
			return
		}
		label, score := modelPredict(p.Active, v)
		jsonReply(w, map[string]any{"label": label, "score": score, "note": "Small spatial-feature prediction; score is not a calibrated probability."})
	})
	mux.HandleFunc("/api/concepts/seed", func(w http.ResponseWriter, r *http.Request) {
		var x struct{ ID, Axis string }
		if err := decode(r, &x); err != nil {
			apiError(w, err)
			return
		}
		count, err := e.seedConcept(x.ID, x.Axis)
		if err != nil {
			apiError(w, err)
			return
		}
		jsonReply(w, map[string]int{"examples": count})
	})
	mux.HandleFunc("/api/concepts/prompt", func(w http.ResponseWriter, r *http.Request) {
		var x struct{ ID string }
		if err := decode(r, &x); err != nil {
			apiError(w, err)
			return
		}
		p, err := e.conceptProject(x.ID)
		if err != nil {
			apiError(w, err)
			return
		}
		jsonReply(w, map[string]string{"prompt": conceptPrompt(p)})
	})
	mux.HandleFunc("/api/concepts/export", e.exportConcept)
}

// Keep exported metadata in a stable order for readable community diffs.
func canonicalRecipe(p ConceptProject) ([]byte, error) {
	r := conceptRecipe(p)
	rows := r["examples"].([]map[string]any)
	sort.Slice(rows, func(i, j int) bool { return rows[i]["sha256"].(string) < rows[j]["sha256"].(string) })
	return json.MarshalIndent(r, "", "  ")
}
func recipeDigest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
