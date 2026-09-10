package main

import (
	"crypto/pbkdf2"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type PrivacyConfig struct {
	Salt     string `json:"salt"`
	PIN      string `json:"pin_hash"`
	Recovery string `json:"recovery_hash"`
}
type PrivacyLock struct {
	mu       sync.Mutex
	config   PrivacyConfig
	locked   bool
	failures int
	next     time.Time
	problem  string
	path     string
}

func newPrivacy(dir string) *PrivacyLock {
	p := &PrivacyLock{path: filepath.Join(dir, "privacy.json")}
	b, err := os.ReadFile(p.path)
	if err == nil {
		if json.Unmarshal(b, &p.config) != nil || len(p.config.Salt) != 48 || len(p.config.PIN) != 64 || len(p.config.Recovery) != 64 {
			p.problem = "Privacy settings could not be read. Restore privacy.json from your own backup."
		}
		p.locked = true
	} else if !os.IsNotExist(err) {
		p.locked = true
		p.problem = "Privacy settings are inaccessible."
	}
	return p
}
func validPIN(pin string) bool {
	if len(pin) < 6 || len(pin) > 12 {
		return false
	}
	for _, c := range pin {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
func pinHash(pin, salt string) string {
	b, _ := pbkdf2.Key(sha256.New, pin, []byte(salt), 600000, 32)
	return hex.EncodeToString(b)
}
func recoveryHash(code, salt string) string {
	h := sha256.Sum256([]byte("origin0-recovery|" + salt + "|" + strings.TrimSpace(code)))
	return hex.EncodeToString(h[:])
}
func equalSecret(a, b string) bool    { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }
func (p *PrivacyLock) isLocked() bool { p.mu.Lock(); defer p.mu.Unlock(); return p.locked }
func (p *PrivacyLock) failed() error {
	p.failures++
	delay := minInt(60, 1<<minInt(6, p.failures-1))
	p.next = time.Now().Add(time.Duration(delay) * time.Second)
	return errors.New("Incorrect code. Wait before trying again.")
}
func (p *PrivacyLock) action(action, pin, current, recovery string) (map[string]any, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if action == "state" {
		return map[string]any{"enabled": p.config.PIN != "" || p.problem != "", "locked": p.locked, "problem": p.problem}, nil
	}
	if p.problem != "" {
		return nil, errors.New(p.problem)
	}
	switch action {
	case "lock":
		if p.config.PIN == "" {
			return nil, errors.New("Set a PIN in Settings first")
		}
		p.locked = true
	case "unlock", "recover":
		if p.config.PIN == "" {
			return nil, errors.New("No PIN is configured")
		}
		if time.Now().Before(p.next) {
			return nil, errors.New("Wait before trying another code")
		}
		if action == "unlock" {
			if !validPIN(pin) || !equalSecret(pinHash(pin, p.config.Salt), p.config.PIN) {
				return nil, p.failed()
			}
		} else {
			if len(recovery) > 200 || !equalSecret(recoveryHash(recovery, p.config.Salt), p.config.Recovery) {
				return nil, p.failed()
			}
			if err := os.Remove(p.path); err != nil {
				return nil, err
			}
			p.config = PrivacyConfig{}
		}
		p.locked = false
		p.failures = 0
		p.next = time.Time{}
	case "set":
		if p.locked {
			return nil, errors.New("Unlock before changing the PIN")
		}
		if time.Now().Before(p.next) {
			return nil, errors.New("Wait before trying another code")
		}
		if p.config.PIN != "" && (!validPIN(current) || !equalSecret(pinHash(current, p.config.Salt), p.config.PIN)) {
			return nil, p.failed()
		}
		if !validPIN(pin) {
			return nil, errors.New("Use a PIN containing 6 to 12 digits")
		}
		salt := randomID()
		code := randomID()
		cfg := PrivacyConfig{Salt: salt, PIN: pinHash(pin, salt), Recovery: recoveryHash(code, salt)}
		b, _ := json.Marshal(cfg)
		if err := atomicWrite(p.path, b); err != nil {
			return nil, err
		}
		p.config = cfg
		p.failures = 0
		p.next = time.Time{}
		return map[string]any{"recovery": code, "enabled": true, "locked": false}, nil
	default:
		return nil, errors.New("Unknown privacy action")
	}
	return map[string]any{"enabled": p.config.PIN != "", "locked": p.locked}, nil
}
func (e *Engine) privacyRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/privacy", func(w http.ResponseWriter, r *http.Request) {
		var q struct{ Action, PIN, Current, Recovery string }
		if err := decode(r, &q); err != nil {
			apiError(w, err)
			return
		}
		v, err := e.privacy.action(q.Action, q.PIN, q.Current, q.Recovery)
		if err != nil {
			apiError(w, err)
			return
		}
		jsonReply(w, v)
	})
}
