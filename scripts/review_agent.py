#!/usr/bin/env python3
"""
ReviewIQ — Stateful PR Review Agent

Maintains persistent state across interactions:
  - Findings with lifecycle tracking (open -> resolved/wontfix/retracted)
  - Full conversation history sent as multi-turn messages
  - Review rounds with SHA tracking for incremental diffs
  - Per-finding discussion threads

Dual backend: local JSON + hidden GitHub PR comment.
"""

import json
import os
import re
import subprocess
import sys
import urllib.request
import urllib.error
from pathlib import Path

import state as st


# ── Config ───────────────────────────────────────────────────────────────────

MODEL = os.environ.get("MODEL", "claude-sonnet-4-6-20250514")
MAX_TOKENS = int(os.environ.get("MAX_TOKENS", "8192"))
API_URL = "https://api.anthropic.com/v1/messages"
REPO = os.environ.get("GITHUB_REPOSITORY", "")
PR_NUMBER = int(os.environ.get("PR_NUMBER", "0"))
EVENT_TYPE = os.environ.get("EVENT_TYPE", "")
COMMENT_BODY = os.environ.get("COMMENT_BODY", "")


# ── Helpers ──────────────────────────────────────────────────────────────────

def log(msg: str) -> None:
    print(f"[review-agent] {msg}", file=sys.stderr)


def run_git(*args: str) -> str:
    """Run a git command and return stdout."""
    result = subprocess.run(
        ["git", *args],
        capture_output=True, text=True, timeout=30,
    )
    return result.stdout.strip()


def gh_api(method: str, endpoint: str, data: dict | None = None) -> dict | list | None:
    """Call GitHub API."""
    url = f"https://api.github.com{endpoint}"
    body = json.dumps(data).encode() if data else None
    headers = {
        "Authorization": f"token {os.environ['GITHUB_TOKEN']}",
        "Accept": "application/vnd.github.v3+json",
    }
    req = urllib.request.Request(url, data=body, method=method, headers=headers)
    if body:
        req.add_header("Content-Type", "application/json")
    with urllib.request.urlopen(req) as resp:
        if resp.status == 204:
            return None
        return json.loads(resp.read().decode())


def post_comment(body: str) -> None:
    """Post a visible comment on the PR."""
    gh_api("POST", f"/repos/{REPO}/issues/{PR_NUMBER}/comments", {"body": body})


def call_claude(system_prompt: str, messages: list[dict]) -> str:
    """Call Claude API with full conversation history."""
    payload = {
        "model": MODEL,
        "max_tokens": MAX_TOKENS,
        "system": system_prompt,
        "messages": messages,
    }

    body = json.dumps(payload).encode()
    req = urllib.request.Request(API_URL, data=body, method="POST", headers={
        "x-api-key": os.environ["ANTHROPIC_API_KEY"],
        "anthropic-version": "2023-06-01",
        "content-type": "application/json",
    })

    with urllib.request.urlopen(req, timeout=120) as resp:
        result = json.loads(resp.read().decode())

    return result["content"][0]["text"]


# ── Context Assembly ─────────────────────────────────────────────────────────

def get_pr_metadata() -> dict:
    """Fetch PR metadata from GitHub API."""
    pr_data = gh_api("GET", f"/repos/{REPO}/pulls/{PR_NUMBER}")
    return {
        "title": pr_data["title"],
        "body": pr_data.get("body") or "",
        "author": pr_data["user"]["login"],
        "base_branch": pr_data["base"]["ref"],
        "head_branch": pr_data["head"]["ref"],
        "base_sha": pr_data["base"]["sha"],
        "head_sha": pr_data["head"]["sha"],
    }


def get_diff(base_sha: str, head_sha: str, base_branch: str, head_branch: str) -> str:
    """Get the PR diff."""
    diff = run_git("diff", f"{base_sha}...{head_sha}")
    if not diff:
        diff = run_git("diff", f"{base_branch}...{head_branch}")
    return diff


def get_changed_files(base_sha: str, head_sha: str, base_branch: str, head_branch: str) -> list[str]:
    """Get list of changed files."""
    files = run_git("diff", "--name-only", f"{base_sha}...{head_sha}")
    if not files:
        files = run_git("diff", "--name-only", f"{base_branch}...{head_branch}")
    return [f for f in files.split("\n") if f.strip()]


