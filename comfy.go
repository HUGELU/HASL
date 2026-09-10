package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func validComfyURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return errors.New("use a local ComfyUI address such as http://127.0.0.1:8188")
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return nil
	}
	return errors.New("ComfyUI must be local; use a localhost tunnel for an authorised remote workstation")
}
func (s *MediaStudio) comfyURL() string { s.mu.Lock(); defer s.mu.Unlock(); return s.config.ComfyURL }
func (s *MediaStudio) comfyJSON(ctx context.Context, base, path string, in, out any) error {
	if err := validComfyURL(base); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var body io.Reader
	method := "GET"
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
		method = "POST"
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("ComfyUI is not responding at %s: %w", base, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("ComfyUI %s: %s", resp.Status, shortText(string(b), 2500))
	}
	if out != nil {
		return json.Unmarshal(b, out)
	}
	return nil
}
func (s *MediaStudio) comfyProbe(ctx context.Context) (any, error) {
	base := s.comfyURL()
	var system, info, queue map[string]any
	if err := s.comfyJSON(ctx, base, "/system_stats", nil, &system); err != nil {
		return nil, err
	}
	if err := s.comfyJSON(ctx, base, "/object_info", nil, &info); err != nil {
		return nil, err
	}
	_ = s.comfyJSON(ctx, base, "/queue", nil, &queue)
	// Return actual checkpoint / LoRA choices, rather than claiming a download is loaded.
	choices := map[string]any{}
	for name, node := range info {
		if name == "CheckpointLoaderSimple" || name == "LoraLoader" || name == "UNETLoader" || name == "CLIPLoader" {
			choices[name] = node
		}
	}
	classes := []string{}
	for k := range info {
		classes = append(classes, k)
	}
	return map[string]any{"system": system, "loaders": choices, "nodes": classes, "queue": queue, "editor_url": base}, nil
}
func node(class string, in map[string]any) map[string]any {
	return map[string]any{"class_type": class, "inputs": in}
}
func link(id string, slot int) []any { return []any{id, slot} }
func studioGraph(q StudioRequest, m StudioModel, uploaded string) (map[string]any, error) {
	if strings.TrimSpace(q.Prompt) == "" || len(q.Prompt) > 16000 {
		return nil, errors.New("enter a prompt up to 16000 characters")
	}
	if q.Width < 256 || q.Height < 256 || q.Width > 2048 || q.Height > 2048 || q.Width%16 != 0 || q.Height%16 != 0 {
		return nil, errors.New("generation size must be 256–2048 pixels, divisible by 16; use finishing for larger exports")
	}
	if q.Steps < 1 || q.Steps > 100 || q.Guidance == nil || *q.Guidance < 0 || *q.Guidance > 30 || q.Seed < -1 || q.Seed > 9007199254740991 {
		return nil, errors.New("invalid steps, guidance or seed")
	}
	files := map[string]string{}
	for _, f := range m.Files {
		files[f.Role] = f.Name
	}
	g := map[string]any{}
	model, clip, vae := link("1", 0), link("1", 1), link("1", 2)
	if m.Recipe == "checkpoint" || m.Recipe == "flux" {
		g["1"] = node("CheckpointLoaderSimple", map[string]any{"ckpt_name": files["checkpoints"]})
	} else {
		clipType := "flux2"
		if m.Recipe == "qwen" {
			clipType = "qwen_image"
		}
		g["1"] = node("UNETLoader", map[string]any{"unet_name": files["diffusion_models"], "weight_dtype": "default"})
		g["10"] = node("CLIPLoader", map[string]any{"clip_name": files["text_encoders"], "type": clipType, "device": "default"})
		clip = link("10", 0)
		g["11"] = node("VAELoader", map[string]any{"vae_name": files["vae"]})
		vae = link("11", 0)
		if m.Recipe == "qwen" {
			g["12"] = node("ModelSamplingAuraFlow", map[string]any{"model": model, "shift": 3.1})
			model = link("12", 0)
		}
	}
	if q.Lora != "" {
		if strings.Contains(q.Lora, "..") || filepath.IsAbs(q.Lora) || q.LoraWeight < 0 || q.LoraWeight > 2 {
			return nil, errors.New("choose an installed LoRA and a weight from 0 to 2")
		}
		g["13"] = node("LoraLoader", map[string]any{"model": model, "clip": clip, "lora_name": q.Lora, "strength_model": q.LoraWeight, "strength_clip": q.LoraWeight})
		model = link("13", 0)
		clip = link("13", 1)
	}
	g["2"] = node("CLIPTextEncode", map[string]any{"text": q.Prompt, "clip": clip})
	g["3"] = node("CLIPTextEncode", map[string]any{"text": q.Negative, "clip": clip})
	latentClass := "EmptyLatentImage"
	if m.Recipe == "flux" || m.Recipe == "qwen" {
		latentClass = "EmptySD3LatentImage"
	}
	if m.Recipe == "flux2" {
		latentClass = "EmptyFlux2LatentImage"
	}
	g["4"] = node(latentClass, map[string]any{"width": q.Width, "height": q.Height, "batch_size": 1})
	denoise := 1.0
	if uploaded != "" {
		if m.Recipe == "flux2" {
			return nil, errors.New("FLUX.2 reference editing requires its reference workflow; open the full ComfyUI editor or use an SDXL image-to-image model")
		}
		if q.Strength <= 0 || q.Strength > 1 {
			return nil, errors.New("reference denoising strength must be above 0 and at most 1")
		}
		denoise = q.Strength
		g["14"] = node("LoadImage", map[string]any{"image": uploaded})
		g["15"] = node("ImageScale", map[string]any{"image": link("14", 0), "upscale_method": "lanczos", "width": q.Width, "height": q.Height, "crop": "disabled"})
		g["4"] = node("VAEEncode", map[string]any{"pixels": link("15", 0), "vae": vae})
	}
	if m.Recipe == "flux2" {
		g["3"] = node("ConditioningZeroOut", map[string]any{"conditioning": link("2", 0)})
		g["20"] = node("RandomNoise", map[string]any{"noise_seed": q.Seed})
		g["21"] = node("KSamplerSelect", map[string]any{"sampler_name": "euler"})
		g["22"] = node("Flux2Scheduler", map[string]any{"steps": q.Steps, "width": q.Width, "height": q.Height})
		g["23"] = node("CFGGuider", map[string]any{"model": model, "positive": link("2", 0), "negative": link("3", 0), "cfg": *q.Guidance})
		g["5"] = node("SamplerCustomAdvanced", map[string]any{"noise": link("20", 0), "guider": link("23", 0), "sampler": link("21", 0), "sigmas": link("22", 0), "latent_image": link("4", 0)})
	} else {
		g["5"] = node("KSampler", map[string]any{"model": model, "positive": link("2", 0), "negative": link("3", 0), "latent_image": link("4", 0), "seed": q.Seed, "steps": q.Steps, "cfg": *q.Guidance, "sampler_name": "euler", "scheduler": "simple", "denoise": denoise})
	}
	g["6"] = node("VAEDecodeTiled", map[string]any{"samples": link("5", 0), "vae": vae, "tile_size": 512, "overlap": 64, "temporal_size": 64, "temporal_overlap": 8})
	g["7"] = node("SaveImage", map[string]any{"images": link("6", 0), "filename_prefix": "ORIGIN0"})
	return g, nil
}
func (s *MediaStudio) uploadComfy(ctx context.Context, base, id string) (string, error) {
	p, err := s.e.objectPath(id)
	if err != nil {
		return "", err
	}
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.Size() > 40<<20 {
		return "", errors.New("reference image exceeds 40 MB")
	}
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	part, _ := mw.CreateFormFile("image", "origin-"+id+".png")
	if _, err = io.Copy(part, f); err != nil {
		return "", err
	}
	_ = mw.WriteField("type", "input")
	_ = mw.Close()
	req, err := http.NewRequestWithContext(ctx, "POST", base+"/upload/image", &b)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", errors.New("ComfyUI rejected reference upload")
	}
	var v struct{ Name, Subfolder string }
	if err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&v); err != nil {
		return "", err
	}
	return strings.TrimPrefix(v.Subfolder+"/"+v.Name, "/"), nil
}
func (s *MediaStudio) submitComfy(q StudioRequest, m StudioModel) (any, error) {
	if q.Seed == -1 {
		q.Seed = time.Now().UnixNano() & 0x1fffffffffffff
	}
	uploaded := ""
	if q.InitAsset != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
		defer cancel()
		var err error
		uploaded, err = s.uploadComfy(ctx, s.comfyURL(), q.InitAsset)
		if err != nil {
			return nil, err
		}
	}
	g, err := studioGraph(q, m, uploaded)
	if err != nil {
		return nil, err
	}
	return s.submitGraph(q, g)
}
func (s *MediaStudio) submitGraph(q StudioRequest, g map[string]any) (ComfyJob, error) {
	if len(g) == 0 || len(g) > 500 {
		return ComfyJob{}, errors.New("import a ComfyUI API-format workflow with 1–500 nodes")
	}
	for _, v := range g {
		n, ok := v.(map[string]any)
		if !ok || n["class_type"] == nil || n["inputs"] == nil {
			return ComfyJob{}, errors.New("use Export (API), not the visual editor workflow JSON")
		}
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ComfyJob{}, errors.New("studio is closing")
	}
	pending := 0
	for _, j := range s.jobs {
		if j.Status == "queued" || j.Status == "running" {
			pending++
		}
	}
	if pending >= 64 {
		s.mu.Unlock()
		return ComfyJob{}, errors.New("64 ComfyUI jobs are waiting; allow the queue to drain")
	}
	j := &ComfyJob{ID: "comfy-" + randomID()[:16], Status: "queued", Message: "Waiting for the local generation budget", Created: now(), Request: q, Graph: g, URL: s.config.ComfyURL}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	s.cancels[j.ID] = cancel
	s.jobs = append(s.jobs, j)
	if len(s.jobs) > 500 {
		for i, x := range s.jobs {
			if x.Status != "queued" && x.Status != "running" {
				s.jobs = append(s.jobs[:i], s.jobs[i+1:]...)
				break
			}
		}
	}
	s.wg.Add(1)
	copy := *j
	s.mu.Unlock()
	s.save()
	go s.runComfy(ctx, j, cancel)
	return copy, nil
}
func (s *MediaStudio) jobMessage(j *ComfyJob, status, message string) {
	s.mu.Lock()
	j.Status = status
	j.Message = message
	s.mu.Unlock()
}
func (s *MediaStudio) runComfy(ctx context.Context, j *ComfyJob, cancel context.CancelFunc) {
	defer s.wg.Done()
	defer cancel()
	defer func() { s.mu.Lock(); delete(s.cancels, j.ID); s.mu.Unlock(); s.save() }()
	fail := func(err error) {
		status := "failed"
		if ctx.Err() != nil {
			status = "interrupted"
		}
		s.jobMessage(j, status, err.Error())
	}
	select {
	case s.e.heavy <- struct{}{}:
		defer func() { <-s.e.heavy }()
	case <-ctx.Done():
		fail(ctx.Err())
		return
	}
	var submit struct {
		PromptID   string `json:"prompt_id"`
		Error      any    `json:"error"`
		NodeErrors any    `json:"node_errors"`
	}
	if err := s.comfyJSON(ctx, j.URL, "/prompt", map[string]any{"prompt": j.Graph, "client_id": j.ID}, &submit); err != nil {
		fail(err)
		return
	}
	if submit.PromptID == "" {
		fail(fmt.Errorf("workflow rejected: %v %v", submit.Error, submit.NodeErrors))
		return
	}
	s.mu.Lock()
	j.PromptID = submit.PromptID
	j.Status = "running"
	j.Message = "ComfyUI accepted the workflow. Loading models / executing nodes; elapsed time is real, percentage is not estimated."
	s.mu.Unlock()
	s.save()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	failures := 0
	for {
		select {
		case <-ctx.Done():
			fail(fmt.Errorf("monitor stopped: %w; check ComfyUI history before retrying", ctx.Err()))
			return
		case <-ticker.C:
		}
		var history map[string]struct {
			Outputs map[string]json.RawMessage `json:"outputs"`
			Status  struct {
				Status    string `json:"status_str"`
				Completed bool   `json:"completed"`
				Messages  any    `json:"messages"`
			} `json:"status"`
		}
		if err := s.comfyJSON(ctx, j.URL, "/history/"+url.PathEscape(submit.PromptID), nil, &history); err != nil {
			failures++
			if failures >= 3 {
				fail(err)
				return
			}
			continue
		}
		failures = 0
		h, ok := history[submit.PromptID]
		if !ok {
			continue
		}
		if h.Status.Status == "error" {
			fail(fmt.Errorf("ComfyUI execution failed: %v", h.Status.Messages))
			return
		}
		if !h.Status.Completed {
			continue
		}
		var saved []AssetRecord
		for _, raw := range h.Outputs {
			var output map[string][]struct {
				Filename  string `json:"filename"`
				Subfolder string `json:"subfolder"`
				Type      string `json:"type"`
			}
			_ = json.Unmarshal(raw, &output)
			for _, kind := range []string{"images", "gifs", "videos", "audio"} {
				for _, f := range output[kind] {
					if f.Filename == "" {
						continue
					}
					query := url.Values{"filename": {f.Filename}, "subfolder": {f.Subfolder}, "type": {f.Type}}
					req, _ := http.NewRequestWithContext(ctx, "GET", j.URL+"/view?"+query.Encode(), nil)
					resp, err := s.client.Do(req)
					if err != nil {
						fail(err)
						return
					}
					if resp.StatusCode/100 != 2 {
						resp.Body.Close()
						fail(errors.New("ComfyUI output download failed"))
						return
					}
					a, err := s.e.storeObject(io.LimitReader(resp.Body, 1<<30), filepath.Base(f.Filename), resp.Header.Get("Content-Type"), "Open Studio / ComfyUI "+j.Request.Model)
					resp.Body.Close()
					if err != nil {
						fail(err)
						return
					}
					saved = append(saved, a)
				}
			}
		}
		if len(saved) == 0 {
			fail(errors.New("workflow completed without a saved image, video or audio output; add a Save node"))
			return
		}
		s.mu.Lock()
		j.Assets = saved
		j.Status = "completed"
		j.Message = "Outputs saved in ORIGIN with the submitted workflow and generation parameters."
		s.mu.Unlock()
		s.e.persist()
		return
	}
}
func (s *MediaStudio) cancelComfy(id string) error {
	s.mu.Lock()
	var target *ComfyJob
	var cancel context.CancelFunc
	for _, j := range s.jobs {
		if j.ID == id {
			v := *j
			target = &v
			cancel = s.cancels[id]
			break
		}
	}
	s.mu.Unlock()
	if target == nil || cancel == nil {
		return errors.New("job is not running")
	}
	if target.PromptID != "" {
		ctx, c := context.WithTimeout(context.Background(), 10*time.Second)
		defer c()
		// Both operations are scoped to our prompt ID in the pinned ComfyUI release.
		_ = s.comfyJSON(ctx, target.URL, "/queue", map[string]any{"delete": []string{target.PromptID}}, nil)
		_ = s.comfyJSON(ctx, target.URL, "/interrupt", map[string]any{"prompt_id": target.PromptID}, nil)
	}
	cancel()
	return nil
}
