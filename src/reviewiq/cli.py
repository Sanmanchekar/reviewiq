from __future__ import annotations

"""
ReviewIQ CLI — Stateful PR review agent from the command line.

Usage:
    reviewiq review <branch>          Full review of a PR branch
    reviewiq check <branch>           Incremental re-review after new commits
    reviewiq status [--pr N]          Show current finding statuses
    reviewiq explain <finding-id>     Deep dive into a specific finding
    reviewiq ask <question>           Ask a follow-up question
    reviewiq retract <finding-id>     Retract a finding (agent was wrong)
    reviewiq wontfix <finding-id>     Mark finding as won't fix
    reviewiq resolve <finding-id>     Mark finding as resolved
    reviewiq approve [--pr N]         Final check for remaining blockers
    reviewiq init                     Initialize .pr-review/ in current repo
    reviewiq ci                       Run in CI mode (reads env vars)
"""

import argparse
import json
import os
import re
import sys
from pathlib import Path

from reviewiq import __version__
from reviewiq import state as st
from reviewiq import git
from reviewiq import engine


def _detect_pr_number(branch: str) -> int:
    """Try to detect PR number from branch name or git."""
    # Try extracting from branch name patterns like pr-42, pull/42, etc.
    match = re.search(r"(?:pr-?|pull/)(\d+)", branch)
    if match:
        return int(match.group(1))

    # Use a hash of the branch name as a stable pseudo-PR number for local use
    return abs(hash(branch)) % 100000


def _find_existing_state(pr_number: int | None = None) -> tuple[int, dict] | None:
    """Find existing state file, optionally for a specific PR."""
    state_dir = Path(".pr-review/reviews")
    if not state_dir.exists():
        return None

    if pr_number:
        state = st.load_local(pr_number)
        if state:
            return pr_number, state
        return None

    # Find most recently modified state file
    state_files = sorted(state_dir.glob("pr-*.json"), key=lambda p: p.stat().st_mtime, reverse=True)
    if not state_files:
        return None

    match = re.search(r"pr-(\d+)\.json", state_files[0].name)
    if match:
        pr_num = int(match.group(1))
        state = st.load_local(pr_num)
        if state:
            return pr_num, state

    return None


# ── Commands ─────────────────────────────────────────────────────────────────

def cmd_init(args: argparse.Namespace) -> None:
    """Initialize .pr-review/ directory with agent.md template."""
    pr_review_dir = Path(".pr-review")
    agent_file = pr_review_dir / "agent.md"

    if agent_file.exists():
        print("Already initialized: .pr-review/agent.md exists")
        return

    pr_review_dir.mkdir(parents=True, exist_ok=True)

    # Write the agent protocol template
    template_path = Path(__file__).parent / "templates" / "agent.md"
    if template_path.exists():
        agent_file.write_text(template_path.read_text())
    else:
        agent_file.write_text(_DEFAULT_AGENT_MD)

    # Add reviews/ to gitignore if not already there
    gitignore = Path(".gitignore")
    gitignore_line = ".pr-review/reviews/"
    if gitignore.exists():
        content = gitignore.read_text()
        if gitignore_line not in content:
            with open(gitignore, "a") as f:
                f.write(f"\n# ReviewIQ state files\n{gitignore_line}\n")
    else:
        gitignore.write_text(f"# ReviewIQ state files\n{gitignore_line}\n")

    print("Initialized ReviewIQ:")
    print("  .pr-review/agent.md    — review protocol (customize this)")
    print("  .gitignore             — updated to exclude state files")
    print()
    print("Next: reviewiq review <branch>")