def read_files(file_list: list[str]) -> str:
    """Read full contents of all changed files."""
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


# ── System Prompt ────────────────────────────────────────────────────────────

def read_system_prompt() -> str:
    """Read the agent protocol file."""
    agent_file = Path(".pr-review/agent.md")
    if agent_file.exists():
        return agent_file.read_text()
    return "You are a PR review agent. Provide thorough, actionable code reviews with concrete fixes."


STRUCTURED_OUTPUT_INSTRUCTION = """

## IMPORTANT: Structured Output for State Tracking

After your human-readable review, you MUST append a JSON block that I will parse to update the review state.
Wrap it in markers exactly like this:

<!-- REVIEWIQ_FINDINGS_START -->
```json
{
  "findings": [
    {
      "id": 1,
      "title": "Short title",
      "severity": "CRITICAL|IMPORTANT|NIT|QUESTION",
      "status": "open",
      "file": "path/to/file.ext",
      "line": 42,
      "problem": "What's wrong",
      "impact": "What breaks",
      "suggested_fix": "code fix here",
      "fix_rationale": "Why this approach"
    }
  ],
  "status_updates": [
    {
      "id": 1,
      "new_status": "resolved|partially_fixed|wontfix|retracted",
      "note": "Why the status changed"
    }
  ],
  "assessment": "APPROVE|REQUEST CHANGES|NEEDS DISCUSSION"
}
```
<!-- REVIEWIQ_FINDINGS_END -->

Rules for the JSON block:
- On initial review: populate "findings" array, leave "status_updates" empty
- On incremental review (check): populate "status_updates" for existing findings, add new findings if any
- On explain/fix/other commands: only include "status_updates" if a finding's status changed
- Finding IDs must be sequential integers starting from the highest existing ID + 1 for new findings
- Always include the "assessment" field
"""


# ── Response Parsing ─────────────────────────────────────────────────────────

def parse_structured_output(response: str, review_state: dict, round_number: int) -> str:
    """
    Parse the structured JSON from the response and update state.
    Returns the human-readable part of the response (without the JSON block).
    """
    pattern = r"<!-- REVIEWIQ_FINDINGS_START -->\s*```json\s*(\{.*?\})\s*```\s*<!-- REVIEWIQ_FINDINGS_END -->"
    match = re.search(pattern, response, re.DOTALL)

    if not match:
        log("No structured output found in response — state not updated from response")
        return response

    try:
        data = json.loads(match.group(1))
    except json.JSONDecodeError as e:
        log(f"Failed to parse structured output: {e}")
        return response

    # Process new findings
    for finding_data in data.get("findings", []):
        fid = finding_data["id"]
        if str(fid) not in review_state["findings"]:
            review_state["findings"][str(fid)] = st.new_finding(
                finding_id=fid,
                title=finding_data["title"],
                severity=finding_data["severity"],
                file=finding_data["file"],
                line=finding_data.get("line", 0),
                problem=finding_data["problem"],
                impact=finding_data.get("impact", ""),
                suggested_fix=finding_data.get("suggested_fix", ""),
                fix_rationale=finding_data.get("fix_rationale", ""),
                review_round=round_number,
            )

    # Process status updates
    for update in data.get("status_updates", []):
        fid = update["id"]
        new_status = update["new_status"]
        note = update.get("note", "")
        try:
            st.transition_finding(review_state, fid, new_status, note, round_number)
        except (KeyError, ValueError) as e:
            log(f"Failed to update finding {fid}: {e}")

    # Update assessment
    if "assessment" in data:
        review_state["summary"]["assessment"] = data["assessment"]

    # Recompute summary
    st._recompute_summary(review_state)

    # Strip the JSON block from the human-readable response
    human_response = response[:match.start()].rstrip()
    trailing = response[match.end():].strip()
    if trailing:
        human_response += "\n\n" + trailing

    return human_response


# ── Event Handlers ───────────────────────────────────────────────────────────

