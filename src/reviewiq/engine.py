from __future__ import annotations

"""
ReviewIQ Engine — Core review logic used by both CLI and CI.

Handles Claude API calls, structured output parsing, and state updates.
"""

import json
import os
import re
import sys
import urllib.request
import urllib.error
from pathlib import Path

from reviewiq import state as st
from reviewiq import skills


# ── Config ───────────────────────────────────────────────────────────────────

DEFAULT_MODEL = "claude-sonnet-4-6-20250514"
DEFAULT_MAX_TOKENS = 8192
API_URL = "https://api.anthropic.com/v1/messages"


def log(msg: str) -> None:
    print(f"[reviewiq] {msg}", file=sys.stderr)


# ── Claude API ───────────────────────────────────────────────────────────────

def call_claude(
    system_prompt: str,
    messages: list[dict],
    model: str | None = None,
    max_tokens: int | None = None,
) -> str:
    """Call Claude API with full conversation history."""
    api_key = os.environ.get("ANTHROPIC_API_KEY", "")
    if not api_key:
        raise RuntimeError(
            "ANTHROPIC_API_KEY not set. Export it or pass via environment:\n"
            "  export ANTHROPIC_API_KEY=sk-ant-..."
        )

    payload = {
        "model": model or os.environ.get("MODEL", DEFAULT_MODEL),
        "max_tokens": max_tokens or int(os.environ.get("MAX_TOKENS", str(DEFAULT_MAX_TOKENS))),
        "system": system_prompt,
        "messages": messages,
    }

    body = json.dumps(payload).encode()
    req = urllib.request.Request(API_URL, data=body, method="POST", headers={
        "x-api-key": api_key,
        "anthropic-version": "2023-06-01",
        "content-type": "application/json",
    })

    with urllib.request.urlopen(req, timeout=120) as resp:
        result = json.loads(resp.read().decode())

    return result["content"][0]["text"]


# ── System Prompt ────────────────────────────────────────────────────────────

def read_system_prompt(changed_files: list[str] | None = None, file_contents: str = "") -> str:
    """Read the agent protocol file and append auto-detected skills."""
    agent_file = Path(".pr-review/agent.md")
    if agent_file.exists():
        base_prompt = agent_file.read_text()
    else:
        base_prompt = "You are a PR review agent. Provide thorough, actionable code reviews with concrete fixes."

    # Auto-detect and load relevant skills
    if changed_files:
        detected = skills.detect_skills(changed_files, file_contents)
        skill_prompt = skills.load_skills(detected)
        if skill_prompt:
            log(f"Skills loaded: {', '.join(detected['always'] + detected['languages'] + detected['frameworks'] + detected['devops'])}")
            base_prompt += "\n\n" + skill_prompt

    return base_prompt


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


# ── Review Operations ────────────────────────────────────────────────────────

def run_review(
    review_state: dict,
    diff: str,
    file_contents: str,
    history: str,
    changed_files: list[str],
    head_sha: str,
    base_sha: str,
    pr_title: str = "",
    pr_author: str = "",
    pr_body: str = "",
    base_branch: str = "",
    head_branch: str = "",
) -> str:
    """Run a full initial review. Returns the human-readable response."""
    round_number = len(review_state["review_rounds"]) + 1
    review_state["review_rounds"].append(
        st.new_review_round(round_number, head_sha, base_sha, "review", changed_files)
    )
    review_state["summary"]["last_reviewed_sha"] = head_sha

    user_content = f"""Review this pull request.

## PR Metadata
- **Title**: {pr_title}
- **Author**: {pr_author}
- **Branch**: {head_branch} -> {base_branch}
- **Description**: {pr_body}

## Diff
```diff
{diff}
```

## Full File Contents
{file_contents}

## Recent Commit History
{history}

This is review round {round_number}. Run the full 'review' command as defined in your protocol."""

    st.add_message(review_state, "system", f"Review round {round_number}.", round_number)
    st.add_message(review_state, "developer", user_content, round_number)

    system_prompt = read_system_prompt(changed_files, file_contents) + STRUCTURED_OUTPUT_INSTRUCTION
    messages = st.get_conversation_for_llm(review_state)

    log(f"Calling Claude with {len(messages)} message(s), round {round_number}")
    response = call_claude(system_prompt, messages)

    human_response = parse_structured_output(response, review_state, round_number)
    st.add_message(review_state, "agent", human_response, round_number)

    return human_response