def cmd_review(args: argparse.Namespace) -> None:
    """Run a full review of a PR branch."""
    branch = args.branch
    base = args.base or git.get_base_branch()

    print(f"Reviewing {branch} against {base}...")

    diff = git.get_diff(base, branch)
    if not diff:
        print(f"No diff found between {base} and {branch}.")
        sys.exit(1)

    changed_files = git.get_changed_files(base, branch)
    file_contents = git.read_files(changed_files)
    history = git.get_file_history(changed_files)
    head_sha = git.run_git("rev-parse", branch)
    base_sha = git.run_git("rev-parse", base)

    pr_number = args.pr or _detect_pr_number(branch)
    review_state = st.load(pr_number, prefer="local")

    review_state["pr"].update({
        "title": f"Review of {branch}",
        "author": git.run_git("config", "user.name") or "local",
        "base_branch": base,
        "head_branch": branch,
    })

    response = engine.run_review(
        review_state,
        diff=diff,
        file_contents=file_contents,
        history=history,
        changed_files=changed_files,
        head_sha=head_sha,
        base_sha=base_sha,
        pr_title=review_state["pr"]["title"],
        pr_author=review_state["pr"]["author"],
        base_branch=base,
        head_branch=branch,
    )

    st.save(review_state, targets="local")
    print(response)
    print(f"\nState saved: .pr-review/reviews/pr-{pr_number}.json")
    print(f"Findings: {review_state['summary']['total_findings']} "
          f"({review_state['summary']['open']} open)")


def cmd_check(args: argparse.Namespace) -> None:
    """Incremental re-review after new commits."""
    branch = args.branch
    base = args.base or git.get_base_branch()

    pr_number = args.pr or _detect_pr_number(branch)
    result = _find_existing_state(pr_number)
    if not result:
        print(f"No existing review state found for PR {pr_number}.")
        print("Run 'reviewiq review <branch>' first.")
        sys.exit(1)

    pr_number, review_state = result
    print(f"Re-reviewing {branch} (round {len(review_state['review_rounds']) + 1})...")

    diff = git.get_diff(base, branch)
    changed_files = git.get_changed_files(base, branch)
    file_contents = git.read_files(changed_files)
    head_sha = git.run_git("rev-parse", branch)
    base_sha = git.run_git("rev-parse", base)
    incremental_diff = git.get_incremental_diff(review_state, head_sha)

    response = engine.run_check(
        review_state,
        diff=diff,
        file_contents=file_contents,
        changed_files=changed_files,
        head_sha=head_sha,
        base_sha=base_sha,
        incremental_diff=incremental_diff,
        pr_title=review_state["pr"]["title"],
        pr_author=review_state["pr"]["author"],
        base_branch=base,
        head_branch=branch,
    )

    st.save(review_state, targets="local")
    print(response)
    print(f"\nOpen: {review_state['summary']['open']} | "
          f"Resolved: {review_state['summary']['resolved']} | "
          f"Assessment: {review_state['summary']['assessment']}")


def cmd_status(args: argparse.Namespace) -> None:
    """Show current finding statuses."""
    result = _find_existing_state(args.pr)
    if not result:
        print("No review state found. Run 'reviewiq review <branch>' first.")
        sys.exit(1)

    pr_number, review_state = result
    s = review_state["summary"]

    print(f"ReviewIQ Status — PR #{pr_number} (Round {len(review_state['review_rounds'])})")
    print(f"Assessment: {s['assessment']}")
    print()

    if not review_state["findings"]:
        print("No findings.")
        return

    # Table header
    print(f"{'#':<4} {'Severity':<12} {'Status':<16} {'Title':<40} {'File'}")
    print("-" * 100)

    for fid in sorted(review_state["findings"].keys(), key=int):
        f = review_state["findings"][fid]
        status_display = f["status"].upper()
        print(f"{f['id']:<4} {f['severity']:<12} {status_display:<16} "
              f"{f['title'][:40]:<40} {f['file']}:{f['line']}")

    print()
    print(f"Total: {s['total_findings']} | Open: {s['open']} | "
          f"Resolved: {s['resolved']} | Won't fix: {s['wontfix']} | "
          f"Retracted: {s['retracted']}")


