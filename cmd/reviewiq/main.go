package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/Sanmanchekar/reviewiq/internal/ci"
	"github.com/Sanmanchekar/reviewiq/internal/engine"
	gitops "github.com/Sanmanchekar/reviewiq/internal/git"
	"github.com/Sanmanchekar/reviewiq/internal/state"
)

const version = "1.0.0"

func main() {
	root := &cobra.Command{
		Use:     "reviewiq",
		Short:   "Stateful AI-powered PR review agent",
		Version: version,
	}

	root.AddCommand(initCmd(), reviewCmd(), checkCmd(), statusCmd(),
		explainCmd(), askCmd(), resolveCmd(), retractCmd(), wontfixCmd(),
		approveCmd(), ciCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func detectPRNumber(branch string) int {
	re := regexp.MustCompile(`(?i)(?:pr-?|pull/)(\d+)`)
	if m := re.FindStringSubmatch(branch); len(m) > 1 {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	h := 0
	for _, c := range branch {
		h = h*31 + int(c)
	}
	return int(math.Abs(float64(h))) % 100000
}

func findExistingState(prNumber int) (int, *state.ReviewState) {
	dir := filepath.Join(".pr-review", "reviews")
	if prNumber > 0 {
		s, _ := state.LoadLocal(prNumber)
		if s != nil {
			return prNumber, s
		}
		return 0, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, nil
	}
	type entry struct {
		name string
		mod  int64
	}
	var files []entry
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "pr-") && strings.HasSuffix(e.Name(), ".json") {
			info, _ := e.Info()
			if info != nil {
				files = append(files, entry{e.Name(), info.ModTime().Unix()})
			}
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod > files[j].mod })
	if len(files) == 0 {
		return 0, nil
	}
	re := regexp.MustCompile(`pr-(\d+)\.json`)
	if m := re.FindStringSubmatch(files[0].name); len(m) > 1 {
		n, _ := strconv.Atoi(m[1])
		s, _ := state.LoadLocal(n)
		if s != nil {
			return n, s
		}
	}
	return 0, nil
}

// ── Commands ────────────────────────────────────────────────────────────────

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize .pr-review/ in current repo",
		Run: func(cmd *cobra.Command, args []string) {
			agentFile := filepath.Join(".pr-review", "agent.md")
			skillsDir := filepath.Join(".pr-review", "skills")

			if _, err := os.Stat(agentFile); err == nil {
				if _, err := os.Stat(skillsDir); err == nil {
					fmt.Println("Already initialized: .pr-review/agent.md and skills/ exist")
					return
				}
			}

			os.MkdirAll(filepath.Join(".pr-review", "skills"), 0o755)

			if _, err := os.Stat(agentFile); err != nil {
				os.WriteFile(agentFile, []byte(defaultAgentMD), 0o644)
			}

			// Add to gitignore
			gitignoreLine := ".pr-review/reviews/"
			data, _ := os.ReadFile(".gitignore")
			if !strings.Contains(string(data), gitignoreLine) {
				f, _ := os.OpenFile(".gitignore", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
				if f != nil {
					f.WriteString("\n# ReviewIQ state files\n" + gitignoreLine + "\n")
					f.Close()
				}
			}

			color.Green("Initialized ReviewIQ:")
			fmt.Println("  .pr-review/agent.md    — review protocol (customize this)")
			fmt.Println("  .pr-review/skills/     — add skill .md files here")
			fmt.Println("  .gitignore             — updated to exclude state files")
			fmt.Println()
			fmt.Println("Next: reviewiq review <branch>")
		},
	}
}

func reviewCmd() *cobra.Command {
	var base string
	var pr int
	cmd := &cobra.Command{
		Use:   "review <branch>",
		Short: "Full review of a PR branch",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			branch := args[0]
			if base == "" {
				base = gitops.GetBaseBranch()
			}
			fmt.Printf("Reviewing %s against %s...\n", branch, base)

			diff := gitops.GetDiff(base, branch)
			if diff == "" {
				color.Red("No diff found between %s and %s.", base, branch)
				os.Exit(1)
			}
			changedFiles := gitops.GetChangedFiles(base, branch)
			fileContents := gitops.ReadFiles(changedFiles)
			history := gitops.GetFileHistory(changedFiles)
			headSHA := gitops.Run("rev-parse", branch)
			baseSHA := gitops.Run("rev-parse", base)

			if pr == 0 {
				pr = detectPRNumber(branch)
			}
			s := state.Load(pr, "", "local")
			s.PR.Title = "Review of " + branch
			s.PR.Author = gitops.Run("config", "user.name")
			s.PR.BaseBranch = base
			s.PR.HeadBranch = branch

			response, err := engine.RunReview(s, diff, fileContents, history, changedFiles,
				headSHA, baseSHA, s.PR.Title, s.PR.Author, "", base, branch)
			if err != nil {
				color.Red("Review failed: %s", err)
				os.Exit(1)
			}

			state.Save(s, "local")
			fmt.Println(response)
			fmt.Printf("\n")
			color.Green("State saved: .pr-review/reviews/pr-%d.json", pr)
			fmt.Printf("Findings: %d (%d open)\n", s.Summary.TotalFindings, s.Summary.Open)
		},
	}
	cmd.Flags().StringVar(&base, "base", "", "Base branch (default: auto-detect)")
	cmd.Flags().IntVar(&pr, "pr", 0, "PR number (default: auto-detect)")
	return cmd
}

