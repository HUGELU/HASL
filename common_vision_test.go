package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func civicTestCommand(t *testing.T, c *CommonVision, cmd CivicCommand) {
	t.Helper()
	cmd.Revision = c.state.Revision
	if err := c.mutate(cmd); err != nil {
		t.Fatal(cmd.Action, err)
	}
}
func civicTestReview(action, id string) CivicCommand {
	return CivicCommand{Action: action, ID: id, Reviewer: "Local human test reviewer", Reason: "Reviewed the exact demonstration scope; no external institutional authority claimed.", Human: true, Confirm: true}
}
func TestCivicCompleteCycleAndPersistence(t *testing.T) {
	base := t.TempDir()
	e := NewEngine(base)
	defer e.Stop()
	c := e.commonVision
	if len(c.state.Values) != 13 || !c.state.Values[12].PlusOne || c.state.Values[12].Original != "" {
		t.Fatal("original values were invented or +1 lost")
	}
	civicTestCommand(t, c, CivicCommand{Action: "demo"})
	if len(c.state.Proposals) != 1 || len(c.state.Contributions) != 4 {
		t.Fatal("missing demonstration")
	}
	for _, topic := range civicTopics(&c.state) {
		if topic.HumanRecords != 0 || topic.Support != 0 || topic.DemoRecords != 4 {
			t.Fatal("demonstration counted as public support")
		}
	}
	draft := c.state.Draft
	civicTestCommand(t, c, civicTestReview("adopt-draft", draft))
	adopted, _ := json.Marshal(civicCurrent(&c.state, c.state.Adopted))
	p := c.state.Proposals[0]
	early := CivicCommand{Action: "outcome", Revision: c.state.Revision, Outcome: &CivicOutcome{Proposal: p.ID, Kind: "simulation", Result: "result", Measure: "metric", Source: "fixture", Limitations: "synthetic", Next: "next", Demo: true}}
	if err := c.mutate(early); err == nil {
		t.Fatal("outcome accepted before experiment authorisation")
	}
	civicTestCommand(t, c, civicTestReview("approve-proposal", p.ID))
	civicTestCommand(t, c, civicTestReview("authorise-experiment", p.ID))
	civicTestCommand(t, c, early)
	o := c.state.Outcomes[0]
	civicTestCommand(t, c, CivicCommand{Action: "follow-up", ID: o.ID})
	if len(c.state.Proposals) != 2 || c.state.Proposals[1].Status != "proposed" || c.state.Proposals[1].Objective != "next" {
		t.Fatal("feedback did not create reviewable follow-up")
	}
	if !strings.Contains(civicCurrent(&c.state, c.state.Draft).Manuscript, o.ID) {
		t.Fatal("outcome provenance absent")
	}
	after, _ := json.Marshal(civicCurrent(&c.state, c.state.Adopted))
	if string(adopted) != string(after) {
		t.Fatal("adopted snapshot changed")
	}
	id := c.state.Draft
	civicTestCommand(t, c, civicTestReview("reject-draft", id))
	if c.state.Adopted != draft {
		t.Fatal("reject changed adopted vision")
	}
	e2 := NewEngine(base)
	defer e2.Stop()
	if e2.commonVision.state.Revision != c.state.Revision || e2.commonVision.state.Adopted != draft || len(e2.commonVision.state.Outcomes) != 1 {
		t.Fatal("persisted cycle failed to reload")
	}
}
func TestCivicBotsObjectionsAndStaleReview(t *testing.T) {
	e := NewEngine(t.TempDir())
	defer e.Stop()
	c := e.commonVision
	add := func(actor, kind, stance, text string) {
		civicTestCommand(t, c, CivicCommand{Action: "contribute", Contribution: &CivicContribution{Actor: actor, SourceType: kind, Kind: "opinion", Topic: "access", Text: text, Context: "test fixture", Language: "en", Stance: stance, Consent: true, Importance: 3, Urgency: 2, Consequence: 1}})
	}
	add("Alice", "human", "support", "Provide an offline route")
	add("Bob", "human", "object", "Privacy remains unresolved")
	stale := civicTestReview("adopt-draft", c.state.Draft)
	stale.Revision = c.state.Revision
	for i := 0; i < 20; i++ {
		add("bot", "automated", "support", strings.Repeat("analysis ", i+1))
	}
	add("alice", "human", "support", "The offline route still matters")
	topics := civicTopics(&c.state)
	if len(topics) != 1 || topics[0].HumanRecords != 2 || topics[0].AutomatedRecords != 1 || topics[0].Support != 1 || topics[0].Objections != 1 {
		t.Fatalf("invalid distinct participation: %+v", topics)
	}
	if err := c.mutate(stale); err == nil {
		t.Fatal("stale human review accepted")
	}
	before := c.state.Revision
	bad := CivicCommand{Action: "contribute", Revision: before, Contribution: &CivicContribution{Actor: "bot", SourceType: "human", Kind: "opinion", Topic: "access", Text: "changed identity", Language: "en", Context: "fixture", Stance: "support", Consent: true}}
	if err := c.mutate(bad); err == nil || c.state.Revision != before {
		t.Fatal("actor type change accepted or failed operation mutated state")
	}
	f := civicCurrent(&c.state, c.state.Draft)
	if !strings.Contains(f.Manuscript, "Privacy remains unresolved") {
		t.Fatal("minority objection lost")
	}
	if f.Vision != provisionalVision {
		t.Fatal("posting frequency changed central wording")
	}
	cmd := civicTestReview("adopt-draft", c.state.Draft)
	cmd.Revision = c.state.Revision
	cmd.Human = false
	if err := c.mutate(cmd); err == nil {
		t.Fatal("automated adoption accepted")
	}
}
func TestCivicSourceReviewAndFailureRetention(t *testing.T) {
	e := NewEngine(t.TempDir())
	defer e.Stop()
	c := e.commonVision
	cmd := civicTestReview("value-source", "")
	cmd.Value = &CivicValue{ID: "plus-one", Original: "test source wording", Source: "synthetic test document"}
	cmd.Revision = c.state.Revision
	if err := c.mutate(cmd); err == nil {
		t.Fatal("+1 source accepted without its meaning")
	}
	cmd.Value.Meaning = "Synthetic explanation, not the user's actual value"
	civicTestCommand(t, c, cmd)
	if c.state.Values[12].Original != cmd.Value.Original || c.state.Values[0].Original != "" {
		t.Fatal("source wording not preserved exactly")
	}
	bytesBefore, _ := os.ReadFile(c.path)
	cmd = CivicCommand{Action: "amend-vision", Text: "", Revision: c.state.Revision}
	if c.mutate(cmd) == nil {
		t.Fatal("empty amendment accepted")
	}
	bytesAfter, _ := os.ReadFile(c.path)
	if string(bytesBefore) != string(bytesAfter) {
		t.Fatal("failed change overwrote store")
	}
	path := filepath.Join(t.TempDir(), "origin0_data")
	_ = os.MkdirAll(path, 0700)
	_ = os.WriteFile(filepath.Join(path, "common-vision.json"), []byte("corrupt"), 0600)
	other := NewEngine(filepath.Dir(path))
	defer other.Stop()
	if other.commonVision.loadError == "" {
		t.Fatal("corrupt state silently replaced")
	}
	b, _ := os.ReadFile(filepath.Join(path, "common-vision.json"))
	if string(b) != "corrupt" {
		t.Fatal("corrupt original overwritten")
	}
}
