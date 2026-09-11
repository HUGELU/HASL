package main

// Provider metadata is resolved by the server. The browser never supplies a download URL or checksum.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type SourceQuery struct {
	Provider string `json:"provider"`
	Query    string `json:"query"`
	ID       string `json:"id"`
	Revision string `json:"revision"`
	File     string `json:"file"`
	Role     string `json:"role"`
	Profile  string `json:"profile"`
	Page     int    `json:"page"`
}
type SourceModel struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Provider  string `json:"provider"`
	Source    string `json:"source"`
	Type      string `json:"type"`
	License   string `json:"license"`
	Downloads int64  `json:"downloads"`
	Gated     any    `json:"gated,omitempty"`
}
type SourceFile struct {
	Name        string       `json:"name"`
	Version     string       `json:"version"`
	VersionName string       `json:"version_name,omitempty"`
	BaseModel   string       `json:"base_model,omitempty"`
	Role        string       `json:"role"`
	Profile     string       `json:"profile"`
	Spec        DownloadSpec `json:"spec"`
}
type SourceDetail struct {
	Model    SourceModel  `json:"model"`
	Revision string       `json:"revision"`
	Files    []SourceFile `json:"files"`
	Note     string       `json:"note"`
}
type ProviderKeys struct {
	HF      string `json:"huggingface"`
	Civitai string `json:"civitai"`
}

func (s *MediaStudio) providerKeys() ProviderKeys {
	var keys ProviderKeys
	b, _ := os.ReadFile(filepath.Join(s.root(), "provider-keys.json"))
	_ = json.Unmarshal(b, &keys)
	return keys
}
func (s *MediaStudio) saveProviderKeys(keys ProviderKeys) error {
	b, e := json.Marshal(keys)
	if e != nil {
		return e
	}
	return atomicWrite(filepath.Join(s.root(), "provider-keys.json"), b)
}

type providerTransport struct {
	next http.RoundTripper
	keys ProviderKeys
}

