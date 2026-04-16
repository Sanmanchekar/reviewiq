// Package ci handles GitHub Actions webhook events.
package ci

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/Sanmanchekar/reviewiq/internal/engine"
	gitops "github.com/Sanmanchekar/reviewiq/internal/git"
	gh "github.com/Sanmanchekar/reviewiq/internal/github"
	"github.com/Sanmanchekar/reviewiq/internal/state"
)

func log(msg string) { fmt.Fprintf(os.Stderr, "[reviewiq:ci] %s\n", msg) }

func Run(repo string, prNumber int, eventType, commentBody string) {
	log(fmt.Sprintf("Event: %s, PR: #%d, Repo: %s", eventType, prNumber, repo))

	s := state.Load(prNumber, repo)
	s.PR.Repo = repo
	log(fmt.Sprintf("State loaded: %d findings, %d rounds", s.Summary.TotalFindings, len(s.ReviewRounds)))

	owner, repoName := splitRepo(repo)

	switch eventType {
	case "opened":
		handleOpened(s, owner, repoName, repo, prNumber)
	case "synchronize":
		handleSynchronize(s, owner, repoName, repo, prNumber)
	case "comment":
		handleComment(s, owner, repoName, repo, prNumber, commentBody)
	default:
		log("Unknown event type: " + eventType)
		os.Exit(1)
	}
}

func splitRepo(repo string) (string, string) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return "", repo
	}
	return parts[0], parts[1]
}

func handleOpened(s *state.ReviewState, owner, repoName, fullRepo string, prNumber int) {
	log("Handling PR opened — full review")
	prInfo, err := gh.GetPR(owner, repoName, prNumber)
	if err != nil {
		log("Failed to get PR: " + err.Error())
		return
	}
	s.PR.Title = prInfo.Title
	s.PR.Author = prInfo.Author
	s.PR.BaseBranch = prInfo.BaseBranch
	s.PR.HeadBranch = prInfo.HeadBranch

	diff := gitops.GetDiff(prInfo.BaseSHA, prInfo.HeadSHA)
	changedFiles := gitops.GetChangedFiles(prInfo.BaseSHA, prInfo.HeadSHA)
	fileContents := gitops.ReadFiles(changedFiles)
	history := gitops.GetFileHistory(changedFiles)

	response, err := engine.RunReview(s, diff, fileContents, history, changedFiles,
		prInfo.HeadSHA, prInfo.BaseSHA, prInfo.Title, prInfo.Author, prInfo.Body, prInfo.BaseBranch, prInfo.HeadBranch)
	if err != nil {
		log("Review failed: " + err.Error())
		return
	}

	// Post inline comments for each finding
	round := len(s.ReviewRounds)
	var comments []gh.InlineComment
	for _, f := range s.Findings {
		if f.CreatedRound == round {
			comments = append(comments, gh.InlineComment{
				Path: f.File,
				Line: f.Line,
				Body: gh.FormatSuggestion(f.Severity, f.Title, f.Problem, f.SuggestedFix, f.FixRationale),
			})
		}
	}

	if len(comments) > 0 {
		event := "COMMENT"
		if s.Summary.Open > 0 {
			event = "REQUEST_CHANGES"
		}
		summary := fmt.Sprintf("## ReviewIQ Review — Round %d\n\n**%d findings** | Assessment: **%s**",
			round, s.Summary.TotalFindings, s.Summary.Assessment)
		if err := gh.PostReview(owner, repoName, prNumber, prInfo.HeadSHA, summary, event, comments); err != nil {
			log("Failed to post review with inline comments: " + err.Error())
			// Fallback to plain comment
			_ = gh.PostPRComment(owner, repoName, prNumber, response)
		}
	} else {
		_ = gh.PostPRComment(owner, repoName, prNumber, response)
	}

	state.Save(s)
	log(fmt.Sprintf("Review posted. %d findings, %d inline comments.", s.Summary.TotalFindings, len(comments)))
}

func handleSynchronize(s *state.ReviewState, owner, repoName, fullRepo string, prNumber int) {
	log("Handling push — incremental re-review")
	prInfo, err := gh.GetPR(owner, repoName, prNumber)
	if err != nil {
		log("Failed to get PR: " + err.Error())
		return
	}

	diff := gitops.GetDiff(prInfo.BaseSHA, prInfo.HeadSHA)
	changedFiles := gitops.GetChangedFiles(prInfo.BaseSHA, prInfo.HeadSHA)
	fileContents := gitops.ReadFiles(changedFiles)
	incDiff := gitops.GetIncrementalDiff(s, prInfo.HeadSHA)

	response, err := engine.RunCheck(s, diff, fileContents, changedFiles,
		prInfo.HeadSHA, prInfo.BaseSHA, incDiff, prInfo.Title, prInfo.Author, prInfo.BaseBranch, prInfo.HeadBranch)
	if err != nil {
		log("Re-review failed: " + err.Error())
		return
	}

	// Post inline comments only for NEW findings in this round
	round := len(s.ReviewRounds)
	var comments []gh.InlineComment
	for _, f := range s.Findings {
		if f.CreatedRound == round {
			comments = append(comments, gh.InlineComment{
				Path: f.File,
				Line: f.Line,
				Body: gh.FormatSuggestion(f.Severity, f.Title, f.Problem, f.SuggestedFix, f.FixRationale),
			})
		}
	}

	if len(comments) > 0 {
		summary := fmt.Sprintf("## ReviewIQ Re-review — Round %d\n\nOpen: %d | Resolved: %d | New: %d | Assessment: **%s**",
			round, s.Summary.Open, s.Summary.Resolved, len(comments), s.Summary.Assessment)
		if err := gh.PostReview(owner, repoName, prNumber, prInfo.HeadSHA, summary, "COMMENT", comments); err != nil {
			log("Failed to post inline comments: " + err.Error())
			_ = gh.PostPRComment(owner, repoName, prNumber, response)
		}
	} else {
		// No new findings — post summary as comment
		_ = gh.PostPRComment(owner, repoName, prNumber, response)
	}

	state.Save(s)
	log(fmt.Sprintf("Re-review posted. Open: %d, Resolved: %d", s.Summary.Open, s.Summary.Resolved))
}

