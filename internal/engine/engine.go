// Package engine provides the core review logic — Claude API calls,
// structured output parsing, and state updates.
package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Sanmanchekar/reviewiq/internal/skills"
	"github.com/Sanmanchekar/reviewiq/internal/state"
)

const (
	DefaultModel    = "claude-sonnet-4-6-20250514"
	DefaultMaxTokens = 8192
	APIURL          = "https://api.anthropic.com/v1/messages"
)

func Log(msg string) { fmt.Fprintf(os.Stderr, "[reviewiq] %s\n", msg) }

// ── Claude API ──────────────────────────────────────────────────────────────

func CallClaude(systemPrompt string, messages []state.LLMMessage) (string, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY not set. Export it:\n  export ANTHROPIC_API_KEY=sk-ant-...")
	}
	model := os.Getenv("MODEL")
	if model == "" {
		model = DefaultModel
	}
	maxTokens := DefaultMaxTokens

	payload := map[string]interface{}{
		"model":      model,
		"max_tokens": maxTokens,
		"system":     systemPrompt,
		"messages":   messages,
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", APIURL, strings.NewReader(string(body)))
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API call failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 500)]))
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}
	if len(result.Content) == 0 {
		return "", fmt.Errorf("empty response from Claude")
	}
	return result.Content[0].Text, nil
}

// ── System Prompt ───────────────────────────────────────────────────────────

func ReadSystemPrompt(changedFiles []string, fileContents string) string {
	data, err := os.ReadFile(".pr-review/agent.md")
	var base string
	if err == nil {
		base = string(data)
	} else {
		base = "You are a PR review agent. Provide thorough, actionable code reviews with concrete fixes."
	}

	if len(changedFiles) > 0 {
		detected := skills.Detect(changedFiles, fileContents)
		skillPrompt := skills.LoadSkills(detected)
		if skillPrompt != "" {
			var loaded []string
			loaded = append(loaded, detected.Always...)
			loaded = append(loaded, detected.Languages...)
			loaded = append(loaded, detected.Frameworks...)
			loaded = append(loaded, detected.DevOps...)
			loaded = append(loaded, detected.Domains...)
			Log("Skills loaded: " + strings.Join(loaded, ", "))
			base += "\n\n" + skillPrompt
		}
	}
	return base
}

const StructuredOutputInstruction = `

## IMPORTANT: Structured Output for State Tracking

After your human-readable review, you MUST append a JSON block that I will parse to update the review state.
Wrap it in markers exactly like this:

<!-- REVIEWIQ_FINDINGS_START -->
` + "```json" + `
{
  "findings": [
    {
      "id": 1,
      "title": "Short title",
      "severity": "CRITICAL|IMPORTANT|NIT|QUESTION",
      "status": "open",
      "file": "path/to/file.ext",
      "line": 42,
      "problem": "What's wrong",
      "impact": "What breaks",
      "suggested_fix": "code fix here",
      "fix_rationale": "Why this approach"
    }
  ],
  "status_updates": [
    {
      "id": 1,
      "new_status": "resolved|partially_fixed|wontfix|retracted",
      "note": "Why the status changed"
    }
  ],
  "assessment": "APPROVE|REQUEST CHANGES|NEEDS DISCUSSION"
}
` + "```" + `
<!-- REVIEWIQ_FINDINGS_END -->

Rules for the JSON block:
- On initial review: populate "findings" array, leave "status_updates" empty
- On incremental review (check): populate "status_updates" for existing findings, add new findings if any
- On explain/fix/other commands: only include "status_updates" if a finding's status changed
- Finding IDs must be sequential integers starting from the highest existing ID + 1 for new findings
- Always include the "assessment" field
`

// ── Response Parsing ────────────────────────────────────────────────────────

var findingsPattern = regexp.MustCompile(`(?s)<!-- REVIEWIQ_FINDINGS_START -->\s*` + "```json" + `\s*(\{.*?\})\s*` + "```" + `\s*<!-- REVIEWIQ_FINDINGS_END -->`)

