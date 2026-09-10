package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type ImageRatings struct {
	Match       int    `json:"match"`
	Quality     int    `json:"quality"`
	Consistency int    `json:"consistency"`
	Speed       int    `json:"speed"`
	Notes       string `json:"notes"`
	Updated     int64  `json:"updated"`
}

func validAssetID(id string) bool {
	if len(id) != 64 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}
func (s *NativeImages) referenceImage(id string) ([]byte, error) {
	path, err := s.e.objectPath(id)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, conceptImageLimit+1))
	if err != nil {
		return nil, err
	}
	im, _, _, err := conceptImage(b)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err = png.Encode(&out, im); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
func (s *NativeImages) rateImage(id string, ratings ImageRatings) error {
	for _, n := range []int{ratings.Match, ratings.Quality, ratings.Consistency, ratings.Speed} {
		if n < 1 || n > 5 {
			return errors.New("all four ratings must be from 1 to 5")
		}
	}
	if len(ratings.Notes) > 1000 {
		return errors.New("keep feedback within 1000 bytes")
	}
	s.mu.Lock()
	found := false
	for _, j := range s.jobs {
		if j.ID == id && j.Status == "completed" {
			ratings.Updated = now()
			j.Ratings = &ratings
			found = true
			break
		}
	}
	s.mu.Unlock()
	if !found {
		return errors.New("completed image not found")
	}
	s.save()
	return nil
}

var stepProgress = regexp.MustCompile(`(?m)(\d+)\s*/\s*(\d+)\s*[-|]`)

func imageProgress(j ImageJob) map[string]any {
	if j.Status == "completed" {
		return map[string]any{"percent": 100, "stage": "completed"}
	}
	rows := stepProgress.FindAllStringSubmatch(j.Log, -1)
	if len(rows) > 0 {
		v := rows[len(rows)-1]
		n, _ := strconv.Atoi(v[1])
		d, _ := strconv.Atoi(v[2])
		if d > 0 && n <= d {
			return map[string]any{"percent": minInt(99, 100*n/d), "stage": "sampling"}
		}
	}
	return map[string]any{"percent": nil, "stage": j.Status}
}
func (s *NativeImages) studioRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/images/rated-settings", func(w http.ResponseWriter, r *http.Request) {
		var q ImageRequest
		if err := decode(r, &q); err != nil {
			apiError(w, err)
			return
		}
		if err := validateImageRequest(q); err != nil {
			apiError(w, err)
			return
		}
		s.mu.Lock()
		rec, n := ratedSettings(s.jobs, q, s.catalog.ID)
		s.mu.Unlock()
		jsonReply(w, map[string]any{"request": rec, "examples": n, "applied": n >= 3, "note": "Uses at least three rated results for this exact prompt and reference. It learns your setting preference; it does not train image-model weights or prove a causal quality improvement."})
	})
	mux.HandleFunc("/api/images/feedback", func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			ID      string
			Ratings ImageRatings
		}
		if err := decode(r, &q); err != nil {
			apiError(w, err)
			return
		}
		if err := s.rateImage(q.ID, q.Ratings); err != nil {
			apiError(w, err)
			return
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("/api/images/metadata", func(w http.ResponseWriter, r *http.Request) {
		var q struct{ ID string }
		if err := decode(r, &q); err != nil {
			apiError(w, err)
			return
		}
		s.mu.Lock()
		var b []byte
		for _, j := range s.jobs {
			if j.ID == q.ID {
				b, _ = json.Marshal(map[string]any{"schema": "origin0.image.v1", "request": j.Request, "model": s.catalog, "backend": j.Backend, "elapsed_seconds": j.Finished - j.Started, "ratings": j.Ratings, "asset": j.Asset})
				break
			}
		}
		s.mu.Unlock()
		if b == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=ORIGIN0_IMAGE_SETTINGS.json")
		w.Write(b)
	})
	mux.HandleFunc("/api/images/preview", func(w http.ResponseWriter, r *http.Request) {
		var q struct{ ID string }
		if err := decode(r, &q); err != nil {
			apiError(w, err)
			return
		}
		s.mu.Lock()
		ok := false
		for _, j := range s.jobs {
			if j.ID == q.ID && j.Request.Preview && !j.Request.Shared {
				ok = true
				break
			}
		}
		s.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		f, err := os.Open(filepath.Join(s.root(), "runs", q.ID, "preview.png"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		b, err := io.ReadAll(io.LimitReader(f, 4<<20))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(b))
		if err != nil || cfg.Width > 1024 || cfg.Height > 1024 {
			http.NotFound(w, r)
			return
		}
		if _, err = png.Decode(bytes.NewReader(b)); err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(b)
	})
}

func ratedSettings(jobs []*ImageJob, req ImageRequest, pack string) (ImageRequest, int) {
	type aggregate struct {
		req   ImageRequest
		n     int
		score float64
	}
	groups := map[string]*aggregate{}
	prompt := strings.Join(strings.Fields(strings.ToLower(req.Prompt)), " ")
	for _, j := range jobs {
		if j.Status != "completed" || j.Pack != pack || j.Ratings == nil || j.Request.InitAsset != req.InitAsset || strings.Join(strings.Fields(strings.ToLower(j.Request.Prompt)), " ") != prompt {
			continue
		}
		k := fmt.Sprintf("%d/%d/%d/%.3f", j.Request.Width, j.Request.Height, j.Request.Steps, j.Request.Strength)
		a := groups[k]
		if a == nil {
			a = &aggregate{req: j.Request}
			groups[k] = a
		}
		a.n++
		v := j.Ratings
		a.score += float64(2*v.Match+2*v.Quality+v.Consistency+v.Speed) / 6
	}
	var best *aggregate
	bestScore := 0.0
	for _, a := range groups {
		score := a.score / float64(a.n)
		if a.n >= 3 && (score > bestScore || (score == bestScore && best != nil && a.n > best.n)) {
			best = a
			bestScore = score
		}
	}
	if best == nil {
		return req, 0
	}
	req.Width = best.req.Width
	req.Height = best.req.Height
	req.Steps = best.req.Steps
	req.Strength = best.req.Strength
	return req, best.n
}