func (t providerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	copy := r.Clone(r.Context())
	copy.Header = r.Header.Clone()
	copy.Header.Del("Authorization")
	if r.URL.Scheme == "https" {
		switch r.URL.Hostname() {
		case "huggingface.co":
			if t.keys.HF != "" {
				copy.Header.Set("Authorization", "Bearer "+t.keys.HF)
			}
		case "civitai.com":
			if t.keys.Civitai != "" {
				copy.Header.Set("Authorization", "Bearer "+t.keys.Civitai)
			}
		}
	}
	return t.next.RoundTrip(copy)
}
func (s *MediaStudio) downloadClient() *http.Client {
	t := s.client.Transport
	if t == nil {
		t = http.DefaultTransport
	}
	return &http.Client{Transport: providerTransport{t, s.providerKeys()}, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("too many provider redirects")
		}
		if req.URL.Scheme != "https" {
			return errors.New("provider download redirected away from HTTPS")
		}
		return nil
	}}
}
func (s *MediaStudio) providerJSON(ctx context.Context, raw string, out any) error {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || (u.Host != "huggingface.co" && u.Host != "civitai.com") {
		return errors.New("unsupported model provider")
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, "GET", raw, nil)
	if e != nil {
		return e
	}
	req.Header.Set("User-Agent", "ORIGIN0/1.9.0 (+https://github.com/HUGELU/HASL)")
	res, e := s.downloadClient().Do(req)
	if e != nil {
		return fmt.Errorf("%s could not be reached; check your connection and retry", u.Host)
	}
	defer res.Body.Close()
	if res.StatusCode == 401 || res.StatusCode == 403 {
		return fmt.Errorf("%s requires access for this item. Add your provider token and accept any model terms on its source page", u.Host)
	}
	if res.StatusCode == 429 {
		return fmt.Errorf("%s rate limit reached; wait before searching again", u.Host)
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("%s returned HTTP %d", u.Host, res.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(res.Body, (8<<20)+1))
	if e != nil {
		return e
	}
	if len(b) > 8<<20 {
		return errors.New("provider metadata exceeds 8 MB; choose a narrower query")
	}
	return json.Unmarshal(b, out)
}
func (s *MediaStudio) searchSource(ctx context.Context, q SourceQuery) ([]SourceModel, error) {
	if len(q.Query) > 200 {
		return nil, errors.New("search query exceeds 200 characters")
	}
	items := []SourceModel{}
	switch q.Provider {
	case "huggingface":
		var rows []struct {
			ID        string
			Downloads int64
			Pipeline  string `json:"pipeline_tag"`
			Tags      []string
			Gated     any
		}
		u := "https://huggingface.co/api/models?limit=30&sort=downloads&direction=-1&search=" + url.QueryEscape(strings.TrimSpace(q.Query))
		if e := s.providerJSON(ctx, u, &rows); e != nil {
			return nil, e
		}
		for _, r := range rows {
			license := "See model card"
			for _, t := range r.Tags {
				if strings.HasPrefix(t, "license:") {
					license = strings.TrimPrefix(t, "license:")
				}
			}
			items = append(items, SourceModel{ID: r.ID, Name: r.ID, Provider: q.Provider, Source: "https://huggingface.co/" + r.ID, Type: r.Pipeline, License: license, Downloads: r.Downloads, Gated: r.Gated})
		}
	case "civitai":
		var rows struct {
			Items []struct {
				ID                    int64
				Name                  string
				Type                  string
				Stats                 struct{ DownloadCount int64 }
				AllowCommercialUse    any
				AllowDerivatives      bool
				AllowDifferentLicense bool
			}
		}
		u := "https://civitai.com/api/v1/models?limit=30&nsfw=false&page=" + strconv.Itoa(maxInt(1, minInt(100, q.Page))) + "&query=" + url.QueryEscape(strings.TrimSpace(q.Query))
		if e := s.providerJSON(ctx, u, &rows); e != nil {
			return nil, e
		}
		for _, r := range rows.Items {
			id := strconv.FormatInt(r.ID, 10)
			items = append(items, SourceModel{ID: id, Name: r.Name, Provider: q.Provider, Source: "https://civitai.com/models/" + id, Type: r.Type, License: "Creator terms on model page", Downloads: r.Stats.DownloadCount})
		}
	default:
		return nil, errors.New("choose Hugging Face or Civitai")
	}
	return items, nil
}

var repoIDPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
var fullHash = regexp.MustCompile(`^[0-9a-f]{64}$`)
var revisionPattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,128}$`)

func modelFileType(name, kind, base string) (string, string) {
	n := strings.ToLower(name)
	k := strings.ToLower(kind)
	b := strings.ToLower(base)
	if strings.Contains(k, "lora") || strings.Contains(n, "lora") {
		return "loras", "workflow"
	}
	if strings.Contains(n, "vae") || k == "vae" {
		return "vae", "workflow"
	}
	if strings.Contains(n, "text_encoder") || strings.Contains(n, "t5xxl") || strings.Contains(n, "clip_l") || strings.Contains(n, "qwen_3") {
		return "text_encoders", "workflow"
	}
	if strings.Contains(n, "diffusion") || strings.HasSuffix(n, ".gguf") || strings.Contains(n, "unet") {
		return "diffusion_models", "workflow"
	}
	if strings.Contains(b, "sdxl") || strings.Contains(b, "pony") || strings.Contains(b, "illustrious") {
		return "checkpoints", "sdxl"
	}
	if strings.HasPrefix(b, "sd 1") || strings.Contains(b, "stable-diffusion-v1") {
		return "checkpoints", "sd15"
	}
	return "checkpoints", "workflow"
}
func (s *MediaStudio) inspectSource(ctx context.Context, q SourceQuery) (SourceDetail, error) {
	d := SourceDetail{Files: []SourceFile{}, Note: "Files are model components, not necessarily complete pipelines. Choose a complete checkpoint for direct generation; other components use the matching ComfyUI workflow."}
	switch q.Provider {
	case "huggingface":
		if !repoIDPattern.MatchString(q.ID) {
			return d, errors.New("enter a Hugging Face repository as owner/model")
		}
		revision := q.Revision
		if revision == "" {
			revision = "main"
		}
		if !revisionPattern.MatchString(revision) {
			return d, errors.New("invalid repository revision")
		}
		var r struct {
			ID       string
			SHA      string
			Gated    any
			Tags     []string
			Pipeline string         `json:"pipeline_tag"`
			Card     map[string]any `json:"cardData"`
			Siblings []struct {
				Name string `json:"rfilename"`
				LFS  *struct {
					SHA256 string
					Size   int64
				}
			}
		}
		if e := s.providerJSON(ctx, "https://huggingface.co/api/models/"+q.ID+"/revision/"+url.PathEscape(revision)+"?blobs=true", &r); e != nil {
			return d, e
		}
		if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(r.SHA) {
			return d, errors.New("provider did not return an immutable model revision")
		}
		license := "See model card"
		if x, ok := r.Card["license"].(string); ok {
			license = x
		}
		d.Revision = r.SHA
		d.Model = SourceModel{ID: q.ID, Name: q.ID, Provider: q.Provider, Source: "https://huggingface.co/" + q.ID, Type: r.Pipeline, License: license, Gated: r.Gated}
		for _, f := range r.Siblings {
			ext := strings.ToLower(path.Ext(f.Name))
			if f.LFS == nil || (ext != ".safetensors" && ext != ".gguf") || !fullHash.MatchString(f.LFS.SHA256) || f.LFS.Size <= 0 {
				continue
			}
			role, profile := modelFileType(f.Name, "", fmt.Sprint(r.Card["base_model"]))
			encoded := []string{}
			for _, p := range strings.Split(f.Name, "/") {
				encoded = append(encoded, url.PathEscape(p))
			}
			spec := DownloadSpec{Name: path.Base(f.Name), URL: "https://huggingface.co/" + q.ID + "/resolve/" + r.SHA + "/" + strings.Join(encoded, "/"), Size: f.LFS.Size, SHA256: f.LFS.SHA256, Role: role, License: license, Source: d.Model.Source}
			d.Files = append(d.Files, SourceFile{Name: f.Name, Version: r.SHA, Role: role, Profile: profile, Spec: spec})
		}
	case "civitai":
		id, e := strconv.ParseInt(q.ID, 10, 64)
		if e != nil || id < 1 {
			return d, errors.New("enter the numeric Civitai model ID")
		}
		var r struct {
			ID            int64
			Name          string
			Type          string
			ModelVersions []struct {
				ID        int64
				Name      string
				BaseModel string
				Files     []struct {
					Name        string
					SizeKB      float64
					DownloadURL string
					Hashes      struct{ SHA256 string }
				}
			}
		}
		if e := s.providerJSON(ctx, "https://civitai.com/api/v1/models/"+strconv.FormatInt(id, 10), &r); e != nil {
			return d, e
		}
		d.Model = SourceModel{ID: q.ID, Name: r.Name, Provider: q.Provider, Source: "https://civitai.com/models/" + q.ID, Type: r.Type, License: "Creator terms on model page"}
		for _, v := range r.ModelVersions {
			for _, f := range v.Files {
				hash := strings.ToLower(f.Hashes.SHA256)
				u, e := url.Parse(f.DownloadURL)
				if e != nil || u.Scheme != "https" || u.Host != "civitai.com" || u.User != nil || !strings.HasPrefix(u.Path, "/api/download/models/") || !fullHash.MatchString(hash) || !strings.HasSuffix(strings.ToLower(f.Name), ".safetensors") {
					continue
				}
				role, profile := modelFileType(f.Name, r.Type, v.BaseModel)
				spec := DownloadSpec{Name: path.Base(f.Name), URL: f.DownloadURL, Size: int64(math.Round(f.SizeKB * 1024)), SHA256: hash, Role: role, License: d.Model.License, Source: d.Model.Source}
				d.Files = append(d.Files, SourceFile{Name: f.Name, Version: strconv.FormatInt(v.ID, 10), VersionName: v.Name, BaseModel: v.BaseModel, Role: role, Profile: profile, Spec: spec})
			}
		}
	default:
		return d, errors.New("choose Hugging Face or Civitai")
	}
	return d, nil
}
func recipeDefaults(profile string) (string, int, int, float64, error) {
	switch profile {
	case "sd15":
		return "checkpoint", 512, 20, 7, nil
	case "sdxl":
		return "checkpoint", 1024, 25, 6, nil
	case "workflow":
		return "workflow", 512, 20, 1, nil
	}
	return "", 0, 0, 0, errors.New("choose SD 1.5, SDXL or a workflow component")
}
func (s *MediaStudio) registerSource(ctx context.Context, q SourceQuery) (StudioModel, error) {
	d, e := s.inspectSource(ctx, q)
	if e != nil {
		return StudioModel{}, e
	}
	var chosen *SourceFile
	for i := range d.Files {
		if d.Files[i].Name == q.File && d.Files[i].Version == q.Revision {
			chosen = &d.Files[i]
			break
		}
	}
	if chosen == nil {
		return StudioModel{}, errors.New("file or revision changed; inspect the model again")
	}
	role := q.Role
	if role == "" {
		role = chosen.Role
	}
	if !validModelRole(role) {
		return StudioModel{}, errors.New("invalid shared model folder")
	}
	profile := q.Profile
	if profile == "" {
		profile = chosen.Profile
	}
	if role != "checkpoints" {
		profile = "workflow"
	}
	recipe, width, steps, guidance, e := recipeDefaults(profile)
	if e != nil {
		return StudioModel{}, e
	}
	f := chosen.Spec
	f.Role = role
	if path.Base(f.Name) != f.Name || strings.ContainsAny(f.Name, "\\/:") || f.Size <= 0 {
		return StudioModel{}, errors.New("invalid provider model filename or length")
	}
	// Hash-prefix prevents unrelated repositories with model.safetensors overwriting each other.
	f.Name = f.SHA256[:12] + "-" + f.Name
	key := sha256.Sum256([]byte(q.Provider + "\n" + q.ID + "\n" + chosen.Version + "\n" + chosen.Name + "\n" + role + "\n" + profile))
	m := StudioModel{ID: "community-" + hex.EncodeToString(key[:8]), Name: d.Model.Name + " · " + path.Base(chosen.Name), Engine: "comfy", Recipe: recipe, Description: "Imported from " + q.Provider + ". " + d.Note, License: d.Model.License, Source: d.Model.Source, Width: width, Steps: steps, Guidance: guidance, RAMGB: 16, Files: []DownloadSpec{f}, BaseModel: chosen.BaseModel}
	s.mu.Lock()
	found := false
	for i := range s.community {
		if s.community[i].ID == m.ID {
			s.community[i] = m
			found = true
			break
		}
	}
	if !found {
		s.community = append(s.community, m)
	}
	s.mu.Unlock()
	s.save()
	return m, nil
}
func validModelRole(role string) bool {
	switch role {
	case "checkpoints", "diffusion_models", "text_encoders", "vae", "loras", "controlnet", "clip_vision", "upscale_models", "embeddings":
		return true
	}
	return false
}
