package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const relayMessageLimit = 24 << 20

// The relay sees opaque envelopes. Owner and worker requests are authenticated
// inside AES-GCM; associated data binds direction, room and request identity.
type RelayInvitation struct {
	URL   string `json:"url"`
	Room  string `json:"room"`
	Guest string `json:"guest"`
	Key   string `json:"key"`
}
type relayRegistration struct {
	Room  string `json:"room"`
	Host  string `json:"host"`
	Guest string `json:"guest"`
}
type relayEnvelope struct {
	ID   string `json:"id"`
	Data []byte `json:"data"`
}
type relayRequest struct {
	Path  string `json:"path"`
	Token string `json:"token"`
	Job   string `json:"job"`
	Lease string `json:"lease"`
	Body  []byte `json:"body"`
}
type relayResponse struct {
	Status int    `json:"status"`
	Body   []byte `json:"body"`
}
type relayPending struct {
	relayEnvelope
	response chan []byte
}
type relayRoom struct {
	host, guest string
	seen        time.Time
	queue       chan *relayPending
	pending     map[string]*relayPending
	done        chan struct{}
}
type relayService struct {
	mu       sync.Mutex
	admin    string
	rooms    map[string]*relayRoom
	reserved int64
}

func relayCrypt(key, direction, room, id string, data []byte, seal bool) ([]byte, error) {
	k, err := hex.DecodeString(key)
	if err != nil || len(k) != 32 {
		return nil, errors.New("invalid relay encryption key")
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}
	a, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	aad := []byte("ORIGIN0-relay-v1|" + direction + "|" + room + "|" + id)
	if seal {
		nonce := make([]byte, a.NonceSize())
		if _, err = io.ReadFull(crand.Reader, nonce); err != nil {
			return nil, err
		}
		return a.Seal(nonce, nonce, data, aad), nil
	}
	if len(data) < a.NonceSize() {
		return nil, errors.New("truncated relay envelope")
	}
	return a.Open(nil, data[:a.NonceSize()], data[a.NonceSize():], aad)
}
func validRelayURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && (u.Path == "" || u.Path == "/")
}
func relayHTTPClient() *http.Client {
	return &http.Client{Timeout: 45 * time.Second, Transport: &http.Transport{Proxy: http.ProxyFromEnvironment, ResponseHeaderTimeout: 42 * time.Second, TLSHandshakeTimeout: 10 * time.Second, IdleConnTimeout: 30 * time.Second}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("relay redirects are disabled") }}
}
func relayCall(ctx context.Context, c *http.Client, base, path, token string, body []byte) ([]byte, error) {
	r, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(base, "/")+"/origin-relay/"+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/octet-stream")
	res, err := c.Do(r)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode == 204 {
		return nil, nil
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("relay HTTP %d: %s", res.StatusCode, tail(string(b), 250))
	}
	return b, nil
}
func relayWorkerPost(ctx context.Context, c *http.Client, v PoolInvite, path string, body []byte, headers map[string]string) ([]byte, error) {
	rv := v.Relay
	id := randomID()
	req := relayRequest{Path: path, Token: v.Token, Body: body, Job: headers["X-Job-ID"], Lease: headers["X-Job-Lease"]}
	b, _ := json.Marshal(req)
	if len(b) > relayMessageLimit {
		return nil, errors.New("worker result exceeds relay envelope limit")
	}
	sealed, err := relayCrypt(rv.Key, "request", rv.Room, id, b, true)
	if err != nil {
		return nil, err
	}
	out, err := relayCall(ctx, c, rv.URL, "send/"+rv.Room+"/"+id, rv.Guest, sealed)
	if err != nil {
		return nil, err
	}
	plain, err := relayCrypt(rv.Key, "response", rv.Room, id, out, false)
	if err != nil {
		return nil, errors.New("relay response authentication failed")
	}
	var res relayResponse
	if err = json.Unmarshal(plain, &res); err != nil {
		return nil, err
	}
	if res.Status != 200 {
		return nil, fmt.Errorf("worker group HTTP %d: %s", res.Status, tail(string(res.Body), 400))
	}
	return res.Body, nil
}

