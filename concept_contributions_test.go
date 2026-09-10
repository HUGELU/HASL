package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestConceptContributionUsesReviewedDigestAndDraftPR(t *testing.T) {
	e := studioEngine(t)
	_, _ = e.seedConcept("abc123abc123abcd", "horizontal")
	p, _ := e.conceptProject("abc123abc123abcd")
	b, _ := canonicalRecipe(p)
	count := 0
	client := &http.Client{Transport: conceptRoundTrip(func(r *http.Request) (*http.Response, error) {
		count++
		if r.URL.Host != "api.github.com" || r.Header.Get("Authorization") != "Bearer test-token-for-github" {
			t.Fatal("credential destination or authorization")
		}
		switch count {
		case 1:
			return conceptResponse([]byte(`{"default_branch":"main","source":{"full_name":"HUGELU/HASL"},"permissions":{"push":true}}`)), nil
		case 2:
			return conceptResponse([]byte(`{"object":{"sha":"` + strings.Repeat("a", 40) + `"}}`)), nil
		case 3:
			return conceptResponse([]byte(`{"default_branch":"main"}`)), nil
		case 4:
			if r.Method != "POST" || !strings.HasSuffix(r.URL.Path, "/git/refs") {
				t.Fatal(r.URL)
			}
			return conceptResponse([]byte(`{}`)), nil
		case 5:
			var obj map[string]any
			_ = json.NewDecoder(r.Body).Decode(&obj)
			raw, err := base64.StdEncoding.DecodeString(obj["content"].(string))
			if err != nil || !strings.HasPrefix(obj["branch"].(string), "origin0-recipe-") || recipeDigest(raw) != recipeDigest(b) || strings.Contains(string(raw), e.dataDir) {
				t.Fatal("wrong or private recipe content")
			}
			return conceptResponse([]byte(`{}`)), nil
		case 6:
			var obj map[string]any
			_ = json.NewDecoder(r.Body).Decode(&obj)
			if obj["draft"] != true || obj["base"] != "main" || r.URL.Path != "/repos/HUGELU/HASL/pulls" {
				t.Fatal(obj)
			}
			return conceptResponse([]byte(`{"html_url":"https://github.com/HUGELU/HASL/pull/123"}`)), nil
		default:
			t.Fatal("unexpected request")
			return nil, nil
		}
	})}
	link, err := e.publishConcept(context.Background(), client, p.ID, recipeDigest(b), "Contributor/HASL", "test-token-for-github")
	if err != nil || count != 6 || !strings.HasSuffix(link, "123") {
		t.Fatal(link, err, count)
	}
}
func TestConceptContributionStalePreviewNeverConnects(t *testing.T) {
	e := studioEngine(t)
	client := &http.Client{Transport: conceptRoundTrip(func(*http.Request) (*http.Response, error) {
		t.Fatal("stale preview connected to GitHub")
		return nil, nil
	})}
	if _, err := e.publishConcept(context.Background(), client, "abc123abc123abcd", "old digest", "Contributor/HASL", "test-token-for-github"); err == nil {
		t.Fatal("stale preview accepted")
	}
}
func TestConceptContributionUnknownForkNeverWrites(t *testing.T) {
	e := studioEngine(t)
	_, _ = e.seedConcept("abc123abc123abcd", "horizontal")
	p, _ := e.conceptProject("abc123abc123abcd")
	b, _ := canonicalRecipe(p)
	client := &http.Client{Transport: conceptRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" {
			t.Fatal("unrelated repository mutated")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"default_branch":"main","permissions":{"push":true},"source":{"full_name":"Other/Project"}}`)), Header: http.Header{}}, nil
	})}
	if _, err := e.publishConcept(context.Background(), client, p.ID, recipeDigest(b), "User/Other", "test-token-for-github"); err == nil {
		t.Fatal("unrelated destination accepted")
	}
}