def handle_opened(review_state: dict) -> None:
    """Handle PR opened — full initial review."""
    log("Handling PR opened event")

    meta = get_pr_metadata()
    review_state["pr"].update({
        "title": meta["title"],
        "author": meta["author"],
        "base_branch": meta["base_branch"],
        "head_branch": meta["head_branch"],
    })

    diff = get_diff(meta["base_sha"], meta["head_sha"], meta["base_branch"], meta["head_branch"])
    changed_files = get_changed_files(meta["base_sha"], meta["head_sha"], meta["base_branch"], meta["head_branch"])
    file_contents = read_files(changed_files)
    history = get_file_history(changed_files)

    # Create review round
    round_number = len(review_state["review_rounds"]) + 1
    review_state["review_rounds"].append(
        st.new_review_round(round_number, meta["head_sha"], meta["base_sha"], "opened", changed_files)
    )
    review_state["summary"]["last_reviewed_sha"] = meta["head_sha"]

    # Build the user message for this turn
    user_content = f"""Review this pull request.

## PR Metadata
- **Title**: {meta['title']}
- **Author**: {meta['author']}
- **Branch**: {meta['head_branch']} -> {meta['base_branch']}
- **Description**: {meta['body']}

## Diff
```diff
{diff}
```

## Full File Contents
{file_contents}

## Recent Commit History
{history}

This is review round {round_number}. Run the full 'review' command as defined in your protocol."""

    # Record in conversation
    st.add_message(review_state, "system", f"PR opened. Review round {round_number}.", round_number)
    st.add_message(review_state, "developer", user_content, round_number)

    # Build messages for Claude — full conversation history
    system_prompt = read_system_prompt() + STRUCTURED_OUTPUT_INSTRUCTION
    messages = st.get_conversation_for_llm(review_state)

    log(f"Calling Claude with {len(messages)} message(s), round {round_number}")
    response = call_claude(system_prompt, messages)

    # Parse structured output and update state
    human_response = parse_structured_output(response, review_state, round_number)

    # Record agent response
    st.add_message(review_state, "agent", human_response, round_number)

    # Save state and post comment
    st.save(review_state, targets="both" if REPO else "local")
    post_comment(human_response)
    log(f"Review posted. {review_state['summary']['total_findings']} findings tracked.")


def handle_synchronize(review_state: dict) -> None:
    """Handle push to PR — incremental re-review with state diff."""
    log("Handling push event (incremental re-review)")

    meta = get_pr_metadata()
    diff = get_diff(meta["base_sha"], meta["head_sha"], meta["base_branch"], meta["head_branch"])
    changed_files = get_changed_files(meta["base_sha"], meta["head_sha"], meta["base_branch"], meta["head_branch"])
    file_contents = read_files(changed_files)

    # Get incremental diff (changes since last review)
    incremental_diff = get_incremental_diff(review_state, meta["head_sha"])

    # Create new review round
    round_number = len(review_state["review_rounds"]) + 1
    review_state["review_rounds"].append(
        st.new_review_round(round_number, meta["head_sha"], meta["base_sha"], "synchronize", changed_files)
    )
    review_state["summary"]["last_reviewed_sha"] = meta["head_sha"]

    # Build state context
    state_summary = st.get_state_summary_text(review_state)

    # Build user message
    incremental_section = ""
    if incremental_diff:
        last_round = review_state["review_rounds"][-2] if len(review_state["review_rounds"]) > 1 else None
        prev_sha = last_round["head_sha"][:8] if last_round else "unknown"
        incremental_section = f"""
## Changes Since Last Review (since {prev_sha})
```diff
{incremental_diff}
```
"""

    user_content = f"""The developer has pushed new changes. Run the 'check' command.

{state_summary}

## PR Metadata
- **Title**: {meta['title']}
- **Author**: {meta['author']}
- **Branch**: {meta['head_branch']} -> {meta['base_branch']}
{incremental_section}
## Full Diff (complete)
```diff
{diff}
```

## Full File Contents (current state)
{file_contents}

This is review round {round_number}. Compare against your previous findings above.
For each finding, report: RESOLVED / PARTIALLY FIXED / UNRESOLVED.
Flag any NEW issues introduced by the fixes."""

    st.add_message(review_state, "system", f"Developer pushed new changes. Review round {round_number}.", round_number)
    st.add_message(review_state, "developer", user_content, round_number)

    system_prompt = read_system_prompt() + STRUCTURED_OUTPUT_INSTRUCTION
    messages = st.get_conversation_for_llm(review_state)

    log(f"Calling Claude with {len(messages)} message(s), round {round_number}")
    response = call_claude(system_prompt, messages)

    human_response = parse_structured_output(response, review_state, round_number)
    st.add_message(review_state, "agent", human_response, round_number)

    st.save(review_state, targets="both" if REPO else "local")
    post_comment(human_response)
    log(f"Incremental review posted. Open: {review_state['summary']['open']}, Resolved: {review_state['summary']['resolved']}")


