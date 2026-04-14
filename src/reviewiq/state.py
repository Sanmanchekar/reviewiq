from __future__ import annotations

"""
ReviewIQ State Manager

Dual-backend state persistence:
  - CLI mode: local JSON files at .pr-review/reviews/pr-<N>.json
  - CI mode:  hidden comment on the PR (base64 JSON in HTML markers)

State tracks:
  - Findings with lifecycle (open -> resolved/wontfix/retracted)
  - Full conversation history (proper message pairs, not flat text)
  - Review rounds (which SHA was reviewed, what changed between rounds)
  - Per-finding discussion threads
"""

import json
import base64
import os
import re
import urllib.request
import urllib.error
from datetime import datetime, timezone
from pathlib import Path


# ── State Schema ─────────────────────────────────────────────────────────────

def new_state(pr_number: int, repo: str = "") -> dict:
    """Create a fresh state object for a PR."""
    return {
        "version": 2,
        "pr": {
            "number": pr_number,
            "repo": repo,
            "title": "",
            "author": "",
            "base_branch": "",
            "head_branch": "",
        },
        "review_rounds": [],
        "findings": {},
        "conversation": [],
        "summary": {
            "total_findings": 0,
            "open": 0,
            "resolved": 0,
            "wontfix": 0,
            "retracted": 0,
            "assessment": "PENDING",
            "last_reviewed_sha": "",
        },
    }


def new_finding(
    finding_id: int,
    title: str,
    severity: str,
    file: str,
    line: int,
    problem: str,
    impact: str,
    suggested_fix: str,
    fix_rationale: str,
    review_round: int,
) -> dict:
    """Create a new finding entry."""
    now = _now()
    return {
        "id": finding_id,
        "title": title,
        "severity": severity,
        "status": "open",
        "file": file,
        "line": line,
        "problem": problem,
        "impact": impact,
        "suggested_fix": suggested_fix,
        "fix_rationale": fix_rationale,
        "created_round": review_round,
        "created_at": now,
        "updated_at": now,
        "status_history": [
            {"status": "open", "round": review_round, "timestamp": now}
        ],
        "discussion": [],
    }


def new_review_round(
    round_number: int,
    head_sha: str,
    base_sha: str,
    event: str,
    files_reviewed: list[str],
) -> dict:
    """Create a new review round entry."""
    return {
        "round": round_number,
        "timestamp": _now(),
        "head_sha": head_sha,
        "base_sha": base_sha,
        "event": event,
        "files_reviewed": files_reviewed,
    }


# ── Finding Lifecycle ────────────────────────────────────────────────────────

VALID_STATUSES = {"open", "resolved", "partially_fixed", "wontfix", "retracted"}


def transition_finding(state: dict, finding_id: int, new_status: str, note: str = "", round_number: int = 0) -> dict:
    """Transition a finding to a new status with audit trail."""
    if new_status not in VALID_STATUSES:
        raise ValueError(f"Invalid status: {new_status}. Must be one of {VALID_STATUSES}")

    fid = str(finding_id)
    if fid not in state["findings"]:
        raise KeyError(f"Finding {finding_id} not found")

    finding = state["findings"][fid]
    finding["status"] = new_status
    finding["updated_at"] = _now()
    finding["status_history"].append({
        "status": new_status,
        "round": round_number,
        "timestamp": _now(),
        "note": note,
    })

    _recompute_summary(state)
    return state


def add_finding_discussion(state: dict, finding_id: int, role: str, content: str) -> dict:
    """Add a discussion message to a specific finding."""
    fid = str(finding_id)
    if fid not in state["findings"]:
        raise KeyError(f"Finding {finding_id} not found")

    state["findings"][fid]["discussion"].append({
        "role": role,
        "content": content,
        "timestamp": _now(),
    })
    return state


# ── Conversation History ─────────────────────────────────────────────────────

def add_message(state: dict, role: str, content: str, round_number: int = 0, metadata: dict | None = None) -> dict:
    """Append a message to the conversation history."""
    msg = {
        "role": role,
        "content": content,
        "round": round_number,
        "timestamp": _now(),
    }
    if metadata:
        msg["metadata"] = metadata
    state["conversation"].append(msg)
    return state


