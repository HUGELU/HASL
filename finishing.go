package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type FinishOptions struct {
	Mode      string  `json:"mode"`
	LongEdge  int     `json:"long_edge"`
	Sharpness float64 `json:"sharpness"`
	Detail    float64 `json:"detail"`
	Backend   string  `json:"backend"`
}
type FinishRequest struct {
	Asset string `json:"asset"`
	FinishOptions
}
type FinishJob struct {
	ID       string        `json:"id"`
	Request  FinishRequest `json:"request"`
	Status   string        `json:"status"`
	Message  string        `json:"message"`
	Created  int64         `json:"created"`
	Finished int64         `json:"finished"`
	Progress int           `json:"progress"`
	Key      string        `json:"key"`
	Asset    *AssetRecord  `json:"asset,omitempty"`
	Recipe   *AssetRecord  `json:"recipe,omitempty"`
	Seconds  float64       `json:"seconds"`
	Width    int           `json:"width"`
	Height   int           `json:"height"`
	Backend  string        `json:"backend"`
	Cached   bool          `json:"cached"`
	Log      string        `json:"log"`
}
type Finishing struct {
	e         *Engine
	mu        sync.Mutex
	persistMu sync.Mutex
	wg        sync.WaitGroup
	closed    bool
	cancel    context.CancelFunc
	jobs      []*FinishJob
	catalog   struct {
		Model    string                  `json:"model"`
		Runtimes map[string]DownloadSpec `json:"runtimes"`
	}
}

