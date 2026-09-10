package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const originCommunityRepo = "HUGELU/HASL"

var contributionRepo = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

func contributionCall(ctx context.Context, c *http.Client, token, method, path string, payload any) (map[string]any, error) {
	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}
	r, err := http.NewRequestWithContext(ctx, method, "https://api.github.com"+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Accept", "application/vnd.github+json")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("User-Agent", "ORIGIN0-concept-contribution")
	res, err := c.Do(r)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err = json.Unmarshal(b, &out); err != nil {
		return nil, errors.New("GitHub returned an unreadable response")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("GitHub HTTP %d: %s", res.StatusCode, shortText(fmt.Sprint(out["message"]), 300))
	}
	return out, nil
}
func (e *Engine) publishConcept(ctx context.Context, c *http.Client, id, digest, repo, token string) (string, error) {
	if !contributionRepo.MatchString(repo) || len(token) < 15 || len(token) > 512 {
		return "", errors.New("enter your writable HASL fork (owner/repo) and a GitHub token with Contents/Pull requests write access")
	}
	p, err := e.conceptProject(id)
	if err != nil {
		return "", err
	}
	content, err := canonicalRecipe(p)
	if err != nil {
		return "", err
	}
	if recipeDigest(content) != digest {
		return "", errors.New("project changed; preview the exact recipe again before publishing")
	}
	if len(projectLearning(p).Examples) == 0 {
		return "", errors.New("accept at least one example before contributing a recipe")
	}
	info, err := contributionCall(ctx, c, token, "GET", "/repos/"+repo, nil)
	if err != nil {
		return "", err
	}
	if repo != originCommunityRepo {
		source, _ := info["source"].(map[string]any)
		parent, _ := info["parent"].(map[string]any)
		if source["full_name"] != originCommunityRepo && parent["full_name"] != originCommunityRepo {
			return "", errors.New("the destination must be your fork of HUGELU/HASL")
		}
	}
	permissions, _ := info["permissions"].(map[string]any)
	if permissions["push"] != true {
		return "", errors.New("this token cannot write to that repository; select your own writable HASL fork")
	}
	branch, _ := info["default_branch"].(string)
	if branch == "" {
		return "", errors.New("repository has no default branch")
	}
	ref, err := contributionCall(ctx, c, token, "GET", "/repos/"+repo+"/git/ref/heads/"+url.PathEscape(branch), nil)
	if err != nil {
		return "", err
	}
	object, _ := ref["object"].(map[string]any)
	sha, _ := object["sha"].(string)
	if len(sha) != 40 {
		return "", errors.New("invalid repository base commit")
	}
	upstream := info
	if repo != originCommunityRepo {
		upstream, err = contributionCall(ctx, c, token, "GET", "/repos/"+originCommunityRepo, nil)
		if err != nil {
			return "", err
		}
	}
	base, _ := upstream["default_branch"].(string)
	if base == "" {
		return "", errors.New("upstream default branch unavailable")
	}
	newBranch := "origin0-recipe-" + randomID()[:16]
	if _, err = contributionCall(ctx, c, token, "POST", "/repos/"+repo+"/git/refs", map[string]any{"ref": "refs/heads/" + newBranch, "sha": sha}); err != nil {
		return "", err
	}
	path := "recipes/" + digest[:16] + ".json"
	message := "Add concept recipe: " + shortText(p.Name, 100)
	if _, err = contributionCall(ctx, c, token, "PUT", "/repos/"+repo+"/contents/"+path, map[string]any{"message": message, "branch": newBranch, "content": base64.StdEncoding.EncodeToString(content)}); err != nil {
		return "", fmt.Errorf("recipe branch %s exists, but its file could not be written: %w", newBranch, err)
	}
	head := strings.Split(repo, "/")[0] + ":" + newBranch
	body := "This contribution adds a reviewed concept recipe exported from ORIGIN-0.\n\nIt includes accepted example metadata, source attribution, grouped splits and any recorded classifier evaluations. Image bytes, model weights, access keys and local paths are excluded.\n\nRecipe SHA-256: `" + digest + "`\n\nReview the source licences, labels and evaluation before reusing the recipe. This is data for reproducible experiments; it does not by itself change the shared model or kernel."
	pr, err := contributionCall(ctx, c, token, "POST", "/repos/"+originCommunityRepo+"/pulls", map[string]any{"title": message, "head": head, "base": base, "body": body, "draft": true})
	if err != nil {
		return "", fmt.Errorf("recipe saved on %s branch %s; draft pull request could not be opened: %w", repo, newBranch, err)
	}
	link, _ := pr["html_url"].(string)
	if !strings.HasPrefix(link, "https://github.com/"+originCommunityRepo+"/pull/") {
		return "", errors.New("GitHub did not return a valid contribution link")
	}
	return link, nil
}
func (e *Engine) contributionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/concepts/preview-contribution", func(w http.ResponseWriter, r *http.Request) {
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
		b, err := canonicalRecipe(p)
		if err != nil {
			apiError(w, err)
			return
		}
		jsonReply(w, map[string]string{"content": string(b), "sha256": recipeDigest(b)})
	})
	mux.HandleFunc("/api/concepts/publish", func(w http.ResponseWriter, r *http.Request) {
		var x struct{ ID, Digest, Repo, Token string }
		if err := decode(r, &x); err != nil {
			apiError(w, err)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()
		client := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("GitHub redirects disabled") }}
		link, err := e.publishConcept(ctx, client, x.ID, x.Digest, x.Repo, x.Token)
		x.Token = ""
		if err != nil {
			apiError(w, err)
			return
		}
		jsonReply(w, map[string]string{"url": link})
	})
}
