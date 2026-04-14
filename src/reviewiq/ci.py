from __future__ import annotations

"""
ReviewIQ CI Mode — GitHub Actions integration.

Handles webhook events (PR opened, push, comment) using the shared engine.
Posts results as PR comments and persists state to hidden PR comments.
"""

import json
import os
import re
import sys
import urllib.request

from reviewiq import state as st
from reviewiq import git
from reviewiq import engine


def log(msg: str) -> None:
    print(f"[reviewiq:ci] {msg}", file=sys.stderr)


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


def post_comment(repo: str, pr_number: int, body: str) -> None:
    """Post a visible comment on the PR."""
    gh_api("POST", f"/repos/{repo}/issues/{pr_number}/comments", {"body": body})


def get_pr_metadata(repo: str, pr_number: int) -> dict:
    """Fetch PR metadata from GitHub API."""
    pr_data = gh_api("GET", f"/repos/{repo}/pulls/{pr_number}")
    return {
        "title": pr_data["title"],
        "body": pr_data.get("body") or "",
        "author": pr_data["user"]["login"],
        "base_branch": pr_data["base"]["ref"],
        "head_branch": pr_data["head"]["ref"],
        "base_sha": pr_data["base"]["sha"],
        "head_sha": pr_data["head"]["sha"],
    }


def run(repo: str, pr_number: int, event_type: str, comment_body: str = "") -> None:
    """Main CI entry point."""
    log(f"Event: {event_type}, PR: #{pr_number}, Repo: {repo}")

    review_state = st.load(pr_number, repo, prefer="auto")
    log(f"State loaded: {review_state['summary']['total_findings']} findings, "
        f"{len(review_state['review_rounds'])} rounds")

    if event_type == "opened":
        _handle_opened(review_state, repo, pr_number)
    elif event_type == "synchronize":
        _handle_synchronize(review_state, repo, pr_number)
    elif event_type == "comment":
        if re.search(r"@review-agent", comment_body, re.IGNORECASE):
            _handle_comment(review_state, repo, pr_number, comment_body)
        else:
            log("Comment doesn't mention @review-agent, skipping")
    else:
        log(f"Unknown event type: {event_type}")
        sys.exit(1)


def _handle_opened(review_state: dict, repo: str, pr_number: int) -> None:
    log("Handling PR opened")

    meta = get_pr_metadata(repo, pr_number)
    review_state["pr"].update({
        "title": meta["title"],
        "author": meta["author"],
        "base_branch": meta["base_branch"],
        "head_branch": meta["head_branch"],
    })

    diff = git.get_diff(meta["base_sha"], meta["head_sha"])
    changed_files = git.get_changed_files(meta["base_sha"], meta["head_sha"])
    file_contents = git.read_files(changed_files)
    history = git.get_file_history(changed_files)

    response = engine.run_review(
        review_state,
        diff=diff,
        file_contents=file_contents,
        history=history,
        changed_files=changed_files,
        head_sha=meta["head_sha"],
        base_sha=meta["base_sha"],
        pr_title=meta["title"],
        pr_author=meta["author"],
        pr_body=meta["body"],
        base_branch=meta["base_branch"],
        head_branch=meta["head_branch"],
    )

    st.save(review_state, targets="both")
    post_comment(repo, pr_number, response)
    log(f"Review posted. {review_state['summary']['total_findings']} findings tracked.")


def _handle_synchronize(review_state: dict, repo: str, pr_number: int) -> None:
    log("Handling push (incremental re-review)")

    meta = get_pr_metadata(repo, pr_number)
    diff = git.get_diff(meta["base_sha"], meta["head_sha"])
    changed_files = git.get_changed_files(meta["base_sha"], meta["head_sha"])
    file_contents = git.read_files(changed_files)
    incremental_diff = git.get_incremental_diff(review_state, meta["head_sha"])

    response = engine.run_check(
        review_state,
        diff=diff,
        file_contents=file_contents,
        changed_files=changed_files,
        head_sha=meta["head_sha"],
        base_sha=meta["base_sha"],
        incremental_diff=incremental_diff,
        pr_title=meta["title"],
        pr_author=meta["author"],
        base_branch=meta["base_branch"],
        head_branch=meta["head_branch"],
    )

    st.save(review_state, targets="both")
    post_comment(repo, pr_number, response)
    log(f"Re-review posted. Open: {review_state['summary']['open']}, Resolved: {review_state['summary']['resolved']}")


def _handle_comment(review_state: dict, repo: str, pr_number: int, comment_body: str) -> None:
    command = re.sub(r"@review-agent\s*", "", comment_body, flags=re.IGNORECASE).strip()
    log(f"Handling comment: {command[:80]}...")

    meta = get_pr_metadata(repo, pr_number)
    changed_files = git.get_changed_files(meta["base_sha"], meta["head_sha"])
    file_contents = git.read_files(changed_files)

    finding_id = None
    finding_match = re.search(r"(?:finding|#)\s*(\d+)", command, re.IGNORECASE)
    if finding_match:
        finding_id = int(finding_match.group(1))

    response = engine.run_ask(review_state, command, file_contents, finding_id=finding_id)

    st.save(review_state, targets="both")
    post_comment(repo, pr_number, response)
    log("Reply posted")
