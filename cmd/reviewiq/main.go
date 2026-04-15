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
	gh "github.com/Sanmanchekar/reviewiq/internal/github"
	"github.com/Sanmanchekar/reviewiq/internal/skills"
	"github.com/Sanmanchekar/reviewiq/internal/state"
)

const version = "1.0.0"

func main() {
	root := &cobra.Command{
		Use:     "reviewiq",
		Short:   "Stateful AI-powered PR review agent",
		Version: version,
	}

	root.AddCommand(initCmd(), reviewCmd(), reviewPRCmd(), reviewFullCmd(),
		checkCmd(), statusCmd(), explainCmd(), askCmd(),
		resolveCmd(), retractCmd(), wontfixCmd(), approveCmd(), ciCmd())

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
		Short: "Initialize .pr-review/ and .claude/commands/ in current repo",
		Run: func(cmd *cobra.Command, args []string) {
			created := 0

			// .pr-review/agent.md
			agentFile := filepath.Join(".pr-review", "agent.md")
			os.MkdirAll(filepath.Join(".pr-review", "skills"), 0o755)
			if _, err := os.Stat(agentFile); err != nil {
				os.WriteFile(agentFile, []byte(defaultAgentMD), 0o644)
				created++
			}

			// .claude/commands/ — slash commands for Claude Code
			claudeDir := filepath.Join(".claude", "commands")
			os.MkdirAll(claudeDir, 0o755)
			for name, content := range claudeCommands {
				path := filepath.Join(claudeDir, name+".md")
				if _, err := os.Stat(path); err != nil {
					os.WriteFile(path, []byte(content), 0o644)
					created++
				}
			}

			// .gitignore
			gitignoreLine := ".pr-review/reviews/"
			data, _ := os.ReadFile(".gitignore")
			if !strings.Contains(string(data), gitignoreLine) {
				f, _ := os.OpenFile(".gitignore", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
				if f != nil {
					f.WriteString("\n# ReviewIQ state files\n" + gitignoreLine + "\n")
					f.Close()
				}
			}

			if created == 0 {
				fmt.Println("Already initialized. All files exist.")
				return
			}

			color.Green("Initialized ReviewIQ:\n")
			fmt.Println("  .pr-review/agent.md              — review protocol")
			fmt.Println("  .pr-review/skills/               — add skill .md files here")
			fmt.Println()
			fmt.Println("  Claude Code commands (14):")
			cmds := []string{"reviewiq-full", "reviewiq-pr", "reviewiq-check", "reviewiq-explain", "reviewiq-fix",
				"reviewiq-status", "reviewiq-ask", "reviewiq-retract", "reviewiq-wontfix",
				"reviewiq-resolve", "reviewiq-approve", "reviewiq-summarize", "reviewiq-impact", "reviewiq-test"}
			for _, c := range cmds {
				if _, ok := claudeCommands[c]; ok {
					fmt.Printf("    /%-24s .claude/commands/%s.md\n", c, c)
				}
			}
			fmt.Println()
			fmt.Println("Usage:")
			fmt.Println("  Claude Code: /reviewiq-full <PR-link>   (one-shot, auto-posts)")
			fmt.Println("               /reviewiq-pr <PR-link>     (file-by-file)")
			fmt.Println("  CLI:         reviewiq full <PR-link>")
			fmt.Println("               reviewiq pr <PR-link> --post")
		},
	}
}

