package main

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

	root.AddCommand(initCmd(), initGlobalCmd(), reviewCmd(), reviewPRCmd(), reviewFullCmd(),
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

func detectRepo() string {
	remote := gitops.Run("remote", "get-url", "origin")
	re := regexp.MustCompile(`github\.com[:/]([^/]+)/([^/.]+)`)
	m := re.FindStringSubmatch(remote)
	if m == nil {
		return ""
	}
	return m[1] + "/" + m[2]
}

func detectPRFromBranch() int {
	out, err := exec.Command("gh", "pr", "view", "--json", "number", "-q", ".number").Output()
	if err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(string(out))); err == nil {
			return n
		}
	}
	return 0
}

func parseInput(input string) (owner, repo string, prNumber int) {
	if strings.Contains(input, "github.com") {
		var err error
		owner, repo, prNumber, err = gh.ParsePRLink(input)
		if err != nil {
			color.Red("%s", err)
			os.Exit(1)
		}
		return
	}
	prNumber, _ = strconv.Atoi(input)
	if prNumber == 0 {
		color.Red("Invalid input. Provide a PR link or number.")
		os.Exit(1)
	}
	fullRepo := detectRepo()
	if fullRepo == "" {
		color.Red("Cannot detect GitHub repo from remote. Use full PR link.")
		os.Exit(1)
	}
	parts := strings.SplitN(fullRepo, "/", 2)
	owner, repo = parts[0], parts[1]
	return
}

func loadState(prNumber int, repo string) *state.ReviewState {
	return state.Load(prNumber, repo)
}

// ── Commands ────────────────────────────────────────────────────────────────

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize .pr-review/ in current repo (skills + agent protocol)",
		Run: func(cmd *cobra.Command, args []string) {
			created := 0

			// .pr-review/agent.md
			agentFile := filepath.Join(".pr-review", "agent.md")
			os.MkdirAll(filepath.Join(".pr-review", "skills"), 0o755)
			if _, err := os.Stat(agentFile); err != nil {
				os.WriteFile(agentFile, []byte(defaultAgentMD), 0o644)
				created++
			}

			// Clean up old per-repo slash commands (now installed globally)
			claudeDir := filepath.Join(".claude", "commands")
			for _, pattern := range []string{"review-*.md", "reviewiq-*.md"} {
				oldFiles, _ := filepath.Glob(filepath.Join(claudeDir, pattern))
				for _, old := range oldFiles {
					os.Remove(old)
					fmt.Printf("  Removed per-repo: %s (now global)\n", filepath.Base(old))
				}
			}

			// Clean up old .gitignore entry for local state (no longer used)
			if data, err := os.ReadFile(".gitignore"); err == nil {
				cleaned := strings.Replace(string(data), "\n# ReviewIQ state files\n.pr-review/reviews/\n", "\n", 1)
				if cleaned != string(data) {
					os.WriteFile(".gitignore", []byte(cleaned), 0o644)
				}
			}

			if created == 0 {
				fmt.Println("Already initialized. All files exist.")
			} else {
				color.Green("Initialized ReviewIQ:\n")
				fmt.Println("  .pr-review/agent.md              — review protocol")
				fmt.Println("  .pr-review/skills/               — add skill .md files here")
			}
			fmt.Println("\nSlash commands are installed globally (~/.claude/commands/).")
			fmt.Println("Run 'reviewiq init-global' to reinstall them.")
		},
	}
}

func initGlobalCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init-global",
		Short: "Install slash commands globally to ~/.claude/commands/",
		Run: func(cmd *cobra.Command, args []string) {
			home, err := os.UserHomeDir()
			if err != nil {
				color.Red("Cannot detect home directory: %s", err)
				os.Exit(1)
			}
			globalDir := filepath.Join(home, ".claude", "commands")
			os.MkdirAll(globalDir, 0o755)

			for name, content := range claudeCommands {
				path := filepath.Join(globalDir, name+".md")
				os.WriteFile(path, []byte(content), 0o644)
			}

			// Clean up old review-*.md files
			oldFiles, _ := filepath.Glob(filepath.Join(globalDir, "review-*.md"))
			for _, old := range oldFiles {
				os.Remove(old)
			}

			color.Green("Installed %d slash commands globally:", len(claudeCommands))
			for name := range claudeCommands {
				fmt.Printf("  /%-24s ~/.claude/commands/%s.md\n", name, name)
			}
			fmt.Println("\nAvailable in every repo — no per-repo init needed.")
		},
	}
}

