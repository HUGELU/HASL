package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const visionMethod = "transparent-topic-register-v1"
const provisionalVision = "Build a shared, revisable direction through evidence, disagreement and accountable experiments."

type CivicValue struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	PlusOne  bool   `json:"plus_one"`
	Status   string `json:"status"`
	Original string `json:"original"`
	Meaning  string `json:"meaning"`
	Source   string `json:"source"`
	Revision int    `json:"revision"`
}
type CivicContribution struct {
	ID           string   `json:"id"`
	Actor        string   `json:"actor"`
	SourceType   string   `json:"source_type"`
	Kind         string   `json:"kind"`
	Topic        string   `json:"topic"`
	Text         string   `json:"text"`
	Language     string   `json:"language"`
	Context      string   `json:"context"`
	Source       string   `json:"source"`
	Stance       string   `json:"stance"`
	Values       []string `json:"values"`
	Importance   int      `json:"importance"`
	Urgency      int      `json:"urgency"`
	Consequence  int      `json:"consequence"`
	Consent      bool     `json:"consent"`
	Demo         bool     `json:"demo"`
	Withdrawn    bool     `json:"withdrawn"`
	CorrectionOf string   `json:"correction_of,omitempty"`
	Created      int64    `json:"created"`
}
type CivicProposal struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Topic         string   `json:"topic"`
	Need          string   `json:"need"`
	Objective     string   `json:"objective"`
	Affected      string   `json:"affected"`
	Benefits      string   `json:"benefits"`
	Costs         string   `json:"costs"`
	Tradeoffs     string   `json:"tradeoffs"`
	Alternatives  string   `json:"alternatives"`
	Owner         string   `json:"owner"`
	Authority     string   `json:"authority"`
	Measure       string   `json:"measure"`
	Baseline      string   `json:"baseline"`
	Target        string   `json:"target"`
	StopRule      string   `json:"stop_rule"`
	Assumptions   string   `json:"assumptions"`
	Contributions []string `json:"contributions"`
	Values        []string `json:"values"`
	Status        string   `json:"status"`
	Demo          bool     `json:"demo"`
}
type CivicOutcome struct {
	ID          string `json:"id"`
	Proposal    string `json:"proposal"`
	Kind        string `json:"kind"`
	Result      string `json:"result"`
	Measure     string `json:"measure"`
	Source      string `json:"source"`
	Limitations string `json:"limitations"`
	Next        string `json:"next"`
	Demo        bool   `json:"demo"`
	Created     int64  `json:"created"`
}
type CivicFramework struct {
	ID         string       `json:"id"`
	Revision   int          `json:"revision"`
	Created    int64        `json:"created"`
	Method     string       `json:"method"`
	Vision     string       `json:"vision"`
	Manifesto  string       `json:"manifesto"`
	Manuscript string       `json:"manuscript"`
	Programme  string       `json:"programme"`
	Sources    []string     `json:"sources"`
	Values     []CivicValue `json:"values"`
	Scope      string       `json:"scope"`
	Demo       bool         `json:"demo"`
	Hash       string       `json:"hash"`
}
type CivicDecision struct {
	ID         string `json:"id"`
	Action     string `json:"action"`
	Target     string `json:"target"`
	TargetHash string `json:"target_hash"`
	Reviewer   string `json:"reviewer"`
	Reason     string `json:"reason"`
	Revision   int    `json:"revision"`
	Created    int64  `json:"created"`
}
type CivicState struct {
	Schema        string              `json:"schema"`
	Revision      int                 `json:"revision"`
	Values        []CivicValue        `json:"values"`
	Contributions []CivicContribution `json:"contributions"`
	Proposals     []CivicProposal     `json:"proposals"`
	Outcomes      []CivicOutcome      `json:"outcomes"`
	Frameworks    []CivicFramework    `json:"frameworks"`
	Decisions     []CivicDecision     `json:"decisions"`
	Draft         string              `json:"draft"`
	Adopted       string              `json:"adopted"`
	Gaps          string              `json:"gaps"`
	Scope         string              `json:"scope"`
}
type CivicTopic struct {
	Topic                 string  `json:"topic"`
	HumanRecords          int     `json:"human_records"`
	AutomatedRecords      int     `json:"automated_records"`
	OrganisationalRecords int     `json:"organisational_records"`
	DemoRecords           int     `json:"demo_records"`
	Support               int     `json:"support"`
	Objections            int     `json:"objections"`
	Priority              float64 `json:"priority"`
	Note                  string  `json:"note"`
}
type CommonVision struct {
	mu        sync.Mutex
	path      string
	state     CivicState
	loadError string
}
type CivicCommand struct {
	Revision     int                `json:"revision"`
	Action       string             `json:"action"`
	ID           string             `json:"id"`
	Text         string             `json:"text"`
	Reviewer     string             `json:"reviewer"`
	Reason       string             `json:"reason"`
	Human        bool               `json:"human"`
	Confirm      bool               `json:"confirm"`
	Contribution *CivicContribution `json:"contribution,omitempty"`
	Proposal     *CivicProposal     `json:"proposal,omitempty"`
	Outcome      *CivicOutcome      `json:"outcome,omitempty"`
	Value        *CivicValue        `json:"value,omitempty"`
}