func newFinishing(e *Engine) *Finishing {
	s := &Finishing{e: e}
	b, _ := assets.ReadFile("upscale_manifest.json")
	_ = json.Unmarshal(b, &s.catalog)
	if b, err := os.ReadFile(filepath.Join(e.dataDir, "finishing.json")); err == nil {
		_ = json.Unmarshal(b, &s.jobs)
	}
	for _, j := range s.jobs {
		if j.Status == "running" || j.Status == "queued" {
			j.Status = "interrupted"
			j.Message = "Previous process ended; the source is retained."
		}
	}
	return s
}
func (s *Finishing) save() {
	s.persistMu.Lock()
	defer s.persistMu.Unlock()
	s.mu.Lock()
	b, _ := json.Marshal(s.jobs)
	s.mu.Unlock()
	_ = atomicWrite(filepath.Join(s.e.dataDir, "finishing.json"), b)
}
func (s *Finishing) close() {
	s.mu.Lock()
	s.closed = true
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()
	s.wg.Wait()
	s.save()
}
func validFinish(r FinishOptions) error {
	if r.Mode != "fast" && r.Mode != "neural" {
		return errors.New("choose fast or neural finishing")
	}
	if r.Backend != "cpu" && r.Backend != "auto" {
		return errors.New("choose auto or CPU")
	}
	if r.LongEdge < 64 || r.LongEdge > 7680 || r.Sharpness < 0 || r.Sharpness > 1 || r.Detail < 0 || r.Detail > 1 {
		return errors.New("invalid finishing dimensions or strength")
	}
	return nil
}
func (s *Finishing) source(id string) (*image.NRGBA, error) {
	path, err := s.e.objectPath(id)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.Size() > 64<<20 {
		return nil, errors.New("source must be a PNG/JPEG below 64 MiB")
	}
	b, err := io.ReadAll(io.LimitReader(f, 64<<20))
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != id {
		return nil, errors.New("source checksum mismatch")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil || (format != "png" && format != "jpeg") || cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width)*int64(cfg.Height) > 60_000_000 {
		return nil, errors.New("choose a valid PNG/JPEG below 60 megapixels")
	}
	if mem := physicalMemory(); mem > 0 && int64(cfg.Width)*int64(cfg.Height)*12 > int64(mem/4) {
		return nil, errors.New("source decoding needs more working memory; use a smaller source image")
	}
	im, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	return finishNRGBA(im), nil
}
func (s *Finishing) submit(r FinishRequest) (FinishJob, error) {
	if err := validFinish(r.FinishOptions); err != nil {
		return FinishJob{}, err
	}
	if _, err := s.e.objectPath(r.Asset); err != nil {
		return FinishJob{}, err
	}
	b, _ := json.Marshal(struct {
		Request FinishRequest
		Model   any
	}{r, s.catalog})
	sum := sha256.Sum256(b)
	key := hex.EncodeToString(sum[:])
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return FinishJob{}, errors.New("finishing is stopping")
	}
	pending := 0
	for _, j := range s.jobs {
		if j.Status == "running" || j.Status == "queued" {
			pending++
		}
	}
	if pending >= 32 {
		s.mu.Unlock()
		return FinishJob{}, errors.New("32 finishing jobs are queued")
	}
	j := &FinishJob{ID: "finish-" + randomID()[:16], Request: r, Status: "queued", Created: now(), Key: key, Message: "Queued for image finishing"}
	s.jobs = append(s.jobs, j)
	if len(s.jobs) > 200 {
		for i, v := range s.jobs {
			if v.Status != "queued" && v.Status != "running" {
				s.jobs = append(s.jobs[:i], s.jobs[i+1:]...)
				break
			}
		}
	}
	copy := *j
	s.mu.Unlock()
	s.save()
	s.kick()
	return copy, nil
}
func (s *Finishing) update(j *FinishJob, progress int, message string) {
	s.mu.Lock()
	j.Progress = progress
	j.Message = message
	s.mu.Unlock()
}
func (s *Finishing) kick() {
	s.mu.Lock()
	if s.closed || s.cancel != nil {
		s.mu.Unlock()
		return
	}
	var j *FinishJob
	for _, v := range s.jobs {
		if v.Status == "queued" {
			j = v
			break
		}
	}
	if j == nil {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Minute)
	s.cancel = cancel
	j.Status = "running"
	s.wg.Add(1)
	s.mu.Unlock()
	go func() {
		defer s.wg.Done()
		defer cancel()
		start := time.Now()
		err := s.process(ctx, j)
		s.mu.Lock()
		j.Seconds = time.Since(start).Seconds()
		j.Finished = now()
		if ctx.Err() != nil {
			j.Status = "cancelled"
			j.Message = "Stopped; original image retained"
		} else if err != nil {
			j.Status = "failed"
			j.Message = err.Error()
		} else {
			j.Status = "completed"
			j.Progress = 100
			j.Message = "Finished; original image retained"
		}
		s.cancel = nil
		s.mu.Unlock()
		s.save()
		s.kick()
	}()
}
func (s *Finishing) process(ctx context.Context, j *FinishJob) error {
	s.update(j, 2, "Checking original image")
	src, err := s.source(j.Request.Asset)
	if err != nil {
		return err
	}
	w, h, err := finishDimensions(src.Rect.Dx(), src.Rect.Dy(), j.Request.LongEdge)
	if err != nil {
		return err
	}
	if mem := physicalMemory(); mem > 0 && int64(w)*int64(h)*12 > int64(mem/4) {
		return errors.New("output needs more working memory; choose a smaller long edge")
	}
	s.mu.Lock()
	var cached *FinishJob
	for _, other := range s.jobs {
		if other.ID != j.ID && other.Key == j.Key && other.Status == "completed" && other.Asset != nil {
			copy := *other
			cached = &copy
			break
		}
	}
	s.mu.Unlock()
	if cached != nil {
		p, _ := s.e.objectPath(cached.Asset.ID)
		if f, err := os.Open(p); err == nil {
			hash := sha256.New()
			_, err = io.Copy(hash, f)
			f.Close()
			if err == nil && hex.EncodeToString(hash.Sum(nil)) == cached.Asset.ID {
				s.mu.Lock()
				j.Asset = cached.Asset
				j.Recipe = cached.Recipe
				j.Width = cached.Width
				j.Height = cached.Height
				j.Backend = cached.Backend
				j.Cached = true
				s.mu.Unlock()
				return nil
			}
		}
	}
	if err = s.e.acquireHeavy(ctx); err != nil {
		return err
	}
	defer s.e.releaseHeavy()
	threads := maxInt(1, minInt(8, runtime.NumCPU()-1))
	var result *image.NRGBA
	backend := "lanczos3-cpu"
	if j.Request.Mode == "neural" {
		s.update(j, 8, "Loading verified Real-ESRGAN weights")
		result, backend, err = s.neural(ctx, j, src, threads)
		if err != nil {
			return err
		}
	}
	s.update(j, 75, "Resizing to final dimensions")
	if result == nil {
		result, err = resizeFinish(ctx, src, w, h, threads)
	} else {
		result, err = resizeFinish(ctx, result, w, h, threads)
		if err == nil && j.Request.Detail < 1 {
			var base *image.NRGBA
			base, err = resizeFinish(ctx, src, w, h, threads)
			if err == nil {
				a := j.Request.Detail
				for i := range result.Pix {
					if i%4 == 3 {
						result.Pix[i] = base.Pix[i]
					} else {
						result.Pix[i] = byte(float64(result.Pix[i])*a + float64(base.Pix[i])*(1-a) + .5)
					}
				}
			}
		}
	}
	if err != nil {
		return err
	}
	s.update(j, 85, "Applying bounded sharpening")
	if err = sharpenFinish(ctx, result, j.Request.Sharpness); err != nil {
		return err
	}
	s.update(j, 92, "Encoding PNG and saving recipe")
	f, err := os.CreateTemp(s.e.dataDir, "finish-*.png")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	err = encoder.Encode(f, result)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	f, err = os.Open(f.Name())
	if err != nil {
		return err
	}
	a, err := s.e.storeObject(f, j.ID+".png", "image/png", "ORIGIN-0 "+j.Request.Mode+" finishing; source "+j.Request.Asset)
	f.Close()
	if err != nil {
		return err
	}
	recipe := map[string]any{"schema": "origin0.finish.v1", "source": j.Request.Asset, "result": a.ID, "request": j.Request, "width": w, "height": h, "backend": backend, "model": s.catalog.Model, "model_runtime": s.catalog.Runtimes[runtime.GOOS+"-"+runtime.GOARCH], "interpretation": "Learned detail is an estimate. Compare with the preserved original; this is not recovered ground truth."}
	b, _ := json.MarshalIndent(recipe, "", "  ")
	meta, err := s.e.storeObject(bytes.NewReader(b), j.ID+".json", "application/json", "image finishing recipe")
	if err != nil {
		return err
	}
	s.mu.Lock()
	j.Asset = &a
	j.Recipe = &meta
	j.Width = w
	j.Height = h
	j.Backend = backend
	s.mu.Unlock()
	return nil
}
func (e *Engine) acquireHeavy(ctx context.Context) error {
	select {
	case e.heavy <- struct{}{}:
		e.imageBusy.Store(true)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (e *Engine) releaseHeavy() { e.imageBusy.Store(false); <-e.heavy }
func (s *Finishing) neural(ctx context.Context, j *FinishJob, src *image.NRGBA, threads int) (*image.NRGBA, string, error) {
	for i := 3; i < len(src.Pix); i += 4 {
		if src.Pix[i] != 255 {
			return nil, "", errors.New("learned mode currently requires an opaque image; use fast mode to preserve transparency")
		}
	}
	spec, ok := s.catalog.Runtimes[runtime.GOOS+"-"+runtime.GOARCH]
	if !ok {
		return nil, "", errors.New("neural runtime is not bundled for this build; use fast finishing or install a supported release")
	}
	root := filepath.Join(s.e.dataDir, "upscaler")
	archive := filepath.Join(root, spec.Name)
	if _, err := os.Stat(archive); os.IsNotExist(err) {
		if b, err := assets.ReadFile("bundled/" + spec.Name); err == nil {
			if err = atomicWrite(archive, b); err != nil {
				return nil, "", err
			}
		}
	}
	if err := downloadPinned(ctx, s.e.images.client, spec, archive, func(msg string, _, _ int64) { s.update(j, 8, msg) }); err != nil {
		return nil, "", err
	}
	dir := filepath.Join(root, spec.SHA256[:16])
	if err := extractRuntime(archive, dir); err != nil {
		return nil, "", err
	}
	// Cap the learned pass at 1024 on its long edge. Final Lanczos sizing is separate.
	base := src
	if maxInt(src.Rect.Dx(), src.Rect.Dy()) > 1024 {
		w, h, _ := finishDimensions(src.Rect.Dx(), src.Rect.Dy(), 1024)
		var err error
		base, err = resizeFinish(ctx, src, w, h, threads)
		if err != nil {
			return nil, "", err
		}
	}
	work, err := os.MkdirTemp(root, "run-")
	if err != nil {
		return nil, "", err
	}
	defer os.RemoveAll(work)
	rgb := make([]byte, base.Rect.Dx()*base.Rect.Dy()*3)
	for i := 0; i < len(rgb)/3; i++ {
		copy(rgb[i*3:i*3+3], base.Pix[i*4:i*4+3])
	}
	input, output := filepath.Join(work, "input.rgb"), filepath.Join(work, "output.rgb")
	if err = atomicWrite(input, rgb); err != nil {
		return nil, "", err
	}
	name := "origin0-upscale"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	cmd := nativeCommand(ctx, filepath.Join(dir, name), input, output, strconv.Itoa(base.Rect.Dx()), strconv.Itoa(base.Rect.Dy()), filepath.Join(dir, "realesrgan-x4plus.param"), filepath.Join(dir, "realesrgan-x4plus.bin"), "64", strconv.Itoa(threads), j.Request.Backend)
	backend := "cpu"
	writer := &progressLog{fn: func(line string) {
		s.mu.Lock()
		defer s.mu.Unlock()
		j.Log = tail(j.Log+line, 8000)
		for _, part := range strings.Split(line, "\n") {
			if strings.HasPrefix(part, "backend=") {
				backend = strings.TrimSpace(strings.TrimPrefix(part, "backend="))
			}
			var done, total int
			if _, err := fmt.Sscanf(part, "tile=%d/%d", &done, &total); err == nil && total > 0 {
				j.Progress = 10 + done*60/total
				j.Message = fmt.Sprintf("Learned reconstruction: tile %d / %d", done, total)
			}
		}
	}}
	cmd.Stdout = writer
	cmd.Stderr = writer
	if err = cmd.Run(); err != nil {
		return nil, "", fmt.Errorf("learned upscaling failed: %w; select CPU if the graphics driver failed", err)
	}
	ow, oh := base.Rect.Dx()*4, base.Rect.Dy()*4
	f, err := os.Open(output)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.Size() != int64(ow*oh*3) {
		return nil, "", errors.New("native output dimensions differ")
	}
	raw, err := io.ReadAll(f)
	if err != nil {
		return nil, "", err
	}
	im := image.NewNRGBA(image.Rect(0, 0, ow, oh))
	for i := 0; i < ow*oh; i++ {
		copy(im.Pix[i*4:i*4+3], raw[i*3:i*3+3])
		im.Pix[i*4+3] = 255
	}
	return im, "realesrgan-x4plus-" + backend, nil
}
func (e *Engine) finishingRoutes(mux *http.ServeMux) {
	s := e.finishing
	mux.HandleFunc("/api/finish/state", func(w http.ResponseWriter, r *http.Request) {
		if err := decode(r, &struct{}{}); err != nil {
			apiError(w, err)
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		_, ready := s.catalog.Runtimes[runtime.GOOS+"-"+runtime.GOARCH]
		jsonReply(w, map[string]any{"jobs": s.jobs, "neural_available": ready, "busy": s.cancel != nil, "max_long_edge": 7680})
	})
	mux.HandleFunc("/api/finish/start", func(w http.ResponseWriter, r *http.Request) {
		var req FinishRequest
		if err := decode(r, &req); err != nil {
			apiError(w, err)
			return
		}
		j, err := s.submit(req)
		if err != nil {
			apiError(w, err)
			return
		}
		jsonReply(w, j)
	})
	mux.HandleFunc("/api/finish/cancel", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID string `json:"id"`
		}
		if err := decode(r, &req); err != nil {
			apiError(w, err)
			return
		}
		s.mu.Lock()
		for _, j := range s.jobs {
			if j.ID == req.ID {
				if j.Status == "running" && s.cancel != nil {
					s.cancel()
				} else if j.Status == "queued" {
					j.Status = "cancelled"
					j.Finished = now()
				}
			}
		}
		s.mu.Unlock()
		s.save()
		jsonReply(w, map[string]bool{"ok": true})
	})
}