def get_conversation_for_llm(state: dict) -> list[dict]:
    """
    Convert conversation history to Claude API messages format.

    Returns list of {"role": "user"|"assistant", "content": "..."} dicts.
    System events become user messages prefixed with [SYSTEM].
    """
    messages = []
    for msg in state["conversation"]:
        if msg["role"] == "developer":
            messages.append({"role": "user", "content": msg["content"]})
        elif msg["role"] == "agent":
            messages.append({"role": "assistant", "content": msg["content"]})
        elif msg["role"] == "system":
            messages.append({"role": "user", "content": f"[SYSTEM EVENT] {msg['content']}"})

    # Claude requires alternating user/assistant. Merge consecutive same-role messages.
    if not messages:
        return messages

    merged = [messages[0]]
    for msg in messages[1:]:
        if msg["role"] == merged[-1]["role"]:
            merged[-1]["content"] += "\n\n" + msg["content"]
        else:
            merged.append(msg)

    # Claude requires first message to be user role
    if merged and merged[0]["role"] == "assistant":
        merged.insert(0, {"role": "user", "content": "[SYSTEM EVENT] Continuing previous review session."})

    return merged


# ── State Queries ────────────────────────────────────────────────────────────

def get_open_findings(state: dict) -> list[dict]:
    return [f for f in state["findings"].values() if f["status"] in ("open", "partially_fixed")]


def get_findings_by_status(state: dict, status: str) -> list[dict]:
    return [f for f in state["findings"].values() if f["status"] == status]


def get_latest_round(state: dict) -> dict | None:
    if not state["review_rounds"]:
        return None
    return state["review_rounds"][-1]


def get_finding(state: dict, finding_id: int) -> dict | None:
    return state["findings"].get(str(finding_id))


def get_state_summary_text(state: dict) -> str:
    """Generate a human-readable state summary for LLM context."""
    s = state["summary"]
    lines = [
        f"## Review State (Round {len(state['review_rounds'])})",
        f"- Total findings: {s['total_findings']}",
        f"- Open: {s['open']}",
        f"- Resolved: {s['resolved']}",
        f"- Won't fix: {s['wontfix']}",
        f"- Retracted: {s['retracted']}",
        f"- Assessment: {s['assessment']}",
        f"- Last reviewed SHA: {s['last_reviewed_sha'][:8] if s['last_reviewed_sha'] else 'none'}",
        "",
        "### Active Findings:",
    ]

    for f in get_open_findings(state):
        lines.append(
            f"  - **Finding {f['id']}** [{f['severity']}] ({f['status']}): "
            f"{f['title']} — `{f['file']}:{f['line']}`"
        )
        if f["discussion"]:
            lines.append(f"    Last discussion: {f['discussion'][-1]['content'][:100]}...")

    resolved = get_findings_by_status(state, "resolved")
    if resolved:
        lines.append("\n### Resolved Findings:")
        for f in resolved:
            last_note = ""
            for h in reversed(f["status_history"]):
                if h.get("note"):
                    last_note = f" — {h['note']}"
                    break
            lines.append(f"  - ~~Finding {f['id']}~~ [{f['severity']}]: {f['title']}{last_note}")

    return "\n".join(lines)


# ── Local File Backend ───────────────────────────────────────────────────────

def _state_dir() -> Path:
    return Path(".pr-review/reviews")


def _state_path(pr_number: int) -> Path:
    return _state_dir() / f"pr-{pr_number}.json"


def load_local(pr_number: int) -> dict | None:
    """Load state from local file. Returns None if no state exists."""
    path = _state_path(pr_number)
    if not path.exists():
        return None
    with open(path) as f:
        return json.load(f)


def save_local(state: dict) -> Path:
    """Save state to local file. Returns the path."""
    _state_dir().mkdir(parents=True, exist_ok=True)
    path = _state_path(state["pr"]["number"])
    with open(path, "w") as f:
        json.dump(state, f, indent=2)
    return path


# ── GitHub Comment Backend ───────────────────────────────────────────────────

STATE_MARKER_START = "<!-- REVIEWIQ_STATE_START -->"
STATE_MARKER_END = "<!-- REVIEWIQ_STATE_END -->"
STATE_COMMENT_HEADER = "<!-- REVIEWIQ_STATE_COMMENT -->"


def _gh_headers() -> dict:
    token = os.environ.get("GITHUB_TOKEN", "")
    return {
        "Authorization": f"token {token}",
        "Accept": "application/vnd.github.v3+json",
    }


def _gh_api(method: str, url: str, data: dict | None = None) -> dict | list | None:
    """Make a GitHub API call."""
    full_url = f"https://api.github.com{url}"
    body = json.dumps(data).encode() if data else None
    req = urllib.request.Request(full_url, data=body, method=method, headers=_gh_headers())
    if body:
        req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req) as resp:
            if resp.status == 204:
                return None
            return json.loads(resp.read().decode())
    except urllib.error.HTTPError as e:
        raise RuntimeError(f"GitHub API error {e.code}: {e.read().decode()[:500]}") from e


def _encode_state(state: dict) -> str:
    """Encode state as base64 for embedding in a comment."""
    return base64.b64encode(json.dumps(state, separators=(",", ":")).encode()).decode()