def run_check(
    review_state: dict,
    diff: str,
    file_contents: str,
    changed_files: list[str],
    head_sha: str,
    base_sha: str,
    incremental_diff: str | None = None,
    pr_title: str = "",
    pr_author: str = "",
    base_branch: str = "",
    head_branch: str = "",
) -> str:
    """Run an incremental re-review. Returns the human-readable response."""
    round_number = len(review_state["review_rounds"]) + 1
    review_state["review_rounds"].append(
        st.new_review_round(round_number, head_sha, base_sha, "check", changed_files)
    )
    review_state["summary"]["last_reviewed_sha"] = head_sha

    state_summary = st.get_state_summary_text(review_state)

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
- **Title**: {pr_title}
- **Author**: {pr_author}
- **Branch**: {head_branch} -> {base_branch}
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

    st.add_message(review_state, "system", f"Developer pushed changes. Review round {round_number}.", round_number)
    st.add_message(review_state, "developer", user_content, round_number)

    system_prompt = read_system_prompt(changed_files, file_contents) + STRUCTURED_OUTPUT_INSTRUCTION
    messages = st.get_conversation_for_llm(review_state)

    log(f"Calling Claude with {len(messages)} message(s), round {round_number}")
    response = call_claude(system_prompt, messages)

    human_response = parse_structured_output(response, review_state, round_number)
    st.add_message(review_state, "agent", human_response, round_number)

    return human_response


def run_ask(
    review_state: dict,
    question: str,
    file_contents: str,
    finding_id: int | None = None,
    changed_files: list[str] | None = None,
) -> str:
    """Ask a follow-up question. Returns the human-readable response."""
    round_number = len(review_state["review_rounds"])
    state_summary = st.get_state_summary_text(review_state)

    finding_context = ""
    if finding_id is not None:
        finding = st.get_finding(review_state, finding_id)
        if finding:
            discussion_text = ""
            if finding["discussion"]:
                discussion_text = "\n**Previous discussion on this finding:**\n"
                for msg in finding["discussion"]:
                    discussion_text += f"  [{msg['role']}] {msg['content']}\n"

            finding_context = f"""
## Referenced Finding #{finding_id}
- **Title**: {finding['title']}
- **Severity**: {finding['severity']}
- **Status**: {finding['status']}
- **File**: `{finding['file']}:{finding['line']}`
- **Problem**: {finding['problem']}
- **Impact**: {finding['impact']}
- **Suggested fix**: {finding['suggested_fix']}
{discussion_text}"""
            st.add_finding_discussion(review_state, finding_id, "developer", question)

    user_content = f"""A developer is asking a follow-up question.

{state_summary}
{finding_context}

## Current Code
{file_contents}

## Developer's Message
{question}

Respond according to your protocol. You have the full conversation history above for context.
If they reference a finding, trace the actual code. If they disagree, engage with their reasoning."""

    st.add_message(review_state, "developer", question, round_number, {"event": "question"})

    system_prompt = read_system_prompt(changed_files, file_contents) + STRUCTURED_OUTPUT_INSTRUCTION
    messages = st.get_conversation_for_llm(review_state)

    log(f"Calling Claude with {len(messages)} message(s)")
    response = call_claude(system_prompt, messages)

    human_response = parse_structured_output(response, review_state, round_number)
    st.add_message(review_state, "agent", human_response, round_number)

    if finding_id is not None:
        st.add_finding_discussion(review_state, finding_id, "agent", human_response[:500])

    return human_response
