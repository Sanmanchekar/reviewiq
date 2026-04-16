// Package github provides GitHub API helpers for PR review operations.
package github

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ── Token Resolution ───────────────────────────────────────────────────────
// Priority: GITHUB_TOKEN env → GH_TOKEN env → `gh auth token` (reuses existing git auth)

var (
	cachedToken string
	tokenOnce   sync.Once
)

// GetToken returns a GitHub token, exported for use by other packages.
func GetToken() (string, error) {
	tokenOnce.Do(func() {
		// 1. Env vars (CI / explicit override)
		if t := os.Getenv("GITHUB_TOKEN"); t != "" {
			cachedToken = t
			return
		}
		if t := os.Getenv("GH_TOKEN"); t != "" {
			cachedToken = t
			return
		}
		// 2. Reuse existing gh CLI auth (browser/SSH/credential-manager)
		out, err := exec.Command("gh", "auth", "token").Output()
		if err == nil {
			cachedToken = strings.TrimSpace(string(out))
		}
	})
	if cachedToken == "" {
		return "", fmt.Errorf("not authenticated: run 'gh auth login' or set GITHUB_TOKEN")
	}
	return cachedToken, nil
}

// ── Types ───────────────────────────────────────────────────────────────────

type PRInfo struct {
	Owner      string
	Repo       string
	Number     int
	Title      string
	Author     string
	BaseBranch string
	HeadBranch string
	BaseSHA    string
	HeadSHA    string
	Body       string
}

type PRFile struct {
	Filename  string `json:"filename"`
	Status    string `json:"status"`    // added, modified, removed, renamed
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Patch     string `json:"patch"`     // the diff for this file
}

type InlineComment struct {
	Path     string // file path
	Line     int    // line number in the diff
	Side     string // "RIGHT" for new code
	Body     string // comment body (supports ```suggestion blocks)
}

// ── PR Link Parsing ─────────────────────────────────────────────────────────

var prLinkPattern = regexp.MustCompile(`github\.com/([^/]+)/([^/]+)/pull/(\d+)`)

func ParsePRLink(link string) (owner, repo string, number int, err error) {
	match := prLinkPattern.FindStringSubmatch(link)
	if match == nil {
		return "", "", 0, fmt.Errorf("invalid PR link: %s\nExpected: https://github.com/owner/repo/pull/123", link)
	}
	fmt.Sscanf(match[3], "%d", &number)
	return match[1], match[2], number, nil
}

// ── API Client ──────────────────────────────────────────────────────────────

func apiRequest(method, endpoint string, body interface{}) ([]byte, error) {
	token, err := GetToken()
	if err != nil {
		return nil, err
	}

	url := "https://api.github.com" + endpoint
	var reader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = strings.NewReader(string(data))
	}

	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GitHub API %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 300)]))
	}
	return respBody, nil
}

// ── PR Operations ───────────────────────────────────────────────────────────

func GetPR(owner, repo string, number int) (*PRInfo, error) {
	data, err := apiRequest("GET", fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number), nil)
	if err != nil {
		return nil, err
	}

	var pr struct {
		Title string `json:"title"`
		Body  string `json:"body"`
		User  struct{ Login string } `json:"user"`
		Base  struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"base"`
		Head struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := json.Unmarshal(data, &pr); err != nil {
		return nil, err
	}

	return &PRInfo{
		Owner: owner, Repo: repo, Number: number,
		Title: pr.Title, Body: pr.Body, Author: pr.User.Login,
		BaseBranch: pr.Base.Ref, HeadBranch: pr.Head.Ref,
		BaseSHA: pr.Base.SHA, HeadSHA: pr.Head.SHA,
	}, nil
}

func GetPRFiles(owner, repo string, number int) ([]PRFile, error) {
	var allFiles []PRFile
	page := 1
	for {
		data, err := apiRequest("GET", fmt.Sprintf("/repos/%s/%s/pulls/%d/files?per_page=100&page=%d", owner, repo, number, page), nil)
		if err != nil {
			return nil, err
		}
		var files []PRFile
		if err := json.Unmarshal(data, &files); err != nil {
			return nil, err
		}
		if len(files) == 0 {
			break
		}
		allFiles = append(allFiles, files...)
		page++
	}
	return allFiles, nil
}

func GetFileContent(owner, repo, ref, path string) (string, error) {
	data, err := apiRequest("GET", fmt.Sprintf("/repos/%s/%s/contents/%s?ref=%s", owner, repo, path, ref), nil)
	if err != nil {
		return "", err
	}
	var file struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return "", err
	}
	if file.Encoding == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(file.Content, "\n", ""))
		if err != nil {
			return "", err
		}
		return string(decoded), nil
	}
	return file.Content, nil
}

// ── Posting Comments ────────────────────────────────────────────────────────

func PostPRComment(owner, repo string, number int, body string) error {
	_, err := apiRequest("POST", fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, number),
		map[string]string{"body": body})
	return err
}

func PostInlineComment(owner, repo string, number int, commitSHA string, comment InlineComment) error {
	payload := map[string]interface{}{
		"body":         comment.Body,
		"commit_id":    commitSHA,
		"path":         comment.Path,
		"line":         comment.Line,
		"side":         "RIGHT",
		"subject_type": "line",
	}
	_, err := apiRequest("POST", fmt.Sprintf("/repos/%s/%s/pulls/%d/comments", owner, repo, number), payload)
	return err
}

func PostReview(owner, repo string, number int, commitSHA, body, event string, comments []InlineComment) error {
	var reviewComments []map[string]interface{}
	for _, c := range comments {
		reviewComments = append(reviewComments, map[string]interface{}{
			"path":         c.Path,
			"line":         c.Line,
			"side":         "RIGHT",
			"body":         c.Body,
			"subject_type": "line",
		})
	}

	payload := map[string]interface{}{
		"commit_id": commitSHA,
		"body":      body,
		"event":     event, // "COMMENT", "APPROVE", "REQUEST_CHANGES"
		"comments":  reviewComments,
	}
	_, err := apiRequest("POST", fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews", owner, repo, number), payload)
	return err
}

// ── Suggestion Format ───────────────────────────────────────────────────────

func FormatSuggestion(severity, title, problem, fix, rationale string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "**[%s] %s**\n\n", severity, title)
	fmt.Fprintf(&b, "%s\n\n", problem)
	if fix != "" {
		fmt.Fprintf(&b, "```suggestion\n%s\n```\n\n", fix)
	}
	if rationale != "" {
		fmt.Fprintf(&b, "_Why_: %s\n", rationale)
	}
	return b.String()
}