def _decode_state(encoded: str) -> dict:
    """Decode base64 state from a comment."""
    return json.loads(base64.b64decode(encoded).decode())


def _find_state_comment(repo: str, pr_number: int) -> tuple[int | None, dict | None]:
    """Find the hidden state comment on a PR. Returns (comment_id, state) or (None, None)."""
    comments = _gh_api("GET", f"/repos/{repo}/issues/{pr_number}/comments")
    if not comments:
        return None, None

    for comment in comments:
        body = comment.get("body", "")
        if STATE_COMMENT_HEADER in body:
            match = re.search(
                rf"{re.escape(STATE_MARKER_START)}\n(.+?)\n{re.escape(STATE_MARKER_END)}",
                body,
                re.DOTALL,
            )
            if match:
                try:
                    state = _decode_state(match.group(1).strip())
                    return comment["id"], state
                except (json.JSONDecodeError, Exception):
                    pass
    return None, None


def load_remote(repo: str, pr_number: int) -> dict | None:
    """Load state from a hidden GitHub PR comment."""
    _, state = _find_state_comment(repo, pr_number)
    return state


def save_remote(state: dict) -> None:
    """Save state to a hidden GitHub PR comment (create or update)."""
    repo = state["pr"]["repo"]
    pr_number = state["pr"]["number"]
    encoded = _encode_state(state)

    # Build a minimal visible summary + hidden state blob
    summary = state["summary"]
    body = f"""{STATE_COMMENT_HEADER}
<details>
<summary>ReviewIQ State (Round {len(state['review_rounds'])}) — {summary['open']} open, {summary['resolved']} resolved</summary>

| Metric | Count |
|--------|-------|
| Total findings | {summary['total_findings']} |
| Open | {summary['open']} |
| Resolved | {summary['resolved']} |
| Won't fix | {summary['wontfix']} |
| Retracted | {summary['retracted']} |
| Assessment | {summary['assessment']} |

</details>

{STATE_MARKER_START}
{encoded}
{STATE_MARKER_END}"""

    comment_id, _ = _find_state_comment(repo, pr_number)
    if comment_id:
        _gh_api("PATCH", f"/repos/{repo}/issues/comments/{comment_id}", {"body": body})
    else:
        _gh_api("POST", f"/repos/{repo}/issues/{pr_number}/comments", {"body": body})


# ── Unified Load/Save ────────────────────────────────────────────────────────

def load(pr_number: int, repo: str = "", prefer: str = "auto") -> dict:
    """
    Load state from the best available backend.

    prefer:
      "local"  — local file only
      "remote" — GitHub comment only
      "auto"   — try local first, then remote, then create new
    """
    state = None

    if prefer in ("local", "auto"):
        state = load_local(pr_number)

    if state is None and prefer in ("remote", "auto") and repo:
        state = load_remote(repo, pr_number)

    if state is None:
        state = new_state(pr_number, repo)

    return state


def save(state: dict, targets: str = "auto") -> None:
    """
    Save state to the specified backends.

    targets:
      "local"  — local file only
      "remote" — GitHub comment only
      "both"   — both backends
      "auto"   — local always, remote if repo is set and GITHUB_TOKEN exists
    """
    if targets in ("local", "both", "auto"):
        save_local(state)

    if targets in ("remote", "both"):
        save_remote(state)
    elif targets == "auto" and state["pr"]["repo"] and os.environ.get("GITHUB_TOKEN"):
        save_remote(state)


# ── Helpers ──────────────────────────────────────────────────────────────────

def _now() -> str:
    return datetime.now(timezone.utc).isoformat()


def _recompute_summary(state: dict) -> None:
    """Recompute the summary counts from findings."""
    findings = state["findings"].values()
    state["summary"]["total_findings"] = len(state["findings"])
    state["summary"]["open"] = sum(1 for f in findings if f["status"] in ("open", "partially_fixed"))
    state["summary"]["resolved"] = sum(1 for f in findings if f["status"] == "resolved")
    state["summary"]["wontfix"] = sum(1 for f in findings if f["status"] == "wontfix")
    state["summary"]["retracted"] = sum(1 for f in findings if f["status"] == "retracted")

    # Auto-assess
    open_count = state["summary"]["open"]
    has_critical = any(
        f["severity"] == "CRITICAL" and f["status"] in ("open", "partially_fixed")
        for f in findings
    )
    if open_count == 0 and state["summary"]["total_findings"] > 0:
        state["summary"]["assessment"] = "APPROVE"
    elif has_critical:
        state["summary"]["assessment"] = "REQUEST CHANGES"
    elif open_count > 0:
        state["summary"]["assessment"] = "REQUEST CHANGES"
    else:
        state["summary"]["assessment"] = "PENDING"
