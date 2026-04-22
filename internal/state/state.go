// Package state manages ReviewIQ review state via GitHub PR comments.
package state

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Sanmanchekar/reviewiq/internal/github"
)

// ── Types ───────────────────────────────────────────────────────────────────

type PRMetadata struct {
	Number     int    `json:"number"`
	Repo       string `json:"repo"`
	Title      string `json:"title"`
	Author     string `json:"author"`
	BaseBranch string `json:"base_branch"`
	HeadBranch string `json:"head_branch"`
}

type ReviewRound struct {
	Round         int      `json:"round"`
	Timestamp     string   `json:"timestamp"`
	HeadSHA       string   `json:"head_sha"`
	BaseSHA       string   `json:"base_sha"`
	Event         string   `json:"event"`
	FilesReviewed []string `json:"files_reviewed"`
}

type StatusEntry struct {
	Status    string `json:"status"`
	Round     int    `json:"round"`
	Timestamp string `json:"timestamp"`
	Note      string `json:"note,omitempty"`
}

type DiscussionMsg struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

type Finding struct {
	ID            int             `json:"id"`
	Title         string          `json:"title"`
	Severity      string          `json:"severity"`
	Status        string          `json:"status"`
	File          string          `json:"file"`
	Line          int             `json:"line"`
	Problem       string          `json:"problem"`
	Impact        string          `json:"impact"`
	SuggestedFix  string          `json:"suggested_fix"`
	FixRationale  string          `json:"fix_rationale"`
	CreatedRound  int             `json:"created_round"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
	StatusHistory []StatusEntry   `json:"status_history"`
	Discussion    []DiscussionMsg `json:"discussion"`
}

type ConversationMsg struct {
	Role      string            `json:"role"`
	Content   string            `json:"content"`
	Round     int               `json:"round"`
	Timestamp string            `json:"timestamp"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type Summary struct {
	TotalFindings  int    `json:"total_findings"`
	Open           int    `json:"open"`
	Resolved       int    `json:"resolved"`
	Wontfix        int    `json:"wontfix"`
	Retracted      int    `json:"retracted"`
	Assessment     string `json:"assessment"`
	LastReviewedSHA string `json:"last_reviewed_sha"`
}

type ReviewState struct {
	Version      int                `json:"version"`
	PR           PRMetadata         `json:"pr"`
	ReviewRounds []ReviewRound      `json:"review_rounds"`
	Findings     map[string]Finding `json:"findings"`
	Conversation []ConversationMsg  `json:"conversation"`
	Summary      Summary            `json:"summary"`
}

type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ── Constructors ────────────────────────────────────────────────────────────

func NewState(prNumber int, repo string) *ReviewState {
	return &ReviewState{
		Version:      2,
		PR:           PRMetadata{Number: prNumber, Repo: repo},
		ReviewRounds: []ReviewRound{},
		Findings:     make(map[string]Finding),
		Conversation: []ConversationMsg{},
		Summary: Summary{
			Assessment: "PENDING",
		},
	}
}

func NewFinding(id int, title, severity, file string, line int, problem, impact, fix, rationale string, round int) Finding {
	now := nowUTC()
	return Finding{
		ID: id, Title: title, Severity: severity, Status: "open",
		File: file, Line: line, Problem: problem, Impact: impact,
		SuggestedFix: fix, FixRationale: rationale,
		CreatedRound: round, CreatedAt: now, UpdatedAt: now,
		StatusHistory: []StatusEntry{{Status: "open", Round: round, Timestamp: now}},
		Discussion:    []DiscussionMsg{},
	}
}

func NewReviewRound(round int, headSHA, baseSHA, event string, files []string) ReviewRound {
	return ReviewRound{
		Round: round, Timestamp: nowUTC(),
		HeadSHA: headSHA, BaseSHA: baseSHA,
		Event: event, FilesReviewed: files,
	}
}

// ── Finding Lifecycle ───────────────────────────────────────────────────────

var ValidStatuses = map[string]bool{
	"open": true, "resolved": true, "partially_fixed": true,
	"wontfix": true, "retracted": true,
}

func (s *ReviewState) TransitionFinding(findingID int, newStatus, note string, round int) error {
	if !ValidStatuses[newStatus] {
		return fmt.Errorf("invalid status: %s", newStatus)
	}
	fid := fmt.Sprintf("%d", findingID)
	f, ok := s.Findings[fid]
	if !ok {
		return fmt.Errorf("finding %d not found", findingID)
	}
	f.Status = newStatus
	f.UpdatedAt = nowUTC()
	f.StatusHistory = append(f.StatusHistory, StatusEntry{
		Status: newStatus, Round: round, Timestamp: nowUTC(), Note: note,
	})
	s.Findings[fid] = f
	s.RecomputeSummary()
	return nil
}

func (s *ReviewState) AddFindingDiscussion(findingID int, role, content string) error {
	fid := fmt.Sprintf("%d", findingID)
	f, ok := s.Findings[fid]
	if !ok {
		return fmt.Errorf("finding %d not found", findingID)
	}
	f.Discussion = append(f.Discussion, DiscussionMsg{
		Role: role, Content: content, Timestamp: nowUTC(),
	})
	s.Findings[fid] = f
	return nil
}

// ── Conversation ────────────────────────────────────────────────────────────

func (s *ReviewState) AddMessage(role, content string, round int, metadata map[string]string) {
	s.Conversation = append(s.Conversation, ConversationMsg{
		Role: role, Content: content, Round: round,
		Timestamp: nowUTC(), Metadata: metadata,
	})
}

func (s *ReviewState) GetConversationForLLM() []LLMMessage {
	var msgs []LLMMessage
	for _, m := range s.Conversation {
		switch m.Role {
		case "developer":
			msgs = append(msgs, LLMMessage{Role: "user", Content: m.Content})
		case "agent":
			msgs = append(msgs, LLMMessage{Role: "assistant", Content: m.Content})
		case "system":
			msgs = append(msgs, LLMMessage{Role: "user", Content: "[SYSTEM EVENT] " + m.Content})
		}
	}
	if len(msgs) == 0 {
		return msgs
	}
	// Merge consecutive same-role messages
	merged := []LLMMessage{msgs[0]}
	for _, m := range msgs[1:] {
		last := &merged[len(merged)-1]
		if m.Role == last.Role {
			last.Content += "\n\n" + m.Content
		} else {
			merged = append(merged, m)
		}
	}
	// First message must be user
	if merged[0].Role == "assistant" {
		merged = append([]LLMMessage{{Role: "user", Content: "[SYSTEM EVENT] Continuing previous review session."}}, merged...)
	}
	return merged
}

// ── Queries ─────────────────────────────────────────────────────────────────

func (s *ReviewState) OpenFindings() []Finding {
	var out []Finding
	for _, f := range s.Findings {
		if f.Status == "open" || f.Status == "partially_fixed" {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *ReviewState) FindingsByStatus(status string) []Finding {
	var out []Finding
	for _, f := range s.Findings {
		if f.Status == status {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *ReviewState) GetFinding(id int) *Finding {
	fid := fmt.Sprintf("%d", id)
	f, ok := s.Findings[fid]
	if !ok {
		return nil
	}
	return &f
}

func (s *ReviewState) LatestRound() *ReviewRound {
	if len(s.ReviewRounds) == 0 {
		return nil
	}
	return &s.ReviewRounds[len(s.ReviewRounds)-1]
}

func (s *ReviewState) SortedFindingIDs() []int {
	var ids []int
	for _, f := range s.Findings {
		ids = append(ids, f.ID)
	}
	sort.Ints(ids)
	return ids
}

func (s *ReviewState) StateSummaryText() string {
	var b strings.Builder
	sm := s.Summary
	fmt.Fprintf(&b, "## Review State (Round %d)\n", len(s.ReviewRounds))
	fmt.Fprintf(&b, "- Total findings: %d\n", sm.TotalFindings)
	fmt.Fprintf(&b, "- Open: %d\n", sm.Open)
	fmt.Fprintf(&b, "- Resolved: %d\n", sm.Resolved)
	fmt.Fprintf(&b, "- Won't fix: %d\n", sm.Wontfix)
	fmt.Fprintf(&b, "- Retracted: %d\n", sm.Retracted)
	fmt.Fprintf(&b, "- Assessment: %s\n", sm.Assessment)
	sha := sm.LastReviewedSHA
	if len(sha) > 8 {
		sha = sha[:8]
	}
	if sha == "" {
		sha = "none"
	}
	fmt.Fprintf(&b, "- Last reviewed SHA: %s\n", sha)
	b.WriteString("\n### Active Findings:\n")
	for _, f := range s.OpenFindings() {
		fmt.Fprintf(&b, "  - **Finding %d** [%s] (%s): %s — `%s:%d`\n",
			f.ID, f.Severity, f.Status, f.Title, f.File, f.Line)
	}
	resolved := s.FindingsByStatus("resolved")
	if len(resolved) > 0 {
		b.WriteString("\n### Resolved Findings:\n")
		for _, f := range resolved {
			note := ""
			for i := len(f.StatusHistory) - 1; i >= 0; i-- {
				if f.StatusHistory[i].Note != "" {
					note = " — " + f.StatusHistory[i].Note
					break
				}
			}
			fmt.Fprintf(&b, "  - ~~Finding %d~~ [%s]: %s%s\n", f.ID, f.Severity, f.Title, note)
		}
	}
	return b.String()
}

func (s *ReviewState) RecomputeSummary() {
	s.Summary.TotalFindings = len(s.Findings)
	s.Summary.Open = 0
	s.Summary.Resolved = 0
	s.Summary.Wontfix = 0
	s.Summary.Retracted = 0
	hasCritical := false
	for _, f := range s.Findings {
		switch f.Status {
		case "open", "partially_fixed":
			s.Summary.Open++
			if f.Severity == "CRITICAL" {
				hasCritical = true
			}
		case "resolved":
			s.Summary.Resolved++
		case "wontfix":
			s.Summary.Wontfix++
		case "retracted":
			s.Summary.Retracted++
		}
	}
	if s.Summary.Open == 0 && s.Summary.TotalFindings > 0 {
		s.Summary.Assessment = "APPROVE"
	} else if hasCritical || s.Summary.Open > 0 {
		s.Summary.Assessment = "REQUEST CHANGES"
	} else {
		s.Summary.Assessment = "PENDING"
	}
}

// ── GitHub Comment Backend ──────────────────────────────────────────────────

const (
	stateMarkerStart  = "<!-- REVIEWIQ_STATE_START -->"
	stateMarkerEnd    = "<!-- REVIEWIQ_STATE_END -->"
	stateCommentHeader = "<!-- REVIEWIQ_STATE_COMMENT -->"
)

func stateRoundMarker(round int) string {
	return fmt.Sprintf("<!-- REVIEWIQ_STATE_ROUND_%d -->", round)
}

func LoadRemote(repo string, prNumber int) (*ReviewState, error) {
	_, s, err := findLatestStateComment(repo, prNumber)
	return s, err
}

// SaveRemote always creates a NEW comment (never overwrites previous rounds).
// Each round's state is preserved as a separate hidden comment for audit trail.
func SaveRemote(s *ReviewState) error {
	encoded, err := encodeState(s)
	if err != nil {
		return err
	}
	sm := s.Summary
	round := len(s.ReviewRounds)
	body := fmt.Sprintf(`%s
%s
<details>
<summary>ReviewIQ State (Round %d) — %d open, %d resolved</summary>

| Metric | Count |
|--------|-------|
| Total findings | %d |
| Open | %d |
| Resolved | %d |
| Won't fix | %d |
| Retracted | %d |
| Assessment | %s |

</details>

%s
%s
%s`,
		stateCommentHeader, stateRoundMarker(round), round, sm.Open, sm.Resolved,
		sm.TotalFindings, sm.Open, sm.Resolved, sm.Wontfix, sm.Retracted, sm.Assessment,
		stateMarkerStart, encoded, stateMarkerEnd)

	// Always POST new comment — preserves all previous round states
	return ghAPI("POST", fmt.Sprintf("/repos/%s/issues/%d/comments", s.PR.Repo, s.PR.Number), map[string]string{"body": body})
}

// ── Load/Save (GitHub-only) ─────────────────────────────────────────────────

func Load(prNumber int, repo string) *ReviewState {
	if repo != "" {
		s, _ := LoadRemote(repo, prNumber)
		if s != nil {
			return s
		}
	}
	return NewState(prNumber, repo)
}

func Save(s *ReviewState) {
	if s.PR.Repo != "" {
		_ = SaveRemote(s)
	}
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

func encodeState(s *ReviewState) (string, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func hasGitHubAuth() bool {
	token, err := github.GetToken()
	return err == nil && token != ""
}

// findLatestStateComment finds all state comments and returns the one with the highest round number.
func findLatestStateComment(repo string, prNumber int) (int, *ReviewState, error) {
	token, err := github.GetToken()
	if err != nil {
		return 0, nil, nil
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d/comments", repo, prNumber)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var comments []struct {
		ID   int    `json:"id"`
		Body string `json:"body"`
	}
	if err := json.Unmarshal(body, &comments); err != nil {
		return 0, nil, nil
	}
	re := regexp.MustCompile(regexp.QuoteMeta(stateMarkerStart) + `\n(.+?)\n` + regexp.QuoteMeta(stateMarkerEnd))

	var bestID int
	var bestState *ReviewState
	bestRound := -1

	for _, c := range comments {
		if !strings.Contains(c.Body, stateCommentHeader) {
			continue
		}
		match := re.FindStringSubmatch(c.Body)
		if len(match) < 2 {
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(match[1]))
		if err != nil {
			continue
		}
		var s ReviewState
		if err := json.Unmarshal(decoded, &s); err != nil {
			continue
		}
		round := len(s.ReviewRounds)
		if round > bestRound {
			bestRound = round
			bestID = c.ID
			bestState = &s
		}
	}
	if bestState != nil {
		return bestID, bestState, nil
	}
	return 0, nil, nil
}

func ghAPI(method, endpoint string, data map[string]string) error {
	token, err := github.GetToken()
	if err != nil {
		return err
	}
	body, _ := json.Marshal(data)
	url := "https://api.github.com" + endpoint
	req, _ := http.NewRequest(method, url, strings.NewReader(string(body)))
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