type relayRecorder struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *relayRecorder) Header() http.Header { return w.header }
func (w *relayRecorder) WriteHeader(n int) {
	if w.status == 0 {
		w.status = n
	}
}
func (w *relayRecorder) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	if w.body.Len()+len(b) > 1<<20 {
		return 0, errors.New("relay response too large")
	}
	return w.body.Write(b)
}
func (p *ComputePool) dispatchRelay(ctx context.Context, rv RelayInvitation, env relayEnvelope) []byte {
	plain, err := relayCrypt(rv.Key, "request", rv.Room, env.ID, env.Data, false)
	if err != nil {
		return nil
	}
	var req relayRequest
	if json.Unmarshal(plain, &req) != nil {
		return nil
	}
	allowed := map[string]bool{"/pool/lease": true, "/pool/heartbeat": true, "/pool/result": true, "/pool/fail": true}
	if !allowed[req.Path] {
		return nil
	}
	r, err := http.NewRequestWithContext(ctx, "POST", "https://localhost"+req.Path, bytes.NewReader(req.Body))
	if err != nil {
		return nil
	}
	r.Header.Set("Authorization", "Bearer "+req.Token)
	r.Header.Set("X-Job-ID", req.Job)
	r.Header.Set("X-Job-Lease", req.Lease)
	w := &relayRecorder{header: http.Header{}}
	p.handler().ServeHTTP(w, r)
	if w.status == 0 {
		w.status = 200
	}
	b, _ := json.Marshal(relayResponse{Status: w.status, Body: w.body.Bytes()})
	out, _ := relayCrypt(rv.Key, "response", rv.Room, env.ID, b, true)
	return out
}
func (p *ComputePool) hostInternet(base, key string) error {
	return p.hostInternetClient(base, key, relayHTTPClient())
}
func (p *ComputePool) hostInternetClient(base, key string, c *http.Client) error {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if !validRelayURL(base) || len(key) < 32 || len(key) > 256 {
		return errors.New("enter an HTTPS relay URL and its operator-issued access key")
	}
	if err := p.host("127.0.0.1:0", "127.0.0.1"); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	b, err := relayCall(ctx, c, base, "register", key, nil)
	if err != nil {
		cancel()
		p.stopHost()
		return err
	}
	var reg relayRegistration
	if json.Unmarshal(b, &reg) != nil || !validID(reg.Room) || len(reg.Host) < 32 || len(reg.Guest) < 32 {
		cancel()
		p.stopHost()
		return errors.New("invalid relay registration")
	}
	rv := RelayInvitation{URL: base, Room: reg.Room, Guest: reg.Guest, Key: (randomID() + randomID())[:64]}
	p.mu.Lock()
	if p.closed || p.server == nil {
		p.mu.Unlock()
		cancel()
		return errors.New("group was stopped while connecting")
	}
	p.relay = &rv
	p.relayCancel = cancel
	p.relayStatus = "Connected through internet relay"
	p.wg.Add(1)
	p.mu.Unlock()
	go func() {
		defer p.wg.Done()
		defer cancel()
		defer func() {
			cleanup, done := context.WithTimeout(context.Background(), 3*time.Second)
			defer done()
			_, _ = relayCall(cleanup, c, base, "delete/"+reg.Room, reg.Host, nil)
		}()
		for ctx.Err() == nil {
			b, er := relayCall(ctx, c, base, "pull/"+reg.Room, reg.Host, nil)
			if er != nil {
				p.mu.Lock()
				if p.relay == &rv {
					p.relayStatus = "Relay reconnecting: " + tail(er.Error(), 180)
				}
				p.mu.Unlock()
				if !poolWait(ctx, 2*time.Second) {
					return
				}
				continue
			}
			if len(b) == 0 {
				continue
			}
			var env relayEnvelope
			if json.Unmarshal(b, &env) != nil || !validID(env.ID) {
				continue
			}
			out := p.dispatchRelay(ctx, rv, env)
			if out == nil {
				out = []byte("invalid envelope")
			}
			_, er = relayCall(ctx, c, base, "result/"+reg.Room+"/"+env.ID, reg.Host, out)
			p.mu.Lock()
			if p.relay == &rv {
				p.relayStatus = "Connected through internet relay"
				if er != nil {
					p.relayStatus = "Relay delivery interrupted; worker may retry"
				}
			}
			p.mu.Unlock()
		}
	}()
	return nil
}