func handleComment(s *state.ReviewState, owner, repoName, fullRepo string, prNumber int, commentBody string) {
	if !regexp.MustCompile(`(?i)@review-agent`).MatchString(commentBody) {
		log("Comment doesn't mention @review-agent, skipping")
		return
	}

	command := regexp.MustCompile(`(?i)@review-agent\s*`).ReplaceAllString(commentBody, "")
	command = strings.TrimSpace(command)
	log(fmt.Sprintf("Handling comment command: %.80s", command))

	// Detect structured commands
	lowerCmd := strings.ToLower(command)

	switch {
	case strings.HasPrefix(lowerCmd, "resolve") || strings.HasPrefix(lowerCmd, "approve"):
		handleResolveComment(s, owner, repoName, fullRepo, prNumber)
	case strings.HasPrefix(lowerCmd, "recheck") || strings.HasPrefix(lowerCmd, "check"):
		handleSynchronize(s, owner, repoName, fullRepo, prNumber)
	default:
		// Freeform ask/explain
		handleAskComment(s, owner, repoName, fullRepo, prNumber, command)
	}
}

func handleResolveComment(s *state.ReviewState, owner, repoName, fullRepo string, prNumber int) {
	log("Handling resolve command")

	open := s.OpenFindings()
	if len(open) == 0 {
		msg := "## ReviewIQ Resolve\n\nNo open findings. PR is already clear."
		_ = gh.PostPRComment(owner, repoName, prNumber, msg)
		// Approve
		_ = gh.PostReview(owner, repoName, prNumber, s.Summary.LastReviewedSHA,
			"All findings resolved — ReviewIQ", "APPROVE", nil)
		s.Summary.Assessment = "APPROVE"
		state.Save(s)
		return
	}

	// Check each finding against current code
	round := len(s.ReviewRounds) + 1
	resolvedCount := 0
	for _, f := range open {
		// Read current file to check if fix was applied
		content, err := gh.GetFileContent(owner, repoName, s.Summary.LastReviewedSHA, f.File)
		if err != nil {
			continue
		}
		// If the suggested fix text appears in the file, consider it resolved
		if f.SuggestedFix != "" && strings.Contains(content, strings.TrimSpace(f.SuggestedFix)) {
			_ = s.TransitionFinding(f.ID, "resolved", "Fix confirmed in code", round)
			resolvedCount++
		}
	}

	remaining := s.OpenFindings()
	var msg string
	if len(remaining) == 0 {
		msg = fmt.Sprintf("## ReviewIQ Resolve — Round %d\n\nAll %d findings resolved. PR APPROVED.", round, resolvedCount)
		_ = gh.PostReview(owner, repoName, prNumber, s.Summary.LastReviewedSHA,
			"All findings resolved — ReviewIQ", "APPROVE", nil)
		s.Summary.Assessment = "APPROVE"
	} else {
		var lines []string
		for _, f := range remaining {
			lines = append(lines, fmt.Sprintf("- **Finding %d** [%s]: %s — `%s:%d`", f.ID, f.Severity, f.Title, f.File, f.Line))
		}
		msg = fmt.Sprintf("## ReviewIQ Resolve — Round %d\n\nResolved: %d | Still open: %d\n\n%s\n\nPush fixes and comment `@review-agent resolve` again.",
			round, resolvedCount, len(remaining), strings.Join(lines, "\n"))
	}

	_ = gh.PostPRComment(owner, repoName, prNumber, msg)
	state.Save(s)
	log(fmt.Sprintf("Resolve: %d resolved, %d remaining", resolvedCount, len(remaining)))
}

func handleAskComment(s *state.ReviewState, owner, repoName, fullRepo string, prNumber int, command string) {
	prInfo, err := gh.GetPR(owner, repoName, prNumber)
	if err != nil {
		log("Failed to get PR metadata: " + err.Error())
		return
	}
	changedFiles := gitops.GetChangedFiles(prInfo.BaseSHA, prInfo.HeadSHA)
	fileContents := gitops.ReadFiles(changedFiles)

	findingID := 0
	if m := regexp.MustCompile(`(?i)(?:finding|#)\s*(\d+)`).FindStringSubmatch(command); len(m) > 1 {
		fmt.Sscanf(m[1], "%d", &findingID)
	}

	response, err := engine.RunAsk(s, command, fileContents, findingID, changedFiles)
	if err != nil {
		log("Ask failed: " + err.Error())
		return
	}
	state.Save(s)
	_ = gh.PostPRComment(owner, repoName, prNumber, response)
	log("Reply posted")
}