type structuredOutput struct {
	Findings []struct {
		ID           int    `json:"id"`
		Title        string `json:"title"`
		Severity     string `json:"severity"`
		Status       string `json:"status"`
		File         string `json:"file"`
		Line         int    `json:"line"`
		Problem      string `json:"problem"`
		Impact       string `json:"impact"`
		SuggestedFix string `json:"suggested_fix"`
		FixRationale string `json:"fix_rationale"`
	} `json:"findings"`
	StatusUpdates []struct {
		ID        int    `json:"id"`
		NewStatus string `json:"new_status"`
		Note      string `json:"note"`
	} `json:"status_updates"`
	Assessment string `json:"assessment"`
}

func ParseStructuredOutput(response string, s *state.ReviewState, round int) string {
	match := findingsPattern.FindStringSubmatchIndex(response)
	if match == nil {
		Log("No structured output found — state not updated from response")
		return response
	}

	jsonStr := response[match[2]:match[3]]
	var data structuredOutput
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		Log("Failed to parse structured output: " + err.Error())
		return response
	}

	for _, fd := range data.Findings {
		fid := fmt.Sprintf("%d", fd.ID)
		if _, exists := s.Findings[fid]; !exists {
			s.Findings[fid] = state.NewFinding(
				fd.ID, fd.Title, fd.Severity, fd.File, fd.Line,
				fd.Problem, fd.Impact, fd.SuggestedFix, fd.FixRationale, round,
			)
		}
	}
	for _, u := range data.StatusUpdates {
		if err := s.TransitionFinding(u.ID, u.NewStatus, u.Note, round); err != nil {
			Log(fmt.Sprintf("Failed to update finding %d: %s", u.ID, err))
		}
	}
	if data.Assessment != "" {
		s.Summary.Assessment = data.Assessment
	}
	s.RecomputeSummary()

	human := strings.TrimSpace(response[:match[0]])
	trailing := strings.TrimSpace(response[match[1]:])
	if trailing != "" {
		human += "\n\n" + trailing
	}
	return human
}

// ── Review Operations ───────────────────────────────────────────────────────

func RunReview(s *state.ReviewState, diff, fileContents, history string, changedFiles []string, headSHA, baseSHA, prTitle, prAuthor, prBody, baseBranch, headBranch string) (string, error) {
	round := len(s.ReviewRounds) + 1
	s.ReviewRounds = append(s.ReviewRounds, state.NewReviewRound(round, headSHA, baseSHA, "review", changedFiles))
	s.Summary.LastReviewedSHA = headSHA

	userContent := fmt.Sprintf(`Review this pull request.

## PR Metadata
- **Title**: %s
- **Author**: %s
- **Branch**: %s -> %s
- **Description**: %s

## Diff
`+"```diff"+`
%s
`+"```"+`

## Full File Contents
%s

## Recent Commit History
%s

This is review round %d. Run the full 'review' command as defined in your protocol.`,
		prTitle, prAuthor, headBranch, baseBranch, prBody, diff, fileContents, history, round)

	s.AddMessage("system", fmt.Sprintf("Review round %d.", round), round, nil)
	s.AddMessage("developer", userContent, round, nil)

	sysPrompt := ReadSystemPrompt(changedFiles, fileContents) + StructuredOutputInstruction
	messages := s.GetConversationForLLM()

	Log(fmt.Sprintf("Calling Claude with %d message(s), round %d", len(messages), round))
	response, err := CallClaude(sysPrompt, messages)
	if err != nil {
		return "", err
	}

	human := ParseStructuredOutput(response, s, round)
	s.AddMessage("agent", human, round, nil)
	return human, nil
}