def cmd_explain(args: argparse.Namespace) -> None:
    """Deep dive into a specific finding."""
    result = _find_existing_state(args.pr)
    if not result:
        print("No review state found.")
        sys.exit(1)

    pr_number, review_state = result
    finding = st.get_finding(review_state, args.finding_id)
    if not finding:
        print(f"Finding {args.finding_id} not found.")
        sys.exit(1)

    # Read the file referenced by the finding
    file_contents = git.read_files([finding["file"]])
    question = f"explain finding {args.finding_id}"

    response = engine.run_ask(review_state, question, file_contents, finding_id=args.finding_id)
    st.save(review_state, targets="local")
    print(response)


def cmd_ask(args: argparse.Namespace) -> None:
    """Ask a follow-up question."""
    result = _find_existing_state(args.pr)
    if not result:
        print("No review state found.")
        sys.exit(1)

    pr_number, review_state = result
    question = " ".join(args.question)

    # Detect if the question references a finding
    finding_id = None
    finding_match = re.search(r"(?:finding|#)\s*(\d+)", question, re.IGNORECASE)
    if finding_match:
        finding_id = int(finding_match.group(1))

    # Read changed files for context
    base = git.get_base_branch()
    branch = review_state["pr"]["head_branch"] or git.get_current_branch()
    changed_files = git.get_changed_files(base, branch)
    file_contents = git.read_files(changed_files)

    response = engine.run_ask(review_state, question, file_contents, finding_id=finding_id)
    st.save(review_state, targets="local")
    print(response)


def cmd_transition(args: argparse.Namespace) -> None:
    """Transition a finding's status (resolve, retract, wontfix)."""
    result = _find_existing_state(args.pr)
    if not result:
        print("No review state found.")
        sys.exit(1)

    pr_number, review_state = result
    finding = st.get_finding(review_state, args.finding_id)
    if not finding:
        print(f"Finding {args.finding_id} not found.")
        sys.exit(1)

    note = args.note or ""
    round_number = len(review_state["review_rounds"])

    try:
        st.transition_finding(review_state, args.finding_id, args.status, note, round_number)
    except ValueError as e:
        print(f"Error: {e}")
        sys.exit(1)

    st.save(review_state, targets="local")
    print(f"Finding {args.finding_id}: {finding['status']} -> {args.status}")
    if note:
        print(f"Note: {note}")
    print(f"Assessment: {review_state['summary']['assessment']}")


def cmd_approve(args: argparse.Namespace) -> None:
    """Final check — any blockers remaining?"""
    result = _find_existing_state(args.pr)
    if not result:
        print("No review state found.")
        sys.exit(1)

    pr_number, review_state = result
    open_findings = st.get_open_findings(review_state)

    blockers = [f for f in open_findings if f["severity"] in ("CRITICAL", "IMPORTANT")]
    nits = [f for f in open_findings if f["severity"] not in ("CRITICAL", "IMPORTANT")]

    if blockers:
        print("BLOCKED — the following findings must be addressed:\n")
        for f in blockers:
            print(f"  [{f['severity']}] Finding {f['id']}: {f['title']}")
            print(f"    {f['file']}:{f['line']} — {f['problem'][:80]}")
            print()
        if nits:
            print(f"Plus {len(nits)} non-blocking nit(s).")
    elif nits:
        print("APPROVE with nits:\n")
        for f in nits:
            print(f"  [NIT] Finding {f['id']}: {f['title']}")
        print("\nNo blockers. Safe to merge.")
    else:
        print("APPROVE — no remaining findings. Safe to merge.")
        review_state["summary"]["assessment"] = "APPROVE"
        st.save(review_state, targets="local")