def handle_comment(review_state: dict) -> None:
    """Handle @review-agent mention in a comment."""
    command = re.sub(r"@review-agent\s*", "", COMMENT_BODY, flags=re.IGNORECASE).strip()
    log(f"Handling comment command: {command[:80]}...")

    meta = get_pr_metadata()
    diff = get_diff(meta["base_sha"], meta["head_sha"], meta["base_branch"], meta["head_branch"])
    changed_files = get_changed_files(meta["base_sha"], meta["head_sha"], meta["base_branch"], meta["head_branch"])
    file_contents = read_files(changed_files)
    state_summary = st.get_state_summary_text(review_state)

    round_number = len(review_state["review_rounds"])  # No new round for comments

    # Check if this references a specific finding
    finding_context = ""
    finding_match = re.search(r"(?:finding|#)\s*(\d+)", command, re.IGNORECASE)
    if finding_match:
        fid = int(finding_match.group(1))
        finding = st.get_finding(review_state, fid)
        if finding:
            discussion_text = ""
            if finding["discussion"]:
                discussion_text = "\n**Previous discussion on this finding:**\n"
                for msg in finding["discussion"]:
                    discussion_text += f"  [{msg['role']}] {msg['content']}\n"

            finding_context = f"""
## Referenced Finding #{fid}
- **Title**: {finding['title']}
- **Severity**: {finding['severity']}
- **Status**: {finding['status']}
- **File**: `{finding['file']}:{finding['line']}`
- **Problem**: {finding['problem']}
- **Impact**: {finding['impact']}
- **Suggested fix**: {finding['suggested_fix']}
{discussion_text}"""
            # Record in finding's discussion thread
            st.add_finding_discussion(review_state, fid, "developer", command)

    user_content = f"""A developer is asking a follow-up question on this PR.

{state_summary}
{finding_context}

## Current Code
{file_contents}

## Developer's Message
{command}

Respond according to your protocol. You have the full conversation history above for context.
If they reference a finding, trace the actual code. If they disagree, engage with their reasoning."""

    st.add_message(review_state, "developer", command, round_number, {"event": "comment"})

    system_prompt = read_system_prompt() + STRUCTURED_OUTPUT_INSTRUCTION
    messages = st.get_conversation_for_llm(review_state)

    log(f"Calling Claude with {len(messages)} message(s)")
    response = call_claude(system_prompt, messages)

    human_response = parse_structured_output(response, review_state, round_number)
    st.add_message(review_state, "agent", human_response, round_number)

    # Record in finding discussion if applicable
    if finding_match:
        fid = int(finding_match.group(1))
        st.add_finding_discussion(review_state, fid, "agent", human_response[:500])

    st.save(review_state, targets="both" if REPO else "local")
    post_comment(human_response)
    log("Reply posted")


# ── Main ─────────────────────────────────────────────────────────────────────

def main() -> None:
    log(f"Event: {EVENT_TYPE}, PR: #{PR_NUMBER}, Repo: {REPO}")

    # Load existing state (tries local first, then remote, then creates new)
    review_state = st.load(PR_NUMBER, REPO, prefer="auto")
    log(f"State loaded: {review_state['summary']['total_findings']} findings, "
        f"{len(review_state['review_rounds'])} rounds, "
        f"{len(review_state['conversation'])} messages")

    if EVENT_TYPE == "opened":
        handle_opened(review_state)
    elif EVENT_TYPE == "synchronize":
        handle_synchronize(review_state)
    elif EVENT_TYPE == "comment":
        if re.search(r"@review-agent", COMMENT_BODY, re.IGNORECASE):
            handle_comment(review_state)
        else:
            log("Comment doesn't mention @review-agent, skipping")
    else:
        log(f"Unknown event type: {EVENT_TYPE}")
        sys.exit(1)


if __name__ == "__main__":
    main()