func RunCheck(s *state.ReviewState, diff, fileContents string, changedFiles []string, headSHA, baseSHA, incrementalDiff, prTitle, prAuthor, baseBranch, headBranch string) (string, error) {
	round := len(s.ReviewRounds) + 1
	s.ReviewRounds = append(s.ReviewRounds, state.NewReviewRound(round, headSHA, baseSHA, "check", changedFiles))
	s.Summary.LastReviewedSHA = headSHA

	stateSummary := s.StateSummaryText()
	incSection := ""
	if incrementalDiff != "" {
		prevSHA := "unknown"
		if len(s.ReviewRounds) > 1 {
			prev := s.ReviewRounds[len(s.ReviewRounds)-2]
			if len(prev.HeadSHA) > 8 {
				prevSHA = prev.HeadSHA[:8]
			}
		}
		incSection = fmt.Sprintf("\n## Changes Since Last Review (since %s)\n```diff\n%s\n```\n", prevSHA, incrementalDiff)
	}

	userContent := fmt.Sprintf(`The developer has pushed new changes. Run the 'check' command.

%s

## PR Metadata
- **Title**: %s
- **Author**: %s
- **Branch**: %s -> %s
%s
## Full Diff (complete)
`+"```diff"+`
%s
`+"```"+`

## Full File Contents (current state)
%s

This is review round %d. Compare against your previous findings above.
For each finding, report: RESOLVED / PARTIALLY FIXED / UNRESOLVED.
Flag any NEW issues introduced by the fixes.`,
		stateSummary, prTitle, prAuthor, headBranch, baseBranch, incSection, diff, fileContents, round)

	s.AddMessage("system", fmt.Sprintf("Developer pushed changes. Review round %d.", round), round, nil)
	s.AddMessage("developer", userContent, round, nil)

	sysPrompt := ReadSystemPrompt(changedFiles, fileContents) + StructuredOutputInstruction
	messages := s.GetConversationForLLM()

	Log(fmt.Sprintf("Calling Claude with %d message(s), round %d", len(messages), round))
	response, err := CallClaude(sysPrompt, messages)
	if err != nil {
		return "", err
	}

	human := ParseStructuredOutput(response, s, round)
	s.AddMessage("agent", human, round, nil)
	return human, nil
}

func RunAsk(s *state.ReviewState, question, fileContents string, findingID int, changedFiles []string) (string, error) {
	round := len(s.ReviewRounds)
	stateSummary := s.StateSummaryText()

	findingContext := ""
	if findingID > 0 {
		f := s.GetFinding(findingID)
		if f != nil {
			disc := ""
			if len(f.Discussion) > 0 {
				disc = "\n**Previous discussion on this finding:**\n"
				for _, m := range f.Discussion {
					disc += fmt.Sprintf("  [%s] %s\n", m.Role, m.Content)
				}
			}
			findingContext = fmt.Sprintf(`
## Referenced Finding #%d
- **Title**: %s
- **Severity**: %s
- **Status**: %s
- **File**: `+"`%s:%d`"+`
- **Problem**: %s
- **Impact**: %s
- **Suggested fix**: %s
%s`, f.ID, f.Title, f.Severity, f.Status, f.File, f.Line, f.Problem, f.Impact, f.SuggestedFix, disc)
			_ = s.AddFindingDiscussion(findingID, "developer", question)
		}
	}

	userContent := fmt.Sprintf(`A developer is asking a follow-up question.

%s
%s

## Current Code
%s

## Developer's Message
%s

Respond according to your protocol. You have the full conversation history above for context.
If they reference a finding, trace the actual code. If they disagree, engage with their reasoning.`,
		stateSummary, findingContext, fileContents, question)

	s.AddMessage("developer", userContent, round, map[string]string{"event": "question"})

	sysPrompt := ReadSystemPrompt(changedFiles, fileContents) + StructuredOutputInstruction
	messages := s.GetConversationForLLM()

	Log(fmt.Sprintf("Calling Claude with %d message(s)", len(messages)))
	response, err := CallClaude(sysPrompt, messages)
	if err != nil {
		return "", err
	}

	human := ParseStructuredOutput(response, s, round)
	s.AddMessage("agent", human, round, nil)

	if findingID > 0 {
		truncated := human
		if len(truncated) > 500 {
			truncated = truncated[:500]
		}
		_ = s.AddFindingDiscussion(findingID, "agent", truncated)
	}
	return human, nil
}