var claudeCommands = map[string]string{
	"reviewiq-full": `Full PR review in one shot: $ARGUMENTS

$ARGUMENTS: GitHub PR link or number.

Reviews entire diff at once, posts inline comments with suggestion blocks, and summary to the PR. No interaction needed.

## Steps
1. Fetch PR: ` + "`gh pr view <number> --json title,author,baseRefName,headRefName,files`" + `
2. Get full diff: ` + "`gh pr diff <number>`" + `
3. Read ALL changed files in full
4. Load ALL relevant skills from ~/.reviewiq/skills/ or .pr-review/skills/
5. Run 4-stage review with cross-file analysis
6. Post inline comments with suggestion blocks on each finding
7. Post summary comment with finding table and assessment
8. Save state to .pr-review/reviews/pr-<N>.json
`,
	"reviewiq-pr": `Review the PR: $ARGUMENTS

Follow the protocol in ` + "`.pr-review/agent.md`" + `. Load relevant skills from ` + "`.pr-review/skills/`" + ` based on the changed files.

## Steps

1. Check for existing state: ` + "`ls .pr-review/reviews/`" + ` — if found, read the state file for prior findings.
2. Detect base branch: ` + "`git symbolic-ref refs/remotes/origin/HEAD | sed 's@^refs/remotes/origin/@@'`" + ` (fallback: main)
3. Get diff: ` + "`git diff <base>...$ARGUMENTS`" + `
4. Read ALL changed files in full
5. Load skill files from .pr-review/skills/ — always load commandments, security, scalability, stability, maintainability, performance. Then load language/framework/domain skills matching the changed files.
6. Run the 4-stage review: Understand → Analyze (against skill checklists) → Assess (CRITICAL/IMPORTANT/NIT/QUESTION) → Report
7. Save findings to .pr-review/reviews/pr-<N>.json per the state schema in agent.md

After review, remind the developer of available commands:
/reviewiq-check, /reviewiq-explain, /reviewiq-fix, /reviewiq-status, /reviewiq-ask, /reviewiq-retract, /reviewiq-wontfix, /reviewiq-approve, /reviewiq-summarize
`,
	"reviewiq-check": `Incremental re-review of branch: $ARGUMENTS

The developer has pushed fixes. Follow the check command in .pr-review/agent.md.

## Steps
1. Load state from .pr-review/reviews/ — you know which findings are open and what SHA was last reviewed.
2. Get the incremental diff since last_reviewed_sha in state.
3. Read all currently changed files in full.
4. Load relevant skills from .pr-review/skills/.
5. For each finding: RESOLVED → transition with note. PARTIALLY FIXED → note what's missing. UNRESOLVED → keep open.
6. Check for NEW issues introduced by the fixes.
7. Create a new review round in state, save.
8. Output status update table and updated summary.
`,
	"reviewiq-explain": `Deep dive into finding: $ARGUMENTS

## Steps
1. Load state from .pr-review/reviews/ — find the finding by ID.
2. Read the file referenced by the finding in full.
3. Trace execution: what calls this? What does it call? Use git grep -n <symbol>.
4. Show concrete scenarios where the issue manifests.
5. If the developer disagrees and is right, transition to retracted.
6. Add exchange to the finding's discussion array. Save state.
`,
	"reviewiq-fix": `Apply the suggested fix for finding: $ARGUMENTS

## Steps
1. Load state — find the finding by ID.
2. Read the current file state.
3. Apply the suggested fix (or refined version from discussion).
4. Self-check: re-read file, verify syntax, logic, imports, no side effects.
5. If fix touches shared code, check callers with git grep.
6. Transition finding to resolved with note. Save state.
`,
	"reviewiq-status": `Show current finding statuses.

## Steps
1. Find the most recent state file in .pr-review/reviews/.
2. Read it and output a status table:

| # | Severity | Status | Title | File |
|---|----------|--------|-------|------|

Open: X | Resolved: Y | Won't fix: Z | Retracted: W | Assessment: ...

Do NOT re-review. Just read and display the state file.
`,
	"reviewiq-ask": `Follow-up question about the review: $ARGUMENTS

## Steps
1. Load state from .pr-review/reviews/.
2. If question references a finding, load its full context and discussion history.
3. Read the relevant code files.
4. Answer using loaded skill knowledge and code tracing.
5. If answer leads to a status change, update state.
6. Add to finding's discussion thread if applicable. Save state.
`,
	"reviewiq-retract": `Retract finding (agent was wrong): $ARGUMENTS

Format: <finding-id> [reason]

## Steps
1. Load state. Parse finding ID and reason.
2. Transition finding to retracted with reason. Recompute summary. Save state.
3. Output: Finding <N>: open → retracted — <reason>
`,
	"reviewiq-wontfix": `Mark finding as won't fix: $ARGUMENTS

Format: <finding-id> [reason]

## Steps
1. Load state. Parse finding ID and reason.
2. Consider if reasoning is sound. If yes, transition to wontfix. If not, explain why and keep open.
3. Record in discussion thread. Recompute summary. Save state.
`,
	"reviewiq-resolve": `Mark finding as resolved: $ARGUMENTS

Format: <finding-id> [note]

## Steps
1. Load state. Parse finding ID and note.
2. Transition to resolved. Recompute summary. Save state.
3. Output: Finding <N>: open → resolved — <note>
4. If all blockers resolved, note PR may be ready to merge.
`,
	"reviewiq-approve": `Final check — any blockers remaining?

## Steps
1. Load state from .pr-review/reviews/.
2. List CRITICAL/IMPORTANT findings still open — these are blockers.
3. Read all changed files one final time.
4. Output BLOCKED (with list) or APPROVE (safe to merge).
5. Update assessment in state if approved. Save state.
`,
	"reviewiq-summarize": `Generate a PR summary for the merge commit.

## Steps
1. Load state. Read the full diff and changed files.
2. Generate concise summary: what changed, why, key decisions, findings addressed, trade-offs.
3. Format for merge commit message or PR description.
`,
	"reviewiq-impact": `Blast radius analysis for the current PR.

## Steps
1. Load state. Get the full diff and changed files.
2. For each changed function/class, trace ALL callers with git grep -n <symbol>.
3. Map: direct callers, transitive dependencies, shared state, external consumers.
4. Flag what could break in production but pass tests.
5. Output blast radius table.
`,
	"reviewiq-test": `Generate test cases for the reviewed changes: $ARGUMENTS

Optional: specific finding ID to focus on.

## Steps
1. Load state for open findings.
2. Find existing test files to learn conventions.
3. Generate tests: happy path, edge cases from findings, regression tests.
4. Match repo's test conventions. Output test code.
`,
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

func reviewPRCmd() *cobra.Command {
	var post bool
	cmd := &cobra.Command{
		Use:   "pr <pr-link-or-number>",
		Short: "Review a GitHub PR file-by-file with inline comments",
		Long: `Review a GitHub PR by fetching diffs from the GitHub API,
reviewing each file against relevant skills, and posting
inline comments with suggestion blocks on the PR.

Examples:
  reviewiq pr https://github.com/owner/repo/pull/42
  reviewiq pr 42  (if inside the repo)`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			input := args[0]

			// Parse PR link or number
			var owner, repo string
			var prNumber int

			if strings.Contains(input, "github.com") {
				var err error
				owner, repo, prNumber, err = gh.ParsePRLink(input)
				if err != nil {
					color.Red("%s", err)
					os.Exit(1)
				}
			} else {
				prNumber, _ = strconv.Atoi(input)
				if prNumber == 0 {
					color.Red("Invalid input. Provide a PR link or number.")
					os.Exit(1)
				}
				// Detect owner/repo from git remote
				remote := gitops.Run("remote", "get-url", "origin")
				re := regexp.MustCompile(`github\.com[:/]([^/]+)/([^/.]+)`)
				m := re.FindStringSubmatch(remote)
				if m == nil {
					color.Red("Cannot detect GitHub repo from remote. Use full PR link.")
					os.Exit(1)
				}
				owner, repo = m[1], m[2]
			}

			bold := color.New(color.Bold)
			cyan := color.New(color.FgCyan)

			// Fetch PR info
			step := color.New(color.FgCyan)
			step.Printf("[reviewiq] Fetching PR #%d from %s/%s...\n", prNumber, owner, repo)

			prInfo, err := gh.GetPR(owner, repo, prNumber)
			if err != nil {
				color.Red("Failed to fetch PR: %s", err)
				os.Exit(1)
			}

			files, err := gh.GetPRFiles(owner, repo, prNumber)
			if err != nil {
				color.Red("Failed to fetch PR files: %s", err)
				os.Exit(1)
			}

			// Show PR overview
			fmt.Println()
			bold.Printf("PR #%d: %s\n", prNumber, prInfo.Title)
			fmt.Printf("Author: @%s\n", prInfo.Author)
			fmt.Printf("Branch: %s → %s\n", prInfo.HeadBranch, prInfo.BaseBranch)
			fmt.Printf("Changed files (%d):\n", len(files))
			for i, f := range files {
				fmt.Printf("  %d. %-45s (+%d, -%d)\n", i+1, f.Filename, f.Additions, f.Deletions)
			}
			fmt.Println()

			// Load state
			s := state.Load(prNumber, fmt.Sprintf("%s/%s", owner, repo), "auto")
			s.PR.Title = prInfo.Title
			s.PR.Author = prInfo.Author
			s.PR.BaseBranch = prInfo.BaseBranch
			s.PR.HeadBranch = prInfo.HeadBranch
			s.PR.Repo = fmt.Sprintf("%s/%s", owner, repo)

			round := len(s.ReviewRounds) + 1
			var allFileNames []string
			for _, f := range files {
				allFileNames = append(allFileNames, f.Filename)
			}
			s.ReviewRounds = append(s.ReviewRounds, state.NewReviewRound(
				round, prInfo.HeadSHA, prInfo.BaseSHA, "reviewiq-pr", allFileNames,
			))
			s.Summary.LastReviewedSHA = prInfo.HeadSHA

			findingCounter := len(s.Findings)
			var allComments []gh.InlineComment

			// Review file by file
			for i, file := range files {
				if file.Patch == "" {
					continue // binary or empty
				}

				cyan.Printf("\n[reviewiq] Reviewing file %d/%d: %s\n", i+1, len(files), file.Filename)

				// Detect skills for THIS file only
				detected := skills.Detect([]string{file.Filename}, file.Patch)
				skillPrompt := skills.LoadSkills(detected)

				var loadedSkills []string
				loadedSkills = append(loadedSkills, detected.Always...)
				loadedSkills = append(loadedSkills, detected.Languages...)
				loadedSkills = append(loadedSkills, detected.Frameworks...)
				loadedSkills = append(loadedSkills, detected.DevOps...)
				loadedSkills = append(loadedSkills, detected.Domains...)
				fmt.Printf("  Skills: %s\n", strings.Join(loadedSkills, ", "))

				// Get full file content from GitHub
				fullContent, _ := gh.GetFileContent(owner, repo, prInfo.HeadSHA, file.Filename)

				// Build prompt for single file review
				sysPrompt := engine.ReadSystemPrompt([]string{file.Filename}, file.Patch)
				if skillPrompt != "" {
					sysPrompt += "\n\n" + skillPrompt
				}
				sysPrompt += engine.StructuredOutputInstruction

				userPrompt := fmt.Sprintf(`Review ONLY this single file from PR #%d.

## File: %s (+%d, -%d)

### Diff:
`+"```diff"+`
%s
`+"```"+`

### Full File Content:
`+"```"+`
%s
`+"```"+`

Review against loaded skills. For each finding:
1. Identify the exact line number in the NEW version of the file
2. Classify severity: CRITICAL / IMPORTANT / NIT / QUESTION
3. Provide a concrete fix as a code suggestion (the exact replacement code for the problematic lines)

Keep it focused — this is one file, not the whole PR.`,
					prNumber, file.Filename, file.Additions, file.Deletions,
					file.Patch, fullContent)

				s.AddMessage("developer", userPrompt, round, nil)
				messages := s.GetConversationForLLM()

				// Only send last 2 messages to keep tokens low (per-file review)
				if len(messages) > 2 {
					messages = messages[len(messages)-2:]
				}

				response, err := engine.CallClaude(sysPrompt, messages)
				if err != nil {
					color.Red("  Review failed: %s", err)
					continue
				}

				// Parse findings
				humanResponse := engine.ParseStructuredOutput(response, s, round)
				s.AddMessage("agent", humanResponse, round, nil)

				// Count new findings for this file
				newFindings := 0
				for fid, f := range s.Findings {
					id, _ := strconv.Atoi(fid)
					if id > findingCounter {
						newFindings++
						// Build inline comment
						comment := gh.InlineComment{
							Path: file.Filename,
							Line: f.Line,
							Body: gh.FormatSuggestion(f.Severity, f.Title, f.Problem, f.SuggestedFix, f.FixRationale),
						}
						allComments = append(allComments, comment)
					}
				}
				findingCounter = len(s.Findings)

				// Print findings for this file
				fmt.Println(humanResponse)
				if newFindings > 0 {
					fmt.Printf("\n  Found: %d finding(s) for %s\n", newFindings, file.Filename)
				} else {
					color.Green("  No findings for %s\n", file.Filename)
				}

				// Save state after each file
				state.Save(s, "local")
			}

			// Post to PR if --post flag
			if post && len(allComments) > 0 {
				step.Println("\n[reviewiq] Posting review to PR...")

				summary := fmt.Sprintf("## ReviewIQ Review\n\n"+
					"**%d findings** across %d files | Assessment: **%s**\n\n"+
					"| Severity | Count |\n|---|---|\n"+
					"| CRITICAL | %d |\n| IMPORTANT | %d |\n| NIT | %d |",
					s.Summary.TotalFindings, len(files), s.Summary.Assessment,
					countBySeverity(s, "CRITICAL"),
					countBySeverity(s, "IMPORTANT"),
					countBySeverity(s, "NIT"))

				event := "COMMENT"
				if s.Summary.Open > 0 {
					event = "REQUEST_CHANGES"
				}

				if err := gh.PostReview(owner, repo, prNumber, prInfo.HeadSHA, summary, event, allComments); err != nil {
					color.Red("Failed to post review: %s", err)
					// Fall back to posting as regular comment
					if err2 := gh.PostPRComment(owner, repo, prNumber, summary+"\n\n_(inline comments failed, findings in state file)_"); err2 != nil {
						color.Red("Fallback comment also failed: %s", err2)
					}
				} else {
					color.Green("Review posted to PR #%d with %d inline comments", prNumber, len(allComments))
				}
			} else if post {
				color.Green("No findings to post.")
			}

			// Final summary
			fmt.Println()
			bold.Println("Review Complete")
			fmt.Printf("Total: %d findings | Open: %d | Assessment: %s\n",
				s.Summary.TotalFindings, s.Summary.Open, s.Summary.Assessment)
			fmt.Printf("State: .pr-review/reviews/pr-%d.json\n", prNumber)

			if !post && len(allComments) > 0 {
				fmt.Printf("\nTo post findings to PR: reviewiq pr %s --post\n", input)
			}

			state.Save(s, "local")
		},
	}
	cmd.Flags().BoolVar(&post, "post", false, "Post findings as inline comments on the PR")
	return cmd
}

func reviewFullCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "full <pr-link-or-number>",
		Short: "Full PR review in one shot — review all files, post comments, suggestions, and resolutions",
		Long: `Reviews the entire PR diff at once using all relevant skills,
then posts a complete review with inline comments, suggestion
blocks, and resolution recommendations on the PR.

Unlike review-pr (file-by-file interactive), this runs end-to-end
in one command with no user interaction.

Examples:
  reviewiq full https://github.com/owner/repo/pull/42
  reviewiq full 42`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			input := args[0]

			var owner, repo string
			var prNumber int

			if strings.Contains(input, "github.com") {
				var err error
				owner, repo, prNumber, err = gh.ParsePRLink(input)
				if err != nil {
					color.Red("%s", err)
					os.Exit(1)
				}
			} else {
				prNumber, _ = strconv.Atoi(input)
				if prNumber == 0 {
					color.Red("Invalid input. Provide a PR link or number.")
					os.Exit(1)
				}
				remote := gitops.Run("remote", "get-url", "origin")
				re := regexp.MustCompile(`github\.com[:/]([^/]+)/([^/.]+)`)
				m := re.FindStringSubmatch(remote)
				if m == nil {
					color.Red("Cannot detect GitHub repo from remote. Use full PR link.")
					os.Exit(1)
				}
				owner, repo = m[1], m[2]
			}

			bold := color.New(color.Bold)
			step := color.New(color.FgCyan)

			// ── Fetch PR ────────────────────────────────────────────────
			step.Printf("[reviewiq] Fetching PR #%d from %s/%s...\n", prNumber, owner, repo)

			prInfo, err := gh.GetPR(owner, repo, prNumber)
			if err != nil {
				color.Red("Failed to fetch PR: %s", err)
				os.Exit(1)
			}

			files, err := gh.GetPRFiles(owner, repo, prNumber)
			if err != nil {
				color.Red("Failed to fetch PR files: %s", err)
				os.Exit(1)
			}

			fmt.Println()
			bold.Printf("PR #%d: %s\n", prNumber, prInfo.Title)
			fmt.Printf("Author: @%s | Branch: %s → %s | Files: %d\n\n",
				prInfo.Author, prInfo.HeadBranch, prInfo.BaseBranch, len(files))

			// ── Build full diff + detect all skills ─────────────────────
			step.Println("[reviewiq] Assembling full diff and detecting skills...")

			var fullDiff strings.Builder
			var allFilenames []string
			var allPatches strings.Builder
			totalAdded, totalRemoved := 0, 0

			for _, f := range files {
				allFilenames = append(allFilenames, f.Filename)
				totalAdded += f.Additions
				totalRemoved += f.Deletions
				if f.Patch != "" {
					fullDiff.WriteString(fmt.Sprintf("--- a/%s\n+++ b/%s\n%s\n\n", f.Filename, f.Filename, f.Patch))
					allPatches.WriteString(f.Patch + "\n")
				}
			}

			// Detect skills across ALL files at once
			detected := skills.Detect(allFilenames, allPatches.String())
			skillPrompt := skills.LoadSkills(detected)

			var loadedSkills []string
			loadedSkills = append(loadedSkills, detected.Always...)
			loadedSkills = append(loadedSkills, detected.Languages...)
			loadedSkills = append(loadedSkills, detected.Frameworks...)
			loadedSkills = append(loadedSkills, detected.DevOps...)
			loadedSkills = append(loadedSkills, detected.Domains...)
			fmt.Printf("Skills loaded: %s\n", strings.Join(loadedSkills, ", "))
			fmt.Printf("Diff size: +%d -%d across %d files\n\n", totalAdded, totalRemoved, len(files))

			// ── Read full file contents ─────────────────────────────────
			step.Println("[reviewiq] Reading full file contents...")
			var fileContents strings.Builder
			for _, f := range files {
				if f.Patch == "" {
					continue
				}
				content, err := gh.GetFileContent(owner, repo, prInfo.HeadSHA, f.Filename)
				if err != nil {
					continue
				}
				fileContents.WriteString(fmt.Sprintf("--- FILE: %s ---\n%s\n--- END: %s ---\n\n", f.Filename, content, f.Filename))
			}

			// ── Load state ──────────────────────────────────────────────
			s := state.Load(prNumber, fmt.Sprintf("%s/%s", owner, repo), "auto")
			s.PR.Title = prInfo.Title
			s.PR.Author = prInfo.Author
			s.PR.BaseBranch = prInfo.BaseBranch
			s.PR.HeadBranch = prInfo.HeadBranch
			s.PR.Repo = fmt.Sprintf("%s/%s", owner, repo)

			round := len(s.ReviewRounds) + 1
			s.ReviewRounds = append(s.ReviewRounds, state.NewReviewRound(
				round, prInfo.HeadSHA, prInfo.BaseSHA, "reviewiq-full", allFilenames,
			))
			s.Summary.LastReviewedSHA = prInfo.HeadSHA

			// ── Run full review via Claude API ──────────────────────────
			step.Println("[reviewiq] Running full review...")

			sysPrompt := engine.ReadSystemPrompt(allFilenames, allPatches.String())
			if skillPrompt != "" {
				sysPrompt += "\n\n" + skillPrompt
			}
			sysPrompt += engine.StructuredOutputInstruction

			// Add cross-file analysis instruction
			sysPrompt += `

## CROSS-FILE ANALYSIS

This is a full PR review (all files at once). In addition to per-file checks:
1. Check if changes in one file break assumptions in another
2. Verify consistent error handling across all changed files
3. Check for missing changes (e.g., updated model but not the migration)
4. Verify test coverage for all changed functionality
5. For each finding, specify the EXACT file path and line number

Format each finding's suggested_fix as the exact replacement code that should go
inside a GitHub suggestion block — it will be posted as an inline PR comment.
`

			userPrompt := fmt.Sprintf(`Review this entire PR in one pass.

## PR #%d: %s
Author: @%s | Branch: %s → %s

## Full Diff
`+"```diff"+`
%s
`+"```"+`

## Full File Contents
%s

## Instructions
1. Review ALL files against the loaded skills
2. Cross-file analysis: check for inconsistencies between files
3. For each finding: exact file path, exact line number, severity, concrete fix
4. Each suggested_fix must be the exact replacement code (for GitHub suggestion blocks)
5. End with overall assessment and key risks

This is review round %d.`,
				prNumber, prInfo.Title, prInfo.Author, prInfo.HeadBranch, prInfo.BaseBranch,
				fullDiff.String(), fileContents.String(), round)

			s.AddMessage("system", fmt.Sprintf("Full PR review round %d.", round), round, nil)
			s.AddMessage("developer", userPrompt, round, nil)

			messages := s.GetConversationForLLM()

			response, err := engine.CallClaude(sysPrompt, messages)
			if err != nil {
				color.Red("Review failed: %s", err)
				os.Exit(1)
			}

			humanResponse := engine.ParseStructuredOutput(response, s, round)
			s.AddMessage("agent", humanResponse, round, nil)

			// Print the review
			fmt.Println(humanResponse)

			// ── Build inline comments ───────────────────────────────────
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

			// ── Post to PR ──────────────────────────────────────────────
			if len(comments) > 0 {
				step.Printf("\n[reviewiq] Posting review to PR #%d (%d inline comments)...\n", prNumber, len(comments))

				summaryBody := fmt.Sprintf("## ReviewIQ Full Review\n\n"+
					"**%d findings** across %d files (+%d, -%d) | Assessment: **%s**\n\n"+
					"| Severity | Count |\n|---|---|\n"+
					"| CRITICAL | %d |\n| IMPORTANT | %d |\n| NIT | %d |\n| QUESTION | %d |\n\n"+
					"Skills used: %s\n\n"+
					"_Each finding is posted as an inline comment with a `suggestion` block — click \"Apply suggestion\" to fix._",
					s.Summary.TotalFindings, len(files), totalAdded, totalRemoved, s.Summary.Assessment,
					countBySeverity(s, "CRITICAL"), countBySeverity(s, "IMPORTANT"),
					countBySeverity(s, "NIT"), countBySeverity(s, "QUESTION"),
					strings.Join(loadedSkills, ", "))

				event := "COMMENT"
				if countBySeverity(s, "CRITICAL") > 0 || countBySeverity(s, "IMPORTANT") > 0 {
					event = "REQUEST_CHANGES"
				} else if s.Summary.Open == 0 {
					event = "APPROVE"
				}

				if err := gh.PostReview(owner, repo, prNumber, prInfo.HeadSHA, summaryBody, event, comments); err != nil {
					color.Red("Failed to post review: %s", err)
					// Fallback: post as regular comment
					if err2 := gh.PostPRComment(owner, repo, prNumber, summaryBody+"\n\n"+humanResponse); err2 != nil {
						color.Red("Fallback also failed: %s", err2)
					} else {
						color.Yellow("Posted as regular comment (inline comments failed)")
					}
				} else {
					color.Green("Review posted to PR #%d: %d inline comments + summary", prNumber, len(comments))
				}
			} else {
				color.Green("No findings — PR looks good!")
				if err := gh.PostPRComment(owner, repo, prNumber, "## ReviewIQ\n\nNo findings. LGTM!"); err != nil {
					color.Red("Failed to post comment: %s", err)
				}
			}

			// ── Save state and print summary ────────────────────────────
			state.Save(s, "both")

			fmt.Println()
			bold.Println("Review Complete")
			fmt.Printf("Findings: %d (CRITICAL: %d, IMPORTANT: %d, NIT: %d)\n",
				s.Summary.TotalFindings,
				countBySeverity(s, "CRITICAL"), countBySeverity(s, "IMPORTANT"), countBySeverity(s, "NIT"))
			fmt.Printf("Assessment: %s\n", s.Summary.Assessment)
			fmt.Printf("State: .pr-review/reviews/pr-%d.json\n", prNumber)
		},
	}
	return cmd
}

func countBySeverity(s *state.ReviewState, severity string) int {
	count := 0
	for _, f := range s.Findings {
		if f.Severity == severity && (f.Status == "open" || f.Status == "partially_fixed") {
			count++
		}
	}
	return count
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
