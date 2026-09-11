package main

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

// ReverseProxy supplies standard-library WebSocket upgrade handling. The local
// session is checked before upgrading; no API key appears in a query string.
func (e *Engine) comfyLive(w http.ResponseWriter, r *http.Request) {
	origin, err := url.Parse(r.Header.Get("Origin"))
	if err != nil || origin.Host != r.Host || r.Method != "GET" || !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		http.Error(w, "same-origin WebSocket required", 403)
		return
	}
	authorised := false
	for _, p := range strings.Split(r.Header.Get("Sec-WebSocket-Protocol"), ",") {
		if strings.TrimSpace(p) == e.sessionKey {
			authorised = true
		}
	}
	if !authorised || e.privacy.isLocked() {
		http.Error(w, "unlock this local session first", 401)
		return
	}
	id := r.URL.Query().Get("clientId")
	s := e.mediaStudio
	s.mu.Lock()
	var base string
	for _, j := range s.jobs {
		if j.ID == id {
			base = j.URL
			break
		}
	}
	s.mu.Unlock()
	if base == "" {
		http.NotFound(w, r)
		return
	}
	target, err := url.Parse(base)
	if err != nil || validComfyURL(base) != nil {
		http.Error(w, "invalid engine address", 400)
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-e.stop:
				cancel()
				return
			case <-ticker.C:
				if e.privacy.isLocked() {
					cancel()
					return
				}
			}
		}
	}()
	proxy := httputil.NewSingleHostReverseProxy(target)
	original := proxy.Director
	proxy.Director = func(req *http.Request) {
		original(req)
		req.URL.Path = "/ws"
		req.URL.RawPath = ""
		req.Host = target.Host
		req.Header.Del("Origin")
		req.Header.Del("Sec-WebSocket-Protocol")
		req.Header.Del("Cookie")
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		if resp.StatusCode == http.StatusSwitchingProtocols {
			resp.Header.Set("Sec-WebSocket-Protocol", "origin0")
		}
		return nil
	}
	proxy.ServeHTTP(w, r.WithContext(ctx))
}