def cmd_ci(args: argparse.Namespace) -> None:
    """Run in CI mode — reads event context from environment variables."""
    # Import the CI-specific module
    repo = os.environ.get("GITHUB_REPOSITORY", "")
    pr_number = int(os.environ.get("PR_NUMBER", "0"))
    event_type = os.environ.get("EVENT_TYPE", "")
    comment_body = os.environ.get("COMMENT_BODY", "")

    if not all([repo, pr_number, event_type]):
        print("CI mode requires GITHUB_REPOSITORY, PR_NUMBER, EVENT_TYPE env vars.", file=sys.stderr)
        sys.exit(1)

    # Lazy import to avoid requiring GitHub env vars for local usage
    from reviewiq import ci
    ci.run(repo, pr_number, event_type, comment_body)


# ── Default agent.md template ────────────────────────────────────────────────

_DEFAULT_AGENT_MD = """# PR Review Agent

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
"""


# ── Argument Parser ──────────────────────────────────────────────────────────

def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="reviewiq",
        description="Stateful AI-powered PR review agent",
    )
    parser.add_argument("-V", "--version", action="version", version=f"reviewiq {__version__}")

    sub = parser.add_subparsers(dest="command", help="Available commands")

    # init
    sub.add_parser("init", help="Initialize .pr-review/ in current repo")

    # review
    p = sub.add_parser("review", help="Full review of a PR branch")
    p.add_argument("branch", help="Branch to review")
    p.add_argument("--base", help="Base branch (default: auto-detect)")
    p.add_argument("--pr", type=int, help="PR number (default: auto-detect)")

    # check
    p = sub.add_parser("check", help="Incremental re-review after new commits")
    p.add_argument("branch", help="Branch to re-review")
    p.add_argument("--base", help="Base branch (default: auto-detect)")
    p.add_argument("--pr", type=int, help="PR number")

    # status
    p = sub.add_parser("status", help="Show current finding statuses")
    p.add_argument("--pr", type=int, help="PR number (default: most recent)")

    # explain
    p = sub.add_parser("explain", help="Deep dive into a specific finding")
    p.add_argument("finding_id", type=int, help="Finding number")
    p.add_argument("--pr", type=int, help="PR number")

    # ask
    p = sub.add_parser("ask", help="Ask a follow-up question")
    p.add_argument("question", nargs="+", help="Your question")
    p.add_argument("--pr", type=int, help="PR number")

    # resolve
    p = sub.add_parser("resolve", help="Mark a finding as resolved")
    p.add_argument("finding_id", type=int, help="Finding number")
    p.add_argument("--note", "-n", help="Resolution note")
    p.add_argument("--pr", type=int, help="PR number")

    # retract
    p = sub.add_parser("retract", help="Retract a finding (agent was wrong)")
    p.add_argument("finding_id", type=int, help="Finding number")
    p.add_argument("--note", "-n", help="Reason for retraction")
    p.add_argument("--pr", type=int, help="PR number")

    # wontfix
    p = sub.add_parser("wontfix", help="Mark a finding as won't fix")
    p.add_argument("finding_id", type=int, help="Finding number")
    p.add_argument("--note", "-n", help="Reason")
    p.add_argument("--pr", type=int, help="PR number")

    # approve
    p = sub.add_parser("approve", help="Final check for remaining blockers")
    p.add_argument("--pr", type=int, help="PR number")

    # ci
    sub.add_parser("ci", help="Run in CI mode (GitHub Actions)")

    return parser


def main() -> None:
    parser = build_parser()
    args = parser.parse_args()

    if not args.command:
        parser.print_help()
        sys.exit(0)

    commands = {
        "init": cmd_init,
        "review": cmd_review,
        "check": cmd_check,
        "status": cmd_status,
        "explain": cmd_explain,
        "ask": cmd_ask,
        "approve": cmd_approve,
        "ci": cmd_ci,
    }

    # Handle transition commands
    if args.command in ("resolve", "retract", "wontfix"):
        status_map = {"resolve": "resolved", "retract": "retracted", "wontfix": "wontfix"}
        args.status = status_map[args.command]
        cmd_transition(args)
        return

    handler = commands.get(args.command)
    if handler:
        handler(args)
    else:
        parser.print_help()


if __name__ == "__main__":
    main()