func checkCmd() *cobra.Command {
	var base string
	var pr int
	cmd := &cobra.Command{
		Use:   "check <branch>",
		Short: "Incremental re-review after new commits",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			branch := args[0]
			if base == "" {
				base = gitops.GetBaseBranch()
			}
			if pr == 0 {
				pr = detectPRNumber(branch)
			}
			prNum, s := findExistingState(pr)
			if s == nil {
				color.Red("No existing review state found. Run 'reviewiq review <branch>' first.")
				os.Exit(1)
			}
			fmt.Printf("Re-reviewing %s (round %d)...\n", branch, len(s.ReviewRounds)+1)

			diff := gitops.GetDiff(base, branch)
			changedFiles := gitops.GetChangedFiles(base, branch)
			fileContents := gitops.ReadFiles(changedFiles)
			headSHA := gitops.Run("rev-parse", branch)
			baseSHA := gitops.Run("rev-parse", base)
			incDiff := gitops.GetIncrementalDiff(s, headSHA)

			response, err := engine.RunCheck(s, diff, fileContents, changedFiles,
				headSHA, baseSHA, incDiff, s.PR.Title, s.PR.Author, base, branch)
			if err != nil {
				color.Red("Re-review failed: %s", err)
				os.Exit(1)
			}

			state.Save(s, "local")
			fmt.Println(response)
			fmt.Printf("\nOpen: %d | Resolved: %d | Assessment: %s\n",
				s.Summary.Open, s.Summary.Resolved, s.Summary.Assessment)
			_ = prNum
		},
	}
	cmd.Flags().StringVar(&base, "base", "", "Base branch")
	cmd.Flags().IntVar(&pr, "pr", 0, "PR number")
	return cmd
}

func statusCmd() *cobra.Command {
	var pr int
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show current finding statuses",
		Run: func(cmd *cobra.Command, args []string) {
			prNum, s := findExistingState(pr)
			if s == nil {
				color.Red("No review state found. Run 'reviewiq review <branch>' first.")
				os.Exit(1)
			}
			sm := s.Summary
			bold := color.New(color.Bold)
			bold.Printf("ReviewIQ Status — PR #%d (Round %d)\n", prNum, len(s.ReviewRounds))
			fmt.Printf("Assessment: %s\n\n", sm.Assessment)

			if len(s.Findings) == 0 {
				fmt.Println("No findings.")
				return
			}

			fmt.Printf("%-4s %-12s %-16s %-40s %s\n", "#", "Severity", "Status", "Title", "File")
			fmt.Println(strings.Repeat("-", 100))

			for _, id := range s.SortedFindingIDs() {
				f := s.Findings[fmt.Sprintf("%d", id)]
				title := f.Title
				if len(title) > 40 {
					title = title[:37] + "..."
				}
				fmt.Printf("%-4d %-12s %-16s %-40s %s:%d\n",
					f.ID, f.Severity, strings.ToUpper(f.Status), title, f.File, f.Line)
			}

			fmt.Printf("\nTotal: %d | Open: %d | Resolved: %d | Won't fix: %d | Retracted: %d\n",
				sm.TotalFindings, sm.Open, sm.Resolved, sm.Wontfix, sm.Retracted)
		},
	}
	cmd.Flags().IntVar(&pr, "pr", 0, "PR number (default: most recent)")
	return cmd
}

func explainCmd() *cobra.Command {
	var pr int
	cmd := &cobra.Command{
		Use:   "explain <finding-id>",
		Short: "Deep dive into a specific finding",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fid, _ := strconv.Atoi(args[0])
			_, s := findExistingState(pr)
			if s == nil {
				color.Red("No review state found.")
				os.Exit(1)
			}
			f := s.GetFinding(fid)
			if f == nil {
				color.Red("Finding %d not found.", fid)
				os.Exit(1)
			}
			fileContents := gitops.ReadFiles([]string{f.File})
			response, err := engine.RunAsk(s, fmt.Sprintf("explain finding %d", fid), fileContents, fid, []string{f.File})
			if err != nil {
				color.Red("Failed: %s", err)
				os.Exit(1)
			}
			state.Save(s, "local")
			fmt.Println(response)
		},
	}
	cmd.Flags().IntVar(&pr, "pr", 0, "PR number")
	return cmd
}

