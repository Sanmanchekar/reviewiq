// Package ci handles GitHub Actions webhook events.
package ci

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/Sanmanchekar/reviewiq/internal/engine"
	gitops "github.com/Sanmanchekar/reviewiq/internal/git"
	"github.com/Sanmanchekar/reviewiq/internal/state"
)

func log(msg string) { fmt.Fprintf(os.Stderr, "[reviewiq:ci] %s\n", msg) }

func PostComment(repo string, prNumber int, body string) error {
	token := os.Getenv("GITHUB_TOKEN")
	payload, _ := json.Marshal(map[string]string{"body": body})
	url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d/comments", repo, prNumber)
	req, _ := http.NewRequest("POST", url, strings.NewReader(string(payload)))
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

type prMeta struct {
	Title      string
	Body       string
	Author     string
	BaseBranch string
	HeadBranch string
	BaseSHA    string
	HeadSHA    string
}

func getPRMetadata(repo string, prNumber int) (*prMeta, error) {
	token := os.Getenv("GITHUB_TOKEN")
	url := fmt.Sprintf("https://api.github.com/repos/%s/pulls/%d", repo, prNumber)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var pr struct {
		Title string `json:"title"`
		Body  string `json:"body"`
		User  struct {
			Login string `json:"login"`
		} `json:"user"`
		Base struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"base"`
		Head struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, err
	}
	return &prMeta{
		Title: pr.Title, Body: pr.Body, Author: pr.User.Login,
		BaseBranch: pr.Base.Ref, HeadBranch: pr.Head.Ref,
		BaseSHA: pr.Base.SHA, HeadSHA: pr.Head.SHA,
	}, nil
}

func Run(repo string, prNumber int, eventType, commentBody string) {
	log(fmt.Sprintf("Event: %s, PR: #%d, Repo: %s", eventType, prNumber, repo))

	s := state.Load(prNumber, repo, "auto")
	log(fmt.Sprintf("State loaded: %d findings, %d rounds", s.Summary.TotalFindings, len(s.ReviewRounds)))

	switch eventType {
	case "opened":
		handleOpened(s, repo, prNumber)
	case "synchronize":
		handleSynchronize(s, repo, prNumber)
	case "comment":
		if regexp.MustCompile(`(?i)@review-agent`).MatchString(commentBody) {
			handleComment(s, repo, prNumber, commentBody)
		} else {
			log("Comment doesn't mention @review-agent, skipping")
		}
	default:
		log("Unknown event type: " + eventType)
		os.Exit(1)
	}
}

func handleOpened(s *state.ReviewState, repo string, prNumber int) {
	log("Handling PR opened")
	meta, err := getPRMetadata(repo, prNumber)
	if err != nil {
		log("Failed to get PR metadata: " + err.Error())
		return
	}
	s.PR.Title = meta.Title
	s.PR.Author = meta.Author
	s.PR.BaseBranch = meta.BaseBranch
	s.PR.HeadBranch = meta.HeadBranch

	diff := gitops.GetDiff(meta.BaseSHA, meta.HeadSHA)
	changedFiles := gitops.GetChangedFiles(meta.BaseSHA, meta.HeadSHA)
	fileContents := gitops.ReadFiles(changedFiles)
	history := gitops.GetFileHistory(changedFiles)

	response, err := engine.RunReview(s, diff, fileContents, history, changedFiles,
		meta.HeadSHA, meta.BaseSHA, meta.Title, meta.Author, meta.Body, meta.BaseBranch, meta.HeadBranch)
	if err != nil {
		log("Review failed: " + err.Error())
		return
	}
	state.Save(s, "both")
	_ = PostComment(repo, prNumber, response)
	log(fmt.Sprintf("Review posted. %d findings tracked.", s.Summary.TotalFindings))
}

func handleSynchronize(s *state.ReviewState, repo string, prNumber int) {
	log("Handling push (incremental re-review)")
	meta, err := getPRMetadata(repo, prNumber)
	if err != nil {
		log("Failed to get PR metadata: " + err.Error())
		return
	}
	diff := gitops.GetDiff(meta.BaseSHA, meta.HeadSHA)
	changedFiles := gitops.GetChangedFiles(meta.BaseSHA, meta.HeadSHA)
	fileContents := gitops.ReadFiles(changedFiles)
	incDiff := gitops.GetIncrementalDiff(s, meta.HeadSHA)

	response, err := engine.RunCheck(s, diff, fileContents, changedFiles,
		meta.HeadSHA, meta.BaseSHA, incDiff, meta.Title, meta.Author, meta.BaseBranch, meta.HeadBranch)
	if err != nil {
		log("Re-review failed: " + err.Error())
		return
	}
	state.Save(s, "both")
	_ = PostComment(repo, prNumber, response)
	log(fmt.Sprintf("Re-review posted. Open: %d, Resolved: %d", s.Summary.Open, s.Summary.Resolved))
}

func handleComment(s *state.ReviewState, repo string, prNumber int, commentBody string) {
	command := regexp.MustCompile(`(?i)@review-agent\s*`).ReplaceAllString(commentBody, "")
	command = strings.TrimSpace(command)
	log(fmt.Sprintf("Handling comment: %.80s...", command))

	meta, err := getPRMetadata(repo, prNumber)
	if err != nil {
		log("Failed to get PR metadata: " + err.Error())
		return
	}
	changedFiles := gitops.GetChangedFiles(meta.BaseSHA, meta.HeadSHA)
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
	state.Save(s, "both")
	_ = PostComment(repo, prNumber, response)
	log("Reply posted")
}