func newRelayService(admin string) *relayService {
	return &relayService{admin: admin, rooms: map[string]*relayRoom{}}
}
func secretEqual(a, b string) bool {
	return len(a) > 0 && len(a) == len(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
func (s *relayService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.URL.Path == "/health" && r.Method == "GET" {
		_, _ = io.WriteString(w, "ORIGIN0 relay ready")
		return
	}
	if r.Method != "POST" {
		http.Error(w, "POST required", 405)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/origin-relay/"), "/")
	if !strings.HasPrefix(r.URL.Path, "/origin-relay/") || len(parts) > 3 {
		http.NotFound(w, r)
		return
	}
	auth := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	s.mu.Lock()
	for id, room := range s.rooms {
		if time.Since(room.seen) > 20*time.Minute {
			close(room.done)
			delete(s.rooms, id)
		}
	}
	if len(parts) == 1 && parts[0] == "register" {
		if !secretEqual(auth, s.admin) || len(s.admin) < 32 {
			s.mu.Unlock()
			http.Error(w, "relay registration key required", 401)
			return
		}
		if len(s.rooms) >= 8 {
			s.mu.Unlock()
			http.Error(w, "relay group capacity reached", 429)
			return
		}
		reg := relayRegistration{Room: randomID(), Host: randomID() + randomID(), Guest: randomID() + randomID()}
		s.rooms[reg.Room] = &relayRoom{host: reg.Host, guest: reg.Guest, seen: time.Now(), queue: make(chan *relayPending, 8), pending: map[string]*relayPending{}, done: make(chan struct{})}
		s.mu.Unlock()
		jsonReply(w, reg)
		return
	}
	if len(parts) < 2 {
		s.mu.Unlock()
		http.NotFound(w, r)
		return
	}
	room := s.rooms[parts[1]]
	if room == nil {
		s.mu.Unlock()
		http.Error(w, "unknown or expired relay group", 404)
		return
	}
	host := secretEqual(auth, room.host)
	guest := secretEqual(auth, room.guest)
	if !host && !(parts[0] == "send" && guest) {
		s.mu.Unlock()
		http.Error(w, "relay access denied", 401)
		return
	}
	if host {
		room.seen = time.Now()
	}
	s.mu.Unlock()
	switch parts[0] {
	case "delete":
		s.mu.Lock()
		if s.rooms[parts[1]] == room {
			delete(s.rooms, parts[1])
			close(room.done)
		}
		s.mu.Unlock()
		w.WriteHeader(204)
	case "pull":
		t := time.NewTimer(15 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-room.done:
				http.Error(w, "group stopped", 410)
				return
			case <-r.Context().Done():
				return
			case <-t.C:
				w.WriteHeader(204)
				return
			case p := <-room.queue:
				s.mu.Lock()
				current := room.pending[p.ID] == p
				env := p.relayEnvelope
				s.mu.Unlock()
				if current {
					jsonReply(w, env)
					return
				}
			}
		}
	case "send":
		if len(parts) != 3 || !validID(parts[2]) {
			http.Error(w, "invalid request identity", 400)
			return
		}
		// Reserve a fixed slot before reading; aggregate envelope memory is bounded.
		s.mu.Lock()
		if s.reserved+relayMessageLimit > 96<<20 || len(room.pending) >= 8 {
			s.mu.Unlock()
			http.Error(w, "relay busy; retry later", 429)
			return
		}
		if room.pending[parts[2]] != nil {
			s.mu.Unlock()
			http.Error(w, "duplicate request", 409)
			return
		}
		s.reserved += relayMessageLimit
		p := &relayPending{relayEnvelope: relayEnvelope{ID: parts[2]}, response: make(chan []byte, 1)}
		room.pending[p.ID] = p
		s.mu.Unlock()
		defer func() {
			s.mu.Lock()
			delete(room.pending, p.ID)
			p.Data = nil
			s.reserved -= relayMessageLimit
			s.mu.Unlock()
		}()
		r.Body = http.MaxBytesReader(w, r.Body, relayMessageLimit+64)
		b, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "relay envelope too large", 413)
			return
		}
		s.mu.Lock()
		p.Data = b
		s.mu.Unlock()
		select {
		case room.queue <- p:
		default:
			http.Error(w, "relay queue full", 429)
			return
		}
		t := time.NewTimer(38 * time.Second)
		defer t.Stop()
		select {
		case <-room.done:
			http.Error(w, "group stopped", 410)
		case <-r.Context().Done():
		case <-t.C:
			http.Error(w, "coordinator did not respond", 504)
		case out := <-p.response:
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(out)
		}
	case "result":
		if len(parts) != 3 {
			http.Error(w, "request identity required", 400)
			return
		}
		s.mu.Lock()
		p := room.pending[parts[2]]
		s.mu.Unlock()
		if p == nil {
			http.Error(w, "request expired", 409)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
		b, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "response too large", 413)
			return
		}
		select {
		case p.response <- b:
			w.WriteHeader(204)
		default:
			http.Error(w, "response already supplied", 409)
		}
	default:
		http.NotFound(w, r)
	}
}

func runInternetRelay() error {
	key := os.Getenv("ORIGIN0_RELAY_ACCESS_KEY")
	if len(key) < 32 {
		return errors.New("set ORIGIN0_RELAY_ACCESS_KEY to a random secret of at least 32 characters")
	}
	listen := os.Getenv("ORIGIN0_RELAY_LISTEN")
	if listen == "" {
		listen = "127.0.0.1:8790"
	}
	cert, keyFile := os.Getenv("ORIGIN0_RELAY_TLS_CERT"), os.Getenv("ORIGIN0_RELAY_TLS_KEY")
	if cert == "" || keyFile == "" {
		host, _, err := net.SplitHostPort(listen)
		if err != nil || !net.ParseIP(host).IsLoopback() {
			return errors.New("without TLS certificate/key, bind the relay to loopback behind an HTTPS reverse proxy")
		}
	}
	srv := &http.Server{Addr: listen, Handler: newRelayService(key), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 50 * time.Second, WriteTimeout: 50 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8 << 10}
	fmt.Println("ORIGIN0 relay listening on", listen, "— no model execution; encrypted envelopes only")
	if cert != "" && keyFile != "" {
		return srv.ListenAndServeTLS(cert, keyFile)
	}
	return srv.ListenAndServe()
}
