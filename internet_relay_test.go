package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRelayEncryptionBindsDirectionRoomAndRequest(t *testing.T) {
	key := strings.Repeat("ab", 32)
	plain := []byte("private prompt and worker credentials")
	b, err := relayCrypt(key, "request", "room", "request1", plain, true)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, plain) {
		t.Fatal("plaintext envelope")
	}
	out, err := relayCrypt(key, "request", "room", "request1", b, false)
	if err != nil || !bytes.Equal(out, plain) {
		t.Fatal(err)
	}
	for _, args := range [][3]string{{"response", "room", "request1"}, {"request", "other", "request1"}, {"request", "room", "request2"}} {
		if _, err = relayCrypt(key, args[0], args[1], args[2], b, false); err == nil {
			t.Fatal("envelope replay accepted")
		}
	}
	b[len(b)-1] ^= 1
	if _, err = relayCrypt(key, "request", "room", "request1", b, false); err == nil {
		t.Fatal("tampered envelope accepted")
	}
}
func TestRelayEndToEndExistingPoolProtocol(t *testing.T) {
	e := studioEngine(t)
	service := newRelayService(strings.Repeat("r", 40))
	server := httptest.NewTLSServer(service)
	defer server.Close()
	p := e.images.pool
	if err := p.hostInternetClient(server.URL, strings.Repeat("r", 40), server.Client()); err != nil {
		t.Fatal(err)
	}
	defer p.stopHost()
	raw, err := p.invite()
	if err != nil {
		t.Fatal(err)
	}
	v, err := parsePoolInvite(raw)
	if err != nil || v.Relay == nil {
		t.Fatal(err)
	}
	j, err := e.images.submit(ImageRequest{Prompt: "A test square", Width: 256, Height: 256, Steps: 1, Seed: 42, Shared: true})
	if err != nil {
		t.Fatal(err)
	}
	ctx, done := context.WithTimeout(context.Background(), 8*time.Second)
	defer done()
	out, err := poolPost(ctx, server.Client(), v, "/pool/lease", map[string]string{"name": "remote worker", "pack": e.images.catalog.ID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var lease struct {
		Job   *ImageJob `json:"job"`
		Lease string    `json:"lease"`
	}
	if err = json.Unmarshal(out, &lease); err != nil || lease.Job == nil || lease.Job.ID != j.ID || lease.Lease == "" {
		t.Fatalf("wrong relay lease: %s %v", out, err)
	}
	if _, err = poolPost(ctx, server.Client(), v, "/pool/heartbeat", map[string]string{"id": j.ID, "lease": lease.Lease, "message": "working through relay"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = poolPost(ctx, server.Client(), v, "/pool/result", []byte("not a png"), map[string]string{"X-Job-ID": j.ID, "X-Job-Lease": lease.Lease}); err == nil {
		t.Fatal("invalid PNG accepted through relay")
	}
	if _, err = poolPost(ctx, server.Client(), v, "/pool/fail", map[string]string{"id": j.ID, "lease": lease.Lease, "message": "test failure"}, nil); err != nil {
		t.Fatal(err)
	}
	bad := v
	bad.Token = strings.Repeat("x", 96)
	if _, err = poolPost(ctx, server.Client(), bad, "/pool/lease", map[string]string{}, nil); err == nil {
		t.Fatal("inner worker authentication bypassed")
	}
	p.stopHost()
	p.wg.Wait()
	service.mu.Lock()
	rooms := len(service.rooms)
	reserved := service.reserved
	service.mu.Unlock()
	if rooms != 0 || reserved != 0 {
		t.Fatalf("relay room/slots remain: %d %d", rooms, reserved)
	}
}
func TestRelayRegistrationAuthorizationAndCapacity(t *testing.T) {
	s := newRelayService(strings.Repeat("a", 32))
	request := func(token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "https://relay.example/origin-relay/register", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	if w := request("wrong"); w.Code != 401 {
		t.Fatal(w.Code)
	}
	for i := 0; i < 8; i++ {
		if w := request(strings.Repeat("a", 32)); w.Code != 200 {
			t.Fatal(w.Code)
		}
	}
	if w := request(strings.Repeat("a", 32)); w.Code != 429 {
		t.Fatal("room cap not enforced")
	}
}
func TestRelayGuestCannotPullOrInjectResult(t *testing.T) {
	service := newRelayService(strings.Repeat("a", 32))
	server := httptest.NewTLSServer(service)
	defer server.Close()
	b, err := relayCall(context.Background(), server.Client(), server.URL, "register", strings.Repeat("a", 32), nil)
	if err != nil {
		t.Fatal(err)
	}
	var reg relayRegistration
	_ = json.Unmarshal(b, &reg)
	for _, path := range []string{"pull/" + reg.Room, "result/" + reg.Room + "/abc123abc123", "delete/" + reg.Room} {
		if _, err = relayCall(context.Background(), server.Client(), server.URL, path, reg.Guest, nil); err == nil {
			t.Fatal("guest acquired host capability", path)
		}
	}
}
func TestRelayRejectsPlainHTTPAndRedirects(t *testing.T) {
	for _, u := range []string{"http://relay.example", "https://user:pass@relay.example", "https://relay.example/?key=x", "file:///tmp"} {
		if validRelayURL(u) {
			t.Fatal(u)
		}
	}
	if !validRelayURL("https://relay.example") {
		t.Fatal("valid HTTPS relay rejected")
	}
	c := relayHTTPClient()
	if c.CheckRedirect(&http.Request{}, nil) == nil {
		t.Fatal("redirect could forward relay token")
	}
}
func TestRelayDisconnectReleasesPendingRequest(t *testing.T) {
	s := newRelayService(strings.Repeat("a", 32))
	server := httptest.NewTLSServer(s)
	defer server.Close()
	b, _ := relayCall(context.Background(), server.Client(), server.URL, "register", strings.Repeat("a", 32), nil)
	var reg relayRegistration
	_ = json.Unmarshal(b, &reg)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = relayCall(ctx, server.Client(), server.URL, "send/"+reg.Room+"/abc123abc123", reg.Guest, []byte("opaque"))
	}()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		n := s.reserved
		s.mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker cancellation stuck")
	}
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		n := s.reserved
		s.mu.Unlock()
		if n == 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("disconnected request held reserved memory")
}
