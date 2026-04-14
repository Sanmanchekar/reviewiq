from __future__ import annotations

"""Git and file operations for ReviewIQ."""

import subprocess
from pathlib import Path

from reviewiq import state as st


def run_git(*args: str) -> str:
    """Run a git command and return stdout."""
    result = subprocess.run(
        ["git", *args],
        capture_output=True, text=True, timeout=30,
    )
    return result.stdout.strip()


def get_base_branch() -> str:
    """Detect the base branch (main or master)."""
    ref = run_git("symbolic-ref", "refs/remotes/origin/HEAD")
    if ref:
        return ref.replace("refs/remotes/origin/", "")
    # Fallback: check if main or master exists
    for branch in ("main", "master"):
        result = run_git("rev-parse", "--verify", f"origin/{branch}")
        if result:
            return branch
    return "main"


def get_diff(base: str, head: str) -> str:
    """Get diff between two refs."""
    return run_git("diff", f"{base}...{head}")


def get_changed_files(base: str, head: str) -> list[str]:
    """Get list of changed files between two refs."""
    files = run_git("diff", "--name-only", f"{base}...{head}")
    return [f for f in files.split("\n") if f.strip()]


def read_files(file_list: list[str]) -> str:
    """Read full contents of files."""
    contents = []
    for filepath in file_list:
        path = Path(filepath)
        if path.exists() and path.is_file():
            try:
                text = path.read_text(errors="replace")
                contents.append(f"--- FILE: {filepath} ---\n{text}\n--- END: {filepath} ---")
            except Exception:
                contents.append(f"--- FILE: {filepath} --- (unreadable)")
    return "\n\n".join(contents)


def get_file_history(file_list: list[str]) -> str:
    """Get recent git history for changed files."""
    histories = []
    for filepath in file_list:
        hist = run_git("log", "-5", "--oneline", "--follow", "--", filepath)
        if hist:
            histories.append(f"--- HISTORY: {filepath} ---\n{hist}\n--- END HISTORY ---")
    return "\n\n".join(histories)


def get_current_sha() -> str:
    """Get current HEAD SHA."""
    return run_git("rev-parse", "HEAD")


def get_current_branch() -> str:
    """Get current branch name."""
    return run_git("rev-parse", "--abbrev-ref", "HEAD")


def get_incremental_diff(review_state: dict, head_sha: str) -> str | None:
    """Get diff since last review round, if available."""
    last_round = st.get_latest_round(review_state)
    if not last_round:
        return None
    last_sha = last_round["head_sha"]
    if last_sha == head_sha:
        return None
    diff = run_git("diff", f"{last_sha}...{head_sha}")
    return diff if diff else None
