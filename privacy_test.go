package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPrivacyPINProtectsAPIsAndRecovers(t *testing.T) {
	e := studioEngine(t)
	p := e.privacy
	v, err := p.action("set", "730491", "", "")
	if err != nil {
		t.Fatal(err)
	}
	code := v["recovery"].(string)
	b, _ := os.ReadFile(p.path)
	if strings.Contains(string(b), "730491") || strings.Contains(string(b), code) {
		t.Fatal("cleartext credential persisted")
	}
	if _, err = p.action("lock", "", "", ""); err != nil {
		t.Fatal(err)
	}
	rr := studioRequest(e, "/api/concepts/state", map[string]any{})
	if rr.Code != http.StatusLocked {
		t.Fatal(rr.Code)
	}
	rr = studioRequest(e, "/api/privacy", map[string]string{"action": "state"})
	if rr.Code != 200 {
		t.Fatal(rr.Body.String())
	}
	var status map[string]any
	json.Unmarshal(rr.Body.Bytes(), &status)
	if status["locked"] != true {
		t.Fatal(status)
	}
	if _, err = p.action("unlock", "wrong", "", ""); err == nil {
		t.Fatal("wrong PIN accepted")
	}
	if _, err = p.action("unlock", "730491", "", ""); err == nil {
		t.Fatal("rate limit ignored")
	}
	p.mu.Lock()
	p.next = time.Time{}
	p.mu.Unlock()
	if _, err = p.action("unlock", "730491", "", ""); err != nil {
		t.Fatal(err)
	}
	if studioRequest(e, "/api/concepts/state", map[string]any{}).Code != 200 {
		t.Fatal("correct PIN did not unlock API")
	}
	restored := newPrivacy(e.dataDir)
	if !restored.isLocked() {
		t.Fatal("restart did not lock")
	}
	if _, err = restored.action("recover", "", "", code); err != nil {
		t.Fatal(err)
	}
	if restored.isLocked() {
		t.Fatal("recovery did not unlock")
	}
	if _, err = os.Stat(p.path); !os.IsNotExist(err) {
		t.Fatal("recovery did not remove PIN")
	}
}
func TestPrivacyCorruptSettingsFailClosed(t *testing.T) {
	e := studioEngine(t)
	os.WriteFile(e.privacy.path, []byte("corrupt"), 0600)
	p := newPrivacy(e.dataDir)
	if !p.isLocked() {
		t.Fatal("corrupt settings opened workspace")
	}
	if _, err := p.action("set", "123456", "", ""); err == nil {
		t.Fatal("silently overwrote damaged settings")
	}
}
