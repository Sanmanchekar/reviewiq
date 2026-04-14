// Package git provides git operations for ReviewIQ.
package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Sanmanchekar/reviewiq/internal/state"
)

func Run(args ...string) string {
	cmd := exec.Command("git", args...)
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}

func GetBaseBranch() string {
	ref := Run("symbolic-ref", "refs/remotes/origin/HEAD")
	if ref != "" {
		return strings.TrimPrefix(ref, "refs/remotes/origin/")
	}
	for _, branch := range []string{"main", "master"} {
		if Run("rev-parse", "--verify", "origin/"+branch) != "" {
			return branch
		}
	}
	return "main"
}

func GetDiff(base, head string) string {
	return Run("diff", base+"..."+head)
}

func GetChangedFiles(base, head string) []string {
	out := Run("diff", "--name-only", base+"..."+head)
	if out == "" {
		return nil
	}
	var files []string
	for _, f := range strings.Split(out, "\n") {
		f = strings.TrimSpace(f)
		if f != "" {
			files = append(files, f)
		}
	}
	return files
}

func ReadFiles(files []string) string {
	var b strings.Builder
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		b.WriteString("--- FILE: " + f + " ---\n")
		b.Write(data)
		b.WriteString("\n--- END: " + f + " ---\n\n")
	}
	return b.String()
}

func GetFileHistory(files []string) string {
	var b strings.Builder
	for _, f := range files {
		hist := Run("log", "-5", "--oneline", "--follow", "--", f)
		if hist != "" {
			b.WriteString("--- HISTORY: " + f + " ---\n")
			b.WriteString(hist)
			b.WriteString("\n--- END HISTORY ---\n\n")
		}
	}
	return b.String()
}

func GetCurrentSHA() string    { return Run("rev-parse", "HEAD") }
func GetCurrentBranch() string { return Run("rev-parse", "--abbrev-ref", "HEAD") }

func GetIncrementalDiff(s *state.ReviewState, headSHA string) string {
	last := s.LatestRound()
	if last == nil || last.HeadSHA == headSHA {
		return ""
	}
	return Run("diff", last.HeadSHA+"..."+headSHA)
}

// FindSkillDirs returns skill directories in priority order.
func FindSkillDirs() []string {
	var dirs []string
	if info, err := os.Stat(filepath.Join(".pr-review", "skills")); err == nil && info.IsDir() {
		dirs = append(dirs, filepath.Join(".pr-review", "skills"))
	}
	// Check next to the binary for bundled skills
	exe, err := os.Executable()
	if err == nil {
		bundled := filepath.Join(filepath.Dir(exe), "skills")
		if info, err := os.Stat(bundled); err == nil && info.IsDir() {
			dirs = append(dirs, bundled)
		}
	}
	return dirs
}