func askCmd() *cobra.Command {
	var pr int
	cmd := &cobra.Command{
		Use:   "ask <question>",
		Short: "Ask a follow-up question",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			question := strings.Join(args, " ")
			_, s := findExistingState(pr)
			if s == nil {
				color.Red("No review state found.")
				os.Exit(1)
			}
			findingID := 0
			if m := regexp.MustCompile(`(?i)(?:finding|#)\s*(\d+)`).FindStringSubmatch(question); len(m) > 1 {
				findingID, _ = strconv.Atoi(m[1])
			}
			base := gitops.GetBaseBranch()
			branch := s.PR.HeadBranch
			if branch == "" {
				branch = gitops.GetCurrentBranch()
			}
			changedFiles := gitops.GetChangedFiles(base, branch)
			fileContents := gitops.ReadFiles(changedFiles)

			response, err := engine.RunAsk(s, question, fileContents, findingID, changedFiles)
			if err != nil {
				color.Red("Failed: %s", err)
				os.Exit(1)
			}
			state.Save(s, "local")
			fmt.Println(response)
		},
	}
	cmd.Flags().IntVar(&pr, "pr", 0, "PR number")
	return cmd
}

func transitionCmd(use, short, targetStatus string) *cobra.Command {
	var pr int
	var note string
	cmd := &cobra.Command{
		Use:   use + " <finding-id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fid, _ := strconv.Atoi(args[0])
			_, s := findExistingState(pr)
			if s == nil {
				color.Red("No review state found.")
				os.Exit(1)
			}
			f := s.GetFinding(fid)
			if f == nil {
				color.Red("Finding %d not found.", fid)
				os.Exit(1)
			}
			oldStatus := f.Status
			round := len(s.ReviewRounds)
			if err := s.TransitionFinding(fid, targetStatus, note, round); err != nil {
				color.Red("Error: %s", err)
				os.Exit(1)
			}
			state.Save(s, "local")
			color.Green("Finding %d: %s -> %s", fid, oldStatus, targetStatus)
			if note != "" {
				fmt.Printf("Note: %s\n", note)
			}
			fmt.Printf("Assessment: %s\n", s.Summary.Assessment)
		},
	}
	cmd.Flags().IntVar(&pr, "pr", 0, "PR number")
	cmd.Flags().StringVarP(&note, "note", "n", "", "Reason/note for the transition")
	return cmd
}

func resolveCmd() *cobra.Command { return transitionCmd("resolve", "Mark finding as resolved", "resolved") }
func retractCmd() *cobra.Command { return transitionCmd("retract", "Retract a finding (agent was wrong)", "retracted") }
func wontfixCmd() *cobra.Command { return transitionCmd("wontfix", "Mark finding as won't fix", "wontfix") }

func approveCmd() *cobra.Command {
	var pr int
	cmd := &cobra.Command{
		Use:   "approve",
		Short: "Final check for remaining blockers",
		Run: func(cmd *cobra.Command, args []string) {
			_, s := findExistingState(pr)
			if s == nil {
				color.Red("No review state found.")
				os.Exit(1)
			}
			open := s.OpenFindings()
			var blockers, nits []state.Finding
			for _, f := range open {
				if f.Severity == "CRITICAL" || f.Severity == "IMPORTANT" {
					blockers = append(blockers, f)
				} else {
					nits = append(nits, f)
				}
			}

			if len(blockers) > 0 {
				color.Red("BLOCKED — the following findings must be addressed:\n")
				for _, f := range blockers {
					fmt.Printf("  [%s] Finding %d: %s\n", f.Severity, f.ID, f.Title)
					fmt.Printf("    %s:%d — %.80s\n\n", f.File, f.Line, f.Problem)
				}
				if len(nits) > 0 {
					fmt.Printf("Plus %d non-blocking nit(s).\n", len(nits))
				}
			} else if len(nits) > 0 {
				color.Green("APPROVE with nits:\n")
				for _, f := range nits {
					fmt.Printf("  [NIT] Finding %d: %s\n", f.ID, f.Title)
				}
				fmt.Println("\nNo blockers. Safe to merge.")
			} else {
				color.Green("APPROVE — no remaining findings. Safe to merge.")
				s.Summary.Assessment = "APPROVE"
				state.Save(s, "local")
			}
		},
	}
	cmd.Flags().IntVar(&pr, "pr", 0, "PR number")
	return cmd
}

func ciCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ci",
		Short: "Run in CI mode (GitHub Actions)",
		Run: func(cmd *cobra.Command, args []string) {
			repo := os.Getenv("GITHUB_REPOSITORY")
			prStr := os.Getenv("PR_NUMBER")
			eventType := os.Getenv("EVENT_TYPE")
			commentBody := os.Getenv("COMMENT_BODY")

			if repo == "" || prStr == "" || eventType == "" {
				color.Red("CI mode requires GITHUB_REPOSITORY, PR_NUMBER, EVENT_TYPE env vars.")
				os.Exit(1)
			}
			prNumber, _ := strconv.Atoi(prStr)
			ci.Run(repo, prNumber, eventType, commentBody)
		},
	}
}

const defaultAgentMD = `# PR Review Agent

You are a stateful PR review agent. See https://github.com/Sanmanchekar/reviewiq for the full protocol.

## Commands
- review: Full 4-stage review (understand, analyze, assess, report)
- check: Incremental re-review after new commits
- explain <N>: Deep dive into finding N
- fix <N>: Apply suggested fix
- status: Show current findings

## Rules
1. Never hallucinate file contents — always read the file
2. Concrete fixes only — every suggestion must be copy-pasteable
3. Match repo conventions
4. Engage with developer pushback — they know the codebase
`