func civicID(prefix string) string { return prefix + "-" + randomID()[:12] }
func civicNorm(s string) string    { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }
func civicHash(v any) string {
	b, _ := json.Marshal(v)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
func civicOne(v string, options ...string) bool {
	for _, x := range options {
		if v == x {
			return true
		}
	}
	return false
}
func civicRequired(v ...string) error {
	for _, s := range v {
		if strings.TrimSpace(s) == "" || len(s) > 8000 {
			return errors.New("complete every required field (maximum 8000 bytes each)")
		}
	}
	return nil
}
func civicLinks(values []CivicValue, ids []string) error {
	if len(ids) > 13 {
		return errors.New("at most 12 + 1 value links")
	}
	for _, id := range ids {
		found := false
		for _, v := range values {
			if v.ID == id {
				found = true
			}
		}
		if !found {
			return errors.New("unknown value link")
		}
	}
	return nil
}
func newCommonVision(e *Engine) *CommonVision {
	c := &CommonVision{path: filepath.Join(e.dataDir, "common-vision.json")}
	b, err := os.ReadFile(c.path)
	if err == nil {
		if json.Unmarshal(b, &c.state) != nil || c.state.Schema != "origin0.common-vision.v1" {
			c.loadError = "Common Vision storage could not be read. Original file retained; edits are disabled."
		}
		return c
	}
	if !os.IsNotExist(err) {
		c.loadError = err.Error()
		return c
	}
	c.state = CivicState{Schema: "origin0.common-vision.v1", Revision: 1, Scope: "Local prototype; participant identities and representativeness are unverified.", Gaps: "No population sampling frame. Missing perspectives have not been assessed. Original 12 + 1 wording, the +1 meaning, original manifesto and manuscript are awaiting source material."}
	for i := 1; i <= 12; i++ {
		c.state.Values = append(c.state.Values, CivicValue{ID: fmt.Sprintf("value-%02d", i), Label: fmt.Sprintf("Value %02d", i), Status: "awaiting-source"})
	}
	c.state.Values = append(c.state.Values, CivicValue{ID: "plus-one", Label: "Distinct +1 principle", PlusOne: true, Status: "awaiting-source"})
	civicDraft(&c.state, provisionalVision)
	b, _ = json.MarshalIndent(c.state, "", "  ")
	if err = atomicWrite(c.path, b); err != nil {
		c.loadError = err.Error()
	}
	return c
}
func civicTopics(s *CivicState) []CivicTopic {
	groups := map[string][]CivicContribution{}
	for _, c := range s.Contributions {
		if !c.Withdrawn {
			groups[c.Topic] = append(groups[c.Topic], c)
		}
	}
	var out []CivicTopic
	for topic, cs := range groups {
		t := CivicTopic{Topic: topic, Note: "Counts are distinct local actor IDs, not verified persons or a population vote. Latest stance per actor/topic is used. Priority is a declared judgement, not factual confidence."}
		latest := map[string]CivicContribution{}
		demo := map[string]bool{}
		for _, c := range cs {
			if c.Demo {
				demo[c.Actor] = true
				continue
			}
			latest[c.SourceType+"\x00"+c.Actor] = c
		}
		total := 0.0
		for _, c := range latest {
			switch c.SourceType {
			case "human":
				t.HumanRecords++
				if c.Stance == "support" {
					t.Support++
				}
				if c.Stance == "object" {
					t.Objections++
				}
				total += float64(c.Importance+c.Urgency+c.Consequence) / 3
			case "automated":
				t.AutomatedRecords++
			default:
				t.OrganisationalRecords++
			}
		}
		t.DemoRecords = len(demo)
		if t.HumanRecords > 0 {
			t.Priority = total / float64(t.HumanRecords)
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Priority == out[j].Priority {
			return out[i].Topic < out[j].Topic
		}
		return out[i].Priority > out[j].Priority
	})
	return out
}
func civicCurrent(s *CivicState, id string) *CivicFramework {
	for i := range s.Frameworks {
		if s.Frameworks[i].ID == id {
			return &s.Frameworks[i]
		}
	}
	return nil
}
func civicDraft(s *CivicState, wording string) {
	f := CivicFramework{ID: civicID("framework"), Revision: s.Revision, Created: now(), Method: visionMethod, Vision: wording, Scope: s.Scope, Values: append([]CivicValue{}, s.Values...), Sources: []string{}}
	var manifesto, manuscript, programme strings.Builder
	manifesto.WriteString("WORKING MANIFESTO — generated register, not the missing original manifesto.\n\nProject commitments from the user's current brief: invite contributions, expose disagreements, separate evidence from value judgments, and require human review before adoption or consequential action.\n\nReview priorities:\n")
	manuscript.WriteString("WORKING MANUSCRIPT — a traceable prototype register, not the original full manuscript.\n\nMethod: " + visionMethod + ". Topics are supplied by contributors and normalised for whitespace/case. The software does not infer semantic agreement. Equal weights apply to declared importance, urgency and consequence (0–3 each). Per-topic priority is the mean of each distinct human actor ID's latest mean score. Automated, organisational and demonstration records do not add human support. No factual confidence is derived from votes.\n\nParticipation scope: " + s.Scope + "\nParticipation gaps: " + s.Gaps + "\n\n")
	for _, t := range civicTopics(s) {
		fmt.Fprintf(&manifesto, "- %s: priority %.2f/3; %d local human IDs, %d objections, %d automated IDs, %d organisational IDs, %d demonstration IDs.\n", t.Topic, t.Priority, t.HumanRecords, t.Objections, t.AutomatedRecords, t.OrganisationalRecords, t.DemoRecords)
	}
	if len(s.Contributions) == 0 {
		manifesto.WriteString("- Awaiting contributions. No common ground has been established.\n")
	}
	manuscript.WriteString("Foundational links:\n")
	for _, v := range s.Values {
		fmt.Fprintf(&manuscript, "[%s] %s — %s; source revision %d\n", v.ID, v.Label, v.Status, v.Revision)
		if v.Original != "" {
			fmt.Fprintf(&manuscript, "Exact supplied wording: %s\nSource: %s\nMeaning supplied: %s\n", v.Original, v.Source, v.Meaning)
		}
	}
	manuscript.WriteString("\nContribution register (original language retained; no automatic translation):\n")
	for _, c := range s.Contributions {
		if c.Withdrawn {
			continue
		}
		f.Sources = append(f.Sources, c.ID)
		f.Demo = f.Demo || c.Demo
		label := "local contribution"
		if c.Demo {
			label = "DEMONSTRATION"
		}
		fmt.Fprintf(&manuscript, "\n[%s] %s | %s | %s | %s | %s | %s\n%s\nSource/context: %s / %s\nValue links: %s\n", c.ID, label, c.SourceType, c.Kind, c.Stance, c.Topic, c.Language, c.Text, c.Source, c.Context, strings.Join(c.Values, ", "))
		if c.Kind == "evidence" {
			manuscript.WriteString("Evidence status: submitted claim/source, not independently verified.\n")
		}
	}
	programme.WriteString("IMPLEMENTATION PROGRAMME — proposals and recorded local decisions; no external actions are executed.\n\n")
	for _, p := range s.Proposals {
		f.Sources = append(f.Sources, p.ID)
		f.Demo = f.Demo || p.Demo
		fmt.Fprintf(&programme, "[%s] %s — %s (demonstration: %t)\nNeed: %s\nObjective: %s\nAffected: %s\nBenefits: %s\nCosts: %s\nTrade-offs: %s\nAlternatives including status quo: %s\nOwner: %s\nAuthority: %s\nMeasure: %s\nBaseline: %s\nTarget: %s\nStop/revise: %s\nAssumptions: %s\nContribution links: %s\nValue links: %s\n\n", p.ID, p.Title, p.Status, p.Demo, p.Need, p.Objective, p.Affected, p.Benefits, p.Costs, p.Tradeoffs, p.Alternatives, p.Owner, p.Authority, p.Measure, p.Baseline, p.Target, p.StopRule, p.Assumptions, strings.Join(p.Contributions, ", "), strings.Join(p.Values, ", "))
	}
	for _, o := range s.Outcomes {
		f.Sources = append(f.Sources, o.ID)
		f.Demo = f.Demo || o.Demo
		fmt.Fprintf(&manuscript, "\nOutcome [%s] → proposal [%s], %s (demonstration: %t)\nResult: %s\nMeasure: %s\nSource: %s\nLimitations: %s\nRequested next change: %s\nThis record alone does not establish causality or a proven policy benefit.\n", o.ID, o.Proposal, o.Kind, o.Demo, o.Result, o.Measure, o.Source, o.Limitations, o.Next)
	}
	f.Manifesto = manifesto.String()
	f.Manuscript = manuscript.String()
	f.Programme = programme.String()
	f.Hash = civicHash(f)
	s.Frameworks = append(s.Frameworks, f)
	s.Draft = f.ID
}
func civicReview(c CivicCommand) error {
	if !c.Human || !c.Confirm {
		return errors.New("a confirmed local human review is required")
	}
	return civicRequired(c.Reviewer, c.Reason)
}
func civicDecision(s *CivicState, c CivicCommand, target, hash string) {
	s.Decisions = append(s.Decisions, CivicDecision{ID: civicID("decision"), Action: c.Action, Target: target, TargetHash: hash, Reviewer: c.Reviewer, Reason: c.Reason, Revision: s.Revision, Created: now()})
}
func civicContributionValid(s *CivicState, c CivicContribution) error {
	if err := civicRequired(c.Actor, c.Topic, c.Text, c.Language, c.Context); err != nil {
		return err
	}
	if len(c.Actor) > 100 || len(c.Topic) > 150 || len(c.Language) > 40 {
		return errors.New("actor, topic or language label is too long")
	}
	if !c.Consent {
		return errors.New("confirm permission for local storage and analysis")
	}
	if !civicOne(c.SourceType, "human", "community", "organisation", "expert", "automated") || !civicOne(c.Kind, "need", "opinion", "evidence", "objection", "solution") || !civicOne(c.Stance, "support", "object", "neutral") {
		return errors.New("invalid contribution type or stance")
	}
	if c.Kind == "evidence" && strings.TrimSpace(c.Source) == "" {
		return errors.New("evidence needs a source reference; it remains unverified")
	}
	for _, n := range []int{c.Importance, c.Urgency, c.Consequence} {
		if n < 0 || n > 3 {
			return errors.New("priority inputs must be 0–3")
		}
	}
	return civicLinks(s.Values, c.Values)
}
func civicAdd(s *CivicState, c CivicContribution) error {
	c.Actor = civicNorm(c.Actor)
	c.Topic = civicNorm(c.Topic)
	if err := civicContributionValid(s, c); err != nil {
		return err
	}
	for _, old := range s.Contributions {
		if old.Actor == c.Actor && old.SourceType != c.SourceType {
			return errors.New("an actor ID cannot change source type; use its established identity")
		}
		if !old.Withdrawn && old.Actor == c.Actor && old.Topic == c.Topic && old.Kind == c.Kind && civicNorm(old.Text) == civicNorm(c.Text) && old.Stance == c.Stance && old.Demo == c.Demo {
			return errors.New("duplicate contribution; no additional participation counted")
		}
	}
	c.ID = civicID("contribution")
	c.Created = now()
	c.Withdrawn = false
	s.Contributions = append(s.Contributions, c)
	return nil
}
func civicProposalValid(s *CivicState, p CivicProposal) error {
	if err := civicRequired(p.Title, p.Topic, p.Need, p.Objective, p.Affected, p.Benefits, p.Costs, p.Tradeoffs, p.Alternatives, p.Owner, p.Authority, p.Measure, p.Baseline, p.Target, p.StopRule, p.Assumptions); err != nil {
		return err
	}
	if len(p.Contributions) == 0 {
		return errors.New("link at least one contribution")
	}
	for _, id := range p.Contributions {
		found := false
		for _, c := range s.Contributions {
			if c.ID == id && !c.Withdrawn {
				found = true
				if c.Demo && !p.Demo {
					return errors.New("a proposal using demonstration contributions must remain a demonstration")
				}
			}
		}
		if !found {
			return errors.New("proposal links an unavailable contribution")
		}
	}
	return civicLinks(s.Values, p.Values)
}
func (c *CommonVision) mutate(cmd CivicCommand) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loadError != "" {
		return errors.New(c.loadError)
	}
	if cmd.Revision != c.state.Revision {
		return errors.New("the workspace changed; refresh and review the latest revision before saving")
	}
	if c.state.Revision >= 500 || len(c.state.Contributions) >= 500 || len(c.state.Frameworks) >= 400 {
		return errors.New("prototype history limit reached; export the workspace before starting another")
	}
	b, _ := json.Marshal(c.state)
	var s CivicState
	_ = json.Unmarshal(b, &s)
	s.Revision++
	wording := provisionalVision
	if f := civicCurrent(&s, s.Draft); f != nil {
		wording = f.Vision
	}
	makeDraft := true
	switch cmd.Action {
	case "contribute":
		if cmd.Contribution == nil {
			return errors.New("contribution required")
		}
		if err := civicAdd(&s, *cmd.Contribution); err != nil {
			return err
		}
	case "withdraw":
		if err := civicReview(cmd); err != nil {
			return err
		}
		found := false
		for i := range s.Contributions {
			if s.Contributions[i].ID == cmd.ID {
				s.Contributions[i].Withdrawn = true
				found = true
			}
		}
		if !found {
			return errors.New("contribution not found")
		}
		civicDecision(&s, cmd, cmd.ID, "")
	case "amend-vision":
		if err := civicRequired(cmd.Text); err != nil {
			return err
		}
		if len(cmd.Text) > 600 {
			return errors.New("keep the central vision under 600 bytes")
		}
		wording = cmd.Text
	case "scope":
		if err := civicRequired(cmd.Text, cmd.Reason); err != nil {
			return err
		}
		s.Scope = cmd.Text
		s.Gaps = cmd.Reason
	case "value-source":
		if err := civicReview(cmd); err != nil {
			return err
		}
		if cmd.Value == nil {
			return errors.New("source value required")
		}
		v := cmd.Value
		if err := civicRequired(v.Original, v.Source); err != nil {
			return err
		}
		found := false
		for i := range s.Values {
			if s.Values[i].ID == v.ID {
				if s.Values[i].PlusOne && strings.TrimSpace(v.Meaning) == "" {
					return errors.New("the distinct +1 meaning must be supplied explicitly")
				}
				s.Values[i].Original = v.Original
				s.Values[i].Source = v.Source
				s.Values[i].Meaning = v.Meaning
				s.Values[i].Revision = s.Revision
				s.Values[i].Status = "supplied-source; human-confirmed locally"
				found = true
			}
		}
		if !found {
			return errors.New("unknown value slot")
		}
		civicDecision(&s, cmd, v.ID, civicHash(v))
	case "adopt-draft", "reject-draft":
		if err := civicReview(cmd); err != nil {
			return err
		}
		f := civicCurrent(&s, cmd.ID)
		if f == nil || cmd.ID != s.Draft || f.Revision != c.state.Revision {
			return errors.New("only the current reviewed draft can be decided")
		}
		if cmd.Action == "adopt-draft" {
			s.Adopted = f.ID
		}
		civicDecision(&s, cmd, f.ID, f.Hash)
		makeDraft = false
	case "proposal":
		if cmd.Proposal == nil {
			return errors.New("proposal required")
		}
		p := *cmd.Proposal
		if err := civicProposalValid(&s, p); err != nil {
			return err
		}
		p.ID = civicID("proposal")
		p.Status = "proposed"
		s.Proposals = append(s.Proposals, p)
	case "approve-proposal", "reject-proposal", "authorise-experiment":
		if err := civicReview(cmd); err != nil {
			return err
		}
		found := false
		for i := range s.Proposals {
			p := &s.Proposals[i]
			if p.ID != cmd.ID {
				continue
			}
			found = true
			if cmd.Action == "authorise-experiment" {
				if p.Status != "reviewed" {
					return errors.New("review this proposal before recording experiment authorisation")
				}
				p.Status = "authorised"
			} else {
				if p.Status != "proposed" {
					return errors.New("only a proposed version can be reviewed")
				}
				if cmd.Action == "approve-proposal" {
					p.Status = "reviewed"
				} else {
					p.Status = "rejected"
				}
			}
			civicDecision(&s, cmd, p.ID, civicHash(p))
		}
		if !found {
			return errors.New("proposal not found")
		}
	case "outcome":
		if cmd.Outcome == nil {
			return errors.New("outcome required")
		}
		o := *cmd.Outcome
		if err := civicRequired(o.Result, o.Measure, o.Source, o.Limitations, o.Next); err != nil {
			return err
		}
		if !civicOne(o.Kind, "simulation", "observation") {
			return errors.New("choose simulation or reported observation")
		}
		found := false
		for i := range s.Proposals {
			p := &s.Proposals[i]
			if p.ID == o.Proposal {
				if !civicOne(p.Status, "authorised", "measured") {
					return errors.New("record human experiment authorisation before outcomes")
				}
				if p.Demo && !o.Demo {
					return errors.New("demonstration outcomes must retain their label")
				}
				if o.Demo && o.Kind != "simulation" {
					return errors.New("demonstration data cannot be presented as observations")
				}
				p.Status = "measured"
				found = true
			}
		}
		if !found {
			return errors.New("proposal not found")
		}
		o.ID = civicID("outcome")
		o.Created = now()
		s.Outcomes = append(s.Outcomes, o)
	case "follow-up":
		found := false
		for _, o := range s.Outcomes {
			if o.ID != cmd.ID {
				continue
			}
			for _, p := range s.Proposals {
				if p.ID == o.Proposal {
					p.ID = civicID("proposal")
					p.Title += " — follow-up"
					p.Objective = o.Next
					p.Assumptions += "\nFollow-up source: " + o.ID + ". " + o.Limitations
					p.Status = "proposed"
					s.Proposals = append(s.Proposals, p)
					found = true
					break
				}
			}
			break
		}
		if !found {
			return errors.New("outcome not found")
		}
	case "demo":
		for _, p := range s.Proposals {
			if p.Demo {
				return errors.New("demonstration already loaded")
			}
		}
		civicDemo(&s)
	default:
		return errors.New("unknown Common Vision action")
	}
	if makeDraft {
		civicDraft(&s, wording)
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if len(b) > 16<<20 {
		return errors.New("prototype store limit reached; existing state retained")
	}
	if err = atomicWrite(c.path, b); err != nil {
		return err
	}
	c.state = s
	return nil
}
func civicDemo(s *CivicState) {
	rows := []CivicContribution{
		{Actor: "demo-resident-a", SourceType: "human", Kind: "need", Topic: "public decision accountability", Text: "I need to see what happened to proposals submitted to the council.", Language: "en", Context: "Synthetic resident example", Stance: "support", Importance: 3, Urgency: 2, Consequence: 2, Consent: true, Demo: true},
		{Actor: "demo-resident-b", SourceType: "human", Kind: "objection", Topic: "public decision accountability", Text: "A digital-only system could exclude people without internet access and expose personal complaints.", Language: "en", Context: "Synthetic minority objection", Stance: "object", Importance: 3, Urgency: 2, Consequence: 3, Consent: true, Demo: true},
		{Actor: "demo-staff-c", SourceType: "human", Kind: "opinion", Topic: "public decision accountability", Text: "Staff need a workload limit and a clear responsibility for responding.", Language: "en", Context: "Synthetic staff perspective", Stance: "neutral", Importance: 2, Urgency: 2, Consequence: 2, Consent: true, Demo: true},
		{Actor: "demo-analysis-bot", SourceType: "automated", Kind: "solution", Topic: "public decision accountability", Text: "Compare a response ledger, a citizens' assembly, and the current process. These are proposals to test, not proven improvements.", Language: "en", Context: "Synthetic automated analysis", Stance: "support", Importance: 3, Urgency: 3, Consequence: 3, Consent: true, Demo: true},
	}
	var ids []string
	for _, r := range rows {
		_ = civicAdd(s, r)
		ids = append(ids, s.Contributions[len(s.Contributions)-1].ID)
	}
	s.Proposals = append(s.Proposals, CivicProposal{ID: civicID("proposal"), Title: "Pilot a public decision response ledger", Topic: "public decision accountability", Need: "Trace submitted needs and explain institutional responses.", Objective: "Test whether a response register makes follow-up understandable without excluding offline participants.", Affected: "Residents, offline participants, service staff and people making sensitive complaints.", Benefits: "Hypothesis: clearer responsibility and fewer unacknowledged proposals.", Costs: "Illustrative budget: 20 staff hours; actual cost is unknown until measured.", Tradeoffs: "Transparency versus privacy; response speed versus staff workload; digital access versus exclusion.", Alternatives: "Status quo with no new register; a small paper/telephone register; a deliberative citizens' assembly with published institutional responses.", Owner: "Demonstration pilot coordinator", Authority: "Local project-owner review permits a simulation only. Any municipal activity needs the competent institution's separate authority.", Measure: "Share of sample submissions receiving a clear response within 30 days; include offline access and privacy incidents.", Baseline: "Synthetic baseline 40%; not a measured real service.", Target: "Illustrative target 60%, with no privacy incidents and no offline access decline.", StopRule: "Stop if sensitive information becomes visible or staff workload exceeds the agreed cap.", Assumptions: "Participants are synthetic. No causal evidence or community mandate exists. Foundational value links await the original 12 + 1 sources.", Contributions: ids, Status: "proposed", Demo: true})
}
func (e *Engine) commonVisionRoutes(mux *http.ServeMux) {
	c := e.commonVision
	mux.HandleFunc("/api/vision/state", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Since int `json:"since"`
		}
		if err := decode(r, &req); err != nil {
			apiError(w, err)
			return
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		if req.Since == c.state.Revision && c.loadError == "" {
			jsonReply(w, map[string]any{"unchanged": true})
			return
		}
		gaps := []map[string]string{{"title": "Original framework", "need": "Supply the exact 12 + 1 wording, the distinctive +1 explanation, original manifesto and manuscript.", "type": "human-source"}, {"title": "Participation coverage", "need": "Define whose participation is missing and obtain authorised contributions; current actor IDs are not proof of unique people.", "type": "human-research"}, {"title": "Interpretation method", "need": "Test multilingual interpretation against contributor corrections and held-out examples.", "type": "technical", "goal": "Propose a bounded, tested improvement to source processing efficiency while retaining exact original inputs; do not alter foundational values or human review rules."}}
		jsonReply(w, map[string]any{"state": c.state, "topics": civicTopics(&c.state), "load_error": c.loadError, "capability_gaps": gaps, "method": visionMethod, "resource_limits": "500 contributions/revisions and a 16 MiB state cap; no GPU or model download required. Export before reaching these prototype limits."})
	})
	mux.HandleFunc("/api/vision/action", func(w http.ResponseWriter, r *http.Request) {
		var cmd CivicCommand
		if err := decode(r, &cmd); err != nil {
			apiError(w, err)
			return
		}
		if err := c.mutate(cmd); err != nil {
			apiError(w, err)
			return
		}
		jsonReply(w, map[string]bool{"saved": true})
	})
	mux.HandleFunc("/api/vision/export", func(w http.ResponseWriter, r *http.Request) {
		if err := decode(r, &struct{}{}); err != nil {
			apiError(w, err)
			return
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.loadError != "" {
			apiError(w, errors.New(c.loadError))
			return
		}
		w.Header().Set("Content-Disposition", "attachment; filename=ORIGIN0_COMMON_VISION.json")
		jsonReply(w, c.state)
	})
}