var claudeCommands = map[string]string{
	"reviewiq-pr": `Review PR: $ARGUMENTS

$ARGUMENTS: PR link, PR number, or branch + optional flag (--full or --interactive).

/reviewiq-pr 42                     # --full (default): all files, auto-post
/reviewiq-pr 42 --full              # same, explicit
/reviewiq-pr 42 --interactive       # file-by-file, post/skip per file

## Steps
1. Detect repo: ` + "`gh repo view --json nameWithOwner -q .nameWithOwner`" + ` — use this EXACT string everywhere
2. Fetch PR data + head SHA via gh CLI
3. Load existing state from GitHub PR hidden comment (look for ` + "`<!-- REVIEWIQ_STATE_COMMENT -->`" + `) or start fresh as round 1
4. Load relevant skills from ~/.reviewiq/skills/ or .pr-review/skills/
5. --full (default): review all files, cross-file analysis, auto-post inline comments + report
6. --interactive: review per file, show findings, ask P(post)/S(skip)/F(fix), auto-skip if no findings
7. Save state to GitHub PR hidden comment (create or update the ` + "`<!-- REVIEWIQ_STATE_COMMENT -->`" + ` comment)
`,
	"reviewiq-recheck": `Re-review PR with history: $ARGUMENTS

$ARGUMENTS: PR link, PR number, or branch.

## Steps
1. Detect repo: ` + "`gh repo view --json nameWithOwner -q .nameWithOwner`" + `
2. Load state from GitHub PR hidden comment (` + "`<!-- REVIEWIQ_STATE_COMMENT -->`" + `)
3. Increment round number: new round = count of previous rounds + 1
4. Fetch current code, compare against last reviewed SHA in state
5. For each pending finding:
   - Code fixed? → auto-resolve, update status history
   - Still broken? → keep pending
   - Changed differently? → mark needs-review
6. Check new changes for NEW issues — assign next finding IDs
7. Post new round report as PR comment (append to timeline, never overwrite previous rounds)
8. Save updated state to GitHub PR hidden comment
`,
	"reviewiq-resolve": `Resolve all findings and approve PR: $ARGUMENTS

$ARGUMENTS: PR link, PR number, or branch.

## IMPORTANT: This command APPLIES fixes, it does NOT just verify them.

You MUST edit the actual source files to fix each finding. Do NOT just check if fixes exist.

## Steps
1. Detect repo: ` + "`REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner)`" + `
2. Get PR info: ` + "`gh pr view <N> --repo $REPO --json headRefOid,headRefName,baseRefName`" + `
3. **Checkout the PR branch**: ` + "`gh pr checkout <N>`" + ` — you must be on the PR branch to push fixes
4. Load state from GitHub PR hidden comment (` + "`<!-- REVIEWIQ_STATE_COMMENT -->`" + `)
   - If no state comment exists, check conversation history for findings from prior review
5. For EACH pending/open finding:
   a. Read the target file using the Read tool
   b. Locate the problematic code at the finding's line number
   c. **EDIT the file** — use the Edit tool to apply the suggested_fix as the replacement code
   d. After editing, verify the fix is syntactically correct
6. **RUN TESTS** after all fixes are applied:
   a. Detect the project's test framework (pytest, go test, npm test, etc.)
   b. Run the test suite: focus on files that were modified
   c. If tests fail: diagnose, fix the issue, re-run until green
   d. If no test framework exists: run linter/type-checker (flake8, mypy, eslint, tsc, etc.)
   e. If no linter either: at minimum run syntax checks (py_compile, node --check, etc.)
7. **COMMIT AND PUSH the fixes**:
   a. ` + "`git add`" + ` all modified files
   b. ` + "`git commit -m \"fix: resolve ReviewIQ findings for PR #<N>\"`" + `
   c. ` + "`git push`" + ` to the PR branch
   d. This is MANDATORY — without push, the PR branch still has the old broken code
8. After push:
   a. Save updated state to GitHub PR hidden comment (mark all as resolved)
   b. Post resolution report as PR comment listing every fix applied + test results
   c. Approve PR: ` + "`gh pr review <N> --repo $REPO --approve --body \"All findings resolved and tests passing — ReviewIQ\"`" + `

## Key behavior
- This command WRITES CODE. It edits files directly.
- Every finding with a suggested_fix gets applied via the Edit tool.
- If a suggested_fix is unclear, use your judgment to write the correct fix.
- Tests MUST pass before committing. If tests fail, fix until green.
- Fixes MUST be committed and pushed. Approval without push is meaningless.
- After all edits, show ` + "`git diff --stat`" + ` so the developer can review before push.
`,
	"reviewiq-test": `Run tests for PR changes: $ARGUMENTS

$ARGUMENTS: PR link, PR number, branch, or empty (uses current branch).

## Steps
1. Detect repo: ` + "`REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner)`" + `
2. Identify changed files:
   - If PR number given: ` + "`gh pr diff <N> --repo $REPO --name-only`" + `
   - If branch: ` + "`git diff --name-only main...HEAD`" + `
   - If empty: use current branch vs main/master
3. Detect test framework from project:
   - Python: pytest, unittest (look for pytest.ini, setup.cfg, pyproject.toml, conftest.py)
   - JavaScript/TypeScript: jest, vitest, mocha (look for package.json scripts)
   - Go: ` + "`go test`" + `
   - Java: maven/gradle test
   - Ruby: rspec, minitest
4. Run tests:
   a. First try targeted: only tests related to changed files
   b. If no targeted tests found, run full suite
   c. Also run linter/type-checker if available (flake8, mypy, eslint, tsc, etc.)
5. Report results:
   - List each test file run and pass/fail status
   - Show any failures with error output
   - If all pass: report green
   - If failures: show what failed and suggest fixes

## Test discovery patterns
- Python: ` + "`pytest <changed_dir>/`" + ` or ` + "`pytest tests/ -k <module_name>`" + `
- JS/TS: ` + "`npx jest --findRelatedTests <changed_files>`" + `
- Go: ` + "`go test ./path/to/changed/...`" + `
- Look for test files matching: test_*.py, *_test.go, *.test.ts, *.spec.ts

## Key behavior
- Always run tests from the project root
- Respect existing test configuration (pytest.ini, jest.config, etc.)
- If no tests exist for changed code, suggest what tests should be written
- Report results clearly — don't just say "tests passed", show what ran
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
			repo := detectRepo()
			if repo == "" {
				color.Red("Cannot detect GitHub repo from remote. State won't be saved.")
			}
			s := state.Load(pr, repo)
			s.PR.Repo = repo
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

			state.Save(s)
			fmt.Println(response)
			fmt.Printf("\n")
			color.Green("State saved to GitHub PR comment")
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
			if pr == 0 {
				pr = detectPRFromBranch()
			}
			repo := detectRepo()
			s := loadState(pr, repo)
			if len(s.ReviewRounds) == 0 {
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

			state.Save(s)
			fmt.Println(response)
			fmt.Printf("\nOpen: %d | Resolved: %d | Assessment: %s\n",
				s.Summary.Open, s.Summary.Resolved, s.Summary.Assessment)
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
			if pr == 0 {
				pr = detectPRFromBranch()
			}
			repo := detectRepo()
			s := loadState(pr, repo)
			if len(s.ReviewRounds) == 0 {
				color.Red("No review state found. Run 'reviewiq review <branch>' first.")
				os.Exit(1)
			}
			sm := s.Summary
			bold := color.New(color.Bold)
			bold.Printf("ReviewIQ Status — PR #%d (Round %d)\n", pr, len(s.ReviewRounds))
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
			if pr == 0 { pr = detectPRFromBranch() }
			s := loadState(pr, detectRepo())
			if len(s.ReviewRounds) == 0 {
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
			state.Save(s)
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
			if pr == 0 { pr = detectPRFromBranch() }
			s := loadState(pr, detectRepo())
			if len(s.ReviewRounds) == 0 {
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
			state.Save(s)
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
			if pr == 0 { pr = detectPRFromBranch() }
			s := loadState(pr, detectRepo())
			if len(s.ReviewRounds) == 0 {
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
			state.Save(s)
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
			if pr == 0 { pr = detectPRFromBranch() }
			s := loadState(pr, detectRepo())
			if len(s.ReviewRounds) == 0 {
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
				state.Save(s)
				// Actually approve on GitHub if repo is set
				if s.PR.Repo != "" && s.PR.Number > 0 {
					out, err := exec.Command("gh", "pr", "review", strconv.Itoa(s.PR.Number),
						"--repo", s.PR.Repo, "--approve",
						"--body", "All findings resolved — ReviewIQ").CombinedOutput()
					if err != nil {
						color.Yellow("GitHub approve failed: %s", strings.TrimSpace(string(out)))
					} else {
						color.Green("PR #%d approved on GitHub", s.PR.Number)
					}
				}
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
			owner, repo, prNumber := parseInput(input)

			bold := color.New(color.Bold)
			cyan := color.New(color.FgCyan)

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
			s := state.Load(prNumber, fmt.Sprintf("%s/%s", owner, repo))
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
				state.Save(s)
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
			fmt.Println("State: saved to GitHub PR comment")

			if !post && len(allComments) > 0 {
				fmt.Printf("\nTo post findings to PR: reviewiq pr %s --post\n", input)
			}

			state.Save(s)
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
			owner, repo, prNumber := parseInput(input)

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
			s := state.Load(prNumber, fmt.Sprintf("%s/%s", owner, repo))
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
			state.Save(s)

			fmt.Println()
			bold.Println("Review Complete")
			fmt.Printf("Findings: %d (CRITICAL: %d, IMPORTANT: %d, NIT: %d)\n",
				s.Summary.TotalFindings,
				countBySeverity(s, "CRITICAL"), countBySeverity(s, "IMPORTANT"), countBySeverity(s, "NIT"))
			fmt.Printf("Assessment: %s\n", s.Summary.Assessment)
			fmt.Println("State: saved to GitHub PR comment")
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
- resolve: Apply fixes to code, run tests, approve PR
- explain <N>: Deep dive into finding N
- status: Show current findings
- test: Run tests for changed files

## Rules
1. Never hallucinate file contents — always read the file
2. Concrete fixes only — every suggestion must be copy-pasteable
3. Match repo conventions
4. Engage with developer pushback — they know the codebase
`
