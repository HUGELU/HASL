package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type PoolPeer struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Seen      int64  `json:"seen"`
	Completed int    `json:"completed"`
}
type PoolInvite struct {
	URL   string `json:"url"`
	Pin   string `json:"pin"`
	Token string `json:"token"`
}
type ComputePool struct {
	s             *NativeImages
	mu            sync.Mutex
	wg            sync.WaitGroup
	closed        bool
	server        *http.Server
	address       string
	pin           string
	members       map[string]*PoolPeer
	joinCancel    context.CancelFunc
	joinState     string
	joinCompleted int
	joinBudget    int
}

func newComputePool(s *NativeImages) *ComputePool {
	return &ComputePool{s: s, members: map[string]*PoolPeer{}, joinState: "Not joined"}
}
func (p *ComputePool) status() any {
	p.mu.Lock()
	defer p.mu.Unlock()
	peers := []PoolPeer{}
	for _, x := range p.members {
		peers = append(peers, *x)
	}
	return map[string]any{"hosting": p.server != nil, "address": p.address, "peers": peers, "joined": p.joinCancel != nil, "worker_status": p.joinState, "completed": p.joinCompleted, "budget": p.joinBudget}
}
func poolCertificate() (tls.Certificate, string, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	if err != nil {
		return tls.Certificate{}, "", err
	}
	tmpl := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "ORIGIN0 volunteer image group"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(48 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	h := sha256.Sum256(der)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, hex.EncodeToString(h[:]), nil
}
func (p *ComputePool) host(listen, advertise string) error {
	if listen == "" {
		listen = "0.0.0.0:8770"
	}
	host, port, err := net.SplitHostPort(listen)
	if err != nil || port == "" {
		return errors.New("enter a listen address such as 0.0.0.0:8770")
	}
	if host != "" && net.ParseIP(host) == nil {
		return errors.New("listen address must be a local IP address")
	}
	cert, pin, err := poolCertificate()
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.server != nil {
		return errors.New("a group is already running or ORIGIN-0 is stopping")
	}
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	_, actualPort, _ := net.SplitHostPort(ln.Addr().String())
	if advertise == "" {
		advertise = host
		if advertise == "0.0.0.0" || advertise == "" {
			advertise = "127.0.0.1"
			if addrs, err := net.InterfaceAddrs(); err == nil {
				for _, a := range addrs {
					if ip, _, err := net.ParseCIDR(a.String()); err == nil && ip.To4() != nil && !ip.IsLoopback() {
						advertise = ip.String()
						break
					}
				}
			}
		}
	}
	if net.ParseIP(advertise) == nil {
		if strings.ContainsAny(advertise, "/:?#@ ") {
			ln.Close()
			return errors.New("advertised host must be an IP address or a DNS hostname")
		}
	}
	p.address = "https://" + net.JoinHostPort(advertise, actualPort)
	p.pin = pin
	p.members = map[string]*PoolPeer{}
	p.server = &http.Server{Handler: p.handler(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 90 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	srv := p.server
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		_ = srv.Serve(tls.NewListener(ln, &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{cert}}))
	}()
	return nil
}
func (p *ComputePool) invite() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.server == nil {
		return "", errors.New("start a worker group first")
	}
	if len(p.members) >= 16 {
		return "", errors.New("16 invitations already exist; restart the group to clear old invitations")
	}
	token := randomID() + randomID()
	p.members[token] = &PoolPeer{ID: "peer-" + randomID()[:12], Name: "Invitation waiting"}
	b, _ := json.Marshal(PoolInvite{p.address, p.pin, token})
	return "origin0:" + base64.RawURLEncoding.EncodeToString(b), nil
}
func (p *ComputePool) stopHost() {
	p.mu.Lock()
	srv := p.server
	p.server = nil
	p.members = map[string]*PoolPeer{}
	p.address = ""
	p.pin = ""
	p.mu.Unlock()
	if srv != nil {
		_ = srv.Close()
	}
	p.s.mu.Lock()
	for _, j := range p.s.jobs {
		if j.Request.Shared && (j.Status == "running" || j.Status == "queued") {
			j.Status = "cancelled"
			j.Message = "Worker group stopped"
			j.Lease = ""
			j.Finished = now()
		}
	}
	p.s.mu.Unlock()
	p.s.save()
}
func (p *ComputePool) leave() {
	p.mu.Lock()
	if p.joinCancel != nil {
		p.joinCancel()
	}
	p.mu.Unlock()
}
func (p *ComputePool) close() {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	p.leave()
	p.stopHost()
	p.wg.Wait()
}
func (p *ComputePool) auth(r *http.Request) (PoolPeer, bool) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	p.mu.Lock()
	defer p.mu.Unlock()
	for secret, x := range p.members {
		if subtle.ConstantTimeCompare([]byte(secret), []byte(token)) == 1 {
			x.Seen = now()
			return *x, true
		}
	}
	return PoolPeer{}, false
}
func (p *ComputePool) expireLocked(t int64) {
	for _, j := range p.s.jobs {
		if j.Request.Shared && j.Status == "running" && j.LeaseUntil < t {
			j.Lease = ""
			j.Worker = ""
			if j.Attempts >= 2 {
				j.Status = "failed"
				j.Message = "Worker disconnected twice; submit again when a worker is available"
				j.Finished = t
			} else {
				j.Status = "queued"
				j.Message = "Worker disconnected; waiting for another worker"
			}
		}
	}
}
func (p *ComputePool) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}
		peer, ok := p.auth(r)
		if !ok {
			http.Error(w, "invalid or revoked invitation", 401)
			return
		}
		s := p.s
		if r.URL.Path == "/pool/result" {
			r.Body = http.MaxBytesReader(w, r.Body, 16<<20)
			b, err := io.ReadAll(r.Body)
			if err != nil {
				apiError(w, err)
				return
			}
			id, lease := r.Header.Get("X-Job-ID"), r.Header.Get("X-Job-Lease")
			s.mu.Lock()
			defer s.save()
			defer s.mu.Unlock()
			p.expireLocked(now())
			var job *ImageJob
			for _, j := range s.jobs {
				if j.ID == id && j.Status == "running" && j.Worker == peer.ID && j.Lease == lease && lease != "" {
					job = j
					break
				}
			}
			if job == nil {
				http.Error(w, "expired, cancelled or unknown job lease", 409)
				return
			}
			if err := validatePNG(b, job.Request); err != nil {
				apiError(w, err)
				return
			}
			a, err := s.e.storeObject(bytes.NewReader(b), job.ID+".png", "image/png", "volunteer worker:"+peer.ID)
			if err != nil {
				apiError(w, err)
				return
			}
			job.Asset = &a
			job.Status = "completed"
			job.Finished = now()
			job.Message = "Image returned by " + peer.Name
			job.Lease = ""
			p.mu.Lock()
			for _, x := range p.members {
				if x.ID == peer.ID {
					x.Completed++
				}
			}
			p.mu.Unlock()
			jsonReply(w, map[string]any{"accepted": true})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16384)
		var v struct {
			Name    string `json:"name"`
			Pack    string `json:"pack"`
			ID      string `json:"id"`
			Lease   string `json:"lease"`
			Message string `json:"message"`
		}
		if err := decode(r, &v); err != nil {
			apiError(w, err)
			return
		}
		switch r.URL.Path {
		case "/pool/lease":
			if v.Pack != s.catalog.ID {
				http.Error(w, "worker model pack differs from this group", 409)
				return
			}
			if len(v.Name) > 80 {
				apiError(w, errors.New("worker name is too long"))
				return
			}
			p.mu.Lock()
			for _, x := range p.members {
				if x.ID == peer.ID && v.Name != "" {
					x.Name = v.Name
				}
			}
			p.mu.Unlock()
			s.mu.Lock()
			defer s.mu.Unlock()
			p.expireLocked(now())
			for _, j := range s.jobs {
				if j.Status == "running" && j.Worker == peer.ID {
					jsonReply(w, map[string]any{"job": nil})
					return
				}
			}
			for _, j := range s.jobs {
				if j.Status == "queued" && j.Request.Shared {
					j.Status = "running"
					j.Started = now()
					j.Worker = peer.ID
					j.Lease = randomID()
					j.LeaseUntil = now() + 120
					j.Attempts++
					j.Message = "Assigned to " + v.Name
					j.Backend = "volunteer"
					jsonReply(w, map[string]any{"job": *j, "lease": j.Lease})
					return
				}
			}
			jsonReply(w, map[string]any{"job": nil})
		case "/pool/heartbeat", "/pool/fail":
			s.mu.Lock()
			defer s.mu.Unlock()
			p.expireLocked(now())
			for _, j := range s.jobs {
				if j.ID == v.ID && j.Worker == peer.ID && j.Lease == v.Lease && v.Lease != "" && j.Status == "running" {
					if r.URL.Path == "/pool/fail" {
						j.Status = "failed"
						j.Finished = now()
						j.Lease = ""
						j.Message = tail(v.Message, 600)
					} else {
						j.LeaseUntil = now() + 120
						j.Message = tail(v.Message, 350)
					}
					jsonReply(w, map[string]any{"accepted": true})
					return
				}
			}
			http.Error(w, "lease no longer active", 409)
		default:
			http.NotFound(w, r)
		}
	})
}
func parsePoolInvite(raw string) (PoolInvite, error) {
	var v PoolInvite
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(strings.TrimSpace(raw), "origin0:"))
	if err != nil || json.Unmarshal(b, &v) != nil {
		return v, errors.New("paste the complete ORIGIN-0 invitation")
	}
	u, err := url.Parse(v.URL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || len(v.Pin) != 64 || len(v.Token) < 32 || len(v.Token) > 160 {
		return v, errors.New("invalid invitation")
	}
	if _, err = hex.DecodeString(v.Pin); err != nil {
		return v, errors.New("invalid TLS fingerprint")
	}
	return v, nil
}
func poolClient(v PoolInvite) *http.Client {
	// Self-signed peers are authenticated by the exact certificate fingerprint in
	// the invitation, not by system CA roots. No redirect may forward the token.
	return &http.Client{Timeout: 45 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, InsecureSkipVerify: true, VerifyConnection: func(cs tls.ConnectionState) error {
		if len(cs.PeerCertificates) != 1 {
			return errors.New("unexpected peer certificate")
		}
		h := sha256.Sum256(cs.PeerCertificates[0].Raw)
		if subtle.ConstantTimeCompare([]byte(hex.EncodeToString(h[:])), []byte(v.Pin)) != 1 {
			return errors.New("worker group fingerprint does not match invitation")
		}
		return nil
	}}}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("worker group redirects are disabled") }}
}
func poolPost(ctx context.Context, c *http.Client, v PoolInvite, path string, body any, headers map[string]string) ([]byte, error) {
	var data []byte
	var err error
	if b, ok := body.([]byte); ok {
		data = b
	} else {
		data, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, "POST", v.URL+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+v.Token)
	req.Header.Set("Content-Type", "application/json")
	for k, x := range headers {
		req.Header.Set(k, x)
	}
	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("worker group HTTP %d: %s", res.StatusCode, tail(string(b), 400))
	}
	return b, nil
}
func (p *ComputePool) join(raw, name string, budget int) error {
	v, err := parsePoolInvite(raw)
	if err != nil {
		return err
	}
	if budget < 1 || budget > 100 || len(name) > 80 || strings.TrimSpace(name) == "" {
		return errors.New("set a worker name and a budget of 1–100 image jobs")
	}
	p.mu.Lock()
	if p.closed || p.joinCancel != nil {
		p.mu.Unlock()
		return errors.New("already joined or stopping")
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.joinCancel = cancel
	p.joinBudget = budget
	p.joinCompleted = 0
	p.joinState = "Connecting to invited group"
	p.wg.Add(1)
	p.mu.Unlock()
	go func() {
		defer p.wg.Done()
		defer cancel()
		defer func() {
			p.mu.Lock()
			p.joinCancel = nil
			p.joinState = "Stopped — compute donation ended"
			p.mu.Unlock()
		}()
		client := poolClient(v)
		defer client.CloseIdleConnections()
		for attempts := 0; attempts < budget && ctx.Err() == nil; {
			p.s.mu.Lock()
			ready := p.s.ready && p.s.cancel == nil && !p.s.closed
			p.s.mu.Unlock()
			if !ready {
				p.workerMessage("Waiting for local image setup or a free generator")
				if !poolWait(ctx, 2*time.Second) {
					return
				}
				continue
			}
			b, err := poolPost(ctx, client, v, "/pool/lease", map[string]any{"name": name, "pack": p.s.catalog.ID}, nil)
			if err != nil {
				p.workerMessage(err.Error())
				if !poolWait(ctx, 5*time.Second) {
					return
				}
				continue
			}
			var answer struct {
				Job   *ImageJob `json:"job"`
				Lease string    `json:"lease"`
			}
			if err = json.Unmarshal(b, &answer); err != nil {
				p.workerMessage("Invalid group response")
				return
			}
			if answer.Job == nil {
				p.workerMessage("Joined — waiting for an image job")
				if !poolWait(ctx, 2*time.Second) {
					return
				}
				continue
			}
			attempts++
			if err := validateImageRequest(answer.Job.Request); err != nil || answer.Job.Pack != p.s.catalog.ID || !validID(answer.Job.ID) || len(answer.Job.ID) > 80 || len(answer.Lease) > 160 {
				p.workerMessage("Rejected invalid image task")
				return
			}
			err = p.runPeer(ctx, client, v, *answer.Job, answer.Lease)
			if err != nil {
				_, _ = poolPost(ctx, client, v, "/pool/fail", map[string]any{"id": answer.Job.ID, "lease": answer.Lease, "message": err.Error()}, nil)
				p.workerMessage(err.Error())
			} else {
				p.mu.Lock()
				p.joinCompleted++
				p.mu.Unlock()
			}
		}
	}()
	return nil
}
func poolWait(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
func (p *ComputePool) workerMessage(s string) { p.mu.Lock(); p.joinState = tail(s, 500); p.mu.Unlock() }
func (p *ComputePool) runPeer(parent context.Context, c *http.Client, v PoolInvite, j ImageJob, lease string) error {
	s := p.s
	s.mu.Lock()
	if s.closed || !s.ready || s.cancel != nil {
		s.mu.Unlock()
		return errors.New("local generator became busy")
	}
	cfg, cli := s.config, s.cli
	ctx, cancel := context.WithTimeout(parent, time.Duration(cfg.MaxMinutes)*time.Minute)
	s.cancel = cancel
	s.active = "worker-" + j.ID
	s.mu.Unlock()
	defer func() {
		cancel()
		s.mu.Lock()
		s.cancel = nil
		s.active = ""
		s.mu.Unlock()
		s.e.imageBusy.Store(false)
		s.kick()
	}()
	s.e.imageBusy.Store(true)
	var heartbeat sync.WaitGroup
	heartbeat.Add(1)
	go func() {
		defer heartbeat.Done()
		for poolWait(ctx, 10*time.Second) {
			p.mu.Lock()
			msg := p.joinState
			p.mu.Unlock()
			if _, err := poolPost(ctx, c, v, "/pool/heartbeat", map[string]any{"id": j.ID, "lease": lease, "message": msg}, nil); err != nil {
				cancel()
				return
			}
		}
	}()
	b, err := s.run(ctx, cli, cfg, j.Request, "worker-"+j.ID, func(line string) { p.workerMessage("Computing " + j.ID + ": " + tail(strings.TrimSpace(line), 300)) })
	if err == nil {
		_, err = poolPost(ctx, c, v, "/pool/result", b, map[string]string{"Content-Type": "image/png", "X-Job-ID": j.ID, "X-Job-Lease": lease})
	}
	cancel()
	heartbeat.Wait()
	return err
}
func (p *ComputePool) routes(mux *http.ServeMux) {
	mux.HandleFunc("/api/pool/state", func(w http.ResponseWriter, r *http.Request) { jsonReply(w, p.status()) })
	mux.HandleFunc("/api/pool/action", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			Action    string `json:"action"`
			Listen    string `json:"listen"`
			Advertise string `json:"advertise"`
			Invite    string `json:"invite"`
			Name      string `json:"name"`
			Budget    int    `json:"budget"`
		}
		if err := decode(r, &v); err != nil {
			apiError(w, err)
			return
		}
		var err error
		var result any
		switch v.Action {
		case "host":
			err = p.host(v.Listen, v.Advertise)
		case "invite":
			var invite string
			invite, err = p.invite()
			result = map[string]string{"invite": invite}
		case "stop":
			p.stopHost()
		case "join":
			err = p.join(v.Invite, v.Name, v.Budget)
		case "leave":
			p.leave()
		default:
			err = errors.New("unknown group action")
		}
		if err != nil {
			apiError(w, err)
			return
		}
		if result == nil {
			result = p.status()
		}
		jsonReply(w, result)
	})
}
