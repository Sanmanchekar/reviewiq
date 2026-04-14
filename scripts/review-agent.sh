#!/usr/bin/env bash
#
# review-agent.sh — CI webhook glue script
#
# Rebuilds PR context from git, calls Claude API, posts review comments.
# Triggered by GitHub Actions on PR open/push/comment events.
#
# Required env vars:
#   ANTHROPIC_API_KEY  — Claude API key
#   GITHUB_TOKEN       — GitHub token with PR comment permissions
#   PR_NUMBER          — Pull request number
#   GITHUB_REPOSITORY  — owner/repo
#   EVENT_TYPE         — "opened" | "synchronize" | "comment"
#   COMMENT_BODY       — (comment events only) the comment text
#
# Optional:
#   MODEL              — Claude model to use (default: claude-sonnet-4-6-20250514)
#   MAX_TOKENS         — Max response tokens (default: 8192)

set -euo pipefail

MODEL="${MODEL:-claude-sonnet-4-6-20250514}"
MAX_TOKENS="${MAX_TOKENS:-8192}"
REPO="${GITHUB_REPOSITORY}"
PR="${PR_NUMBER}"
API_URL="https://api.anthropic.com/v1/messages"
GH_API="https://api.github.com"

# ── Helpers ──────────────────────────────────────────────────────────────────

log() { echo "[review-agent] $*" >&2; }

gh_api() {
  local method="$1" endpoint="$2"
  shift 2
  curl -sfL \
    -X "$method" \
    -H "Authorization: token ${GITHUB_TOKEN}" \
    -H "Accept: application/vnd.github.v3+json" \
    "${GH_API}${endpoint}" \
    "$@"
}

call_claude() {
  local system_prompt="$1" user_prompt="$2"
  curl -sfL "$API_URL" \
    -H "x-api-key: ${ANTHROPIC_API_KEY}" \
    -H "anthropic-version: 2023-06-01" \
    -H "content-type: application/json" \
    -d "$(jq -n \
      --arg model "$MODEL" \
      --argjson max_tokens "$MAX_TOKENS" \
      --arg system "$system_prompt" \
      --arg user "$user_prompt" \
      '{
        model: $model,
        max_tokens: $max_tokens,
        system: $system,
        messages: [{role: "user", content: $user}]
      }')" \
    | jq -r '.content[0].text'
}

post_comment() {
  local body="$1"
  gh_api POST "/repos/${REPO}/issues/${PR}/comments" \
    -d "$(jq -n --arg body "$body" '{body: $body}')"
}

# ── Context Assembly ─────────────────────────────────────────────────────────

assemble_context() {
  log "Assembling PR context..."

  # Get PR metadata
  local pr_data
  pr_data=$(gh_api GET "/repos/${REPO}/pulls/${PR}")
  PR_TITLE=$(echo "$pr_data" | jq -r '.title')
  PR_BODY=$(echo "$pr_data" | jq -r '.body // ""')
  PR_AUTHOR=$(echo "$pr_data" | jq -r '.user.login')
  BASE_BRANCH=$(echo "$pr_data" | jq -r '.base.ref')
  HEAD_BRANCH=$(echo "$pr_data" | jq -r '.head.ref')
  BASE_SHA=$(echo "$pr_data" | jq -r '.base.sha')
  HEAD_SHA=$(echo "$pr_data" | jq -r '.head.sha')

  # Get the diff
  DIFF=$(git diff "${BASE_SHA}...${HEAD_SHA}" 2>/dev/null || git diff "${BASE_BRANCH}...${HEAD_BRANCH}")

  # Get changed files
  CHANGED_FILES=$(git diff --name-only "${BASE_SHA}...${HEAD_SHA}" 2>/dev/null \
    || git diff --name-only "${BASE_BRANCH}...${HEAD_BRANCH}")

  # Read full contents of changed files
  FILE_CONTENTS=""
  while IFS= read -r file; do
    if [[ -f "$file" ]]; then
      FILE_CONTENTS+="
--- FILE: ${file} ---
$(cat "$file")
--- END: ${file} ---
"
    fi
  done <<< "$CHANGED_FILES"

  # Get recent commit history for changed files
  COMMIT_HISTORY=""
  while IFS= read -r file; do
    if [[ -f "$file" ]]; then
      COMMIT_HISTORY+="
--- HISTORY: ${file} ---
$(git log -5 --oneline --follow -- "$file" 2>/dev/null || echo "(no history)")
--- END HISTORY ---
"
    fi
  done <<< "$CHANGED_FILES"

  # Get existing review comments (conversation history)
  EXISTING_COMMENTS=$(gh_api GET "/repos/${REPO}/issues/${PR}/comments" \
    | jq -r '.[] | "[\(.user.login)] \(.body)"' 2>/dev/null || echo "")

  log "Context assembled: $(echo "$CHANGED_FILES" | wc -l | tr -d ' ') files changed"
}

# ── System Prompt ────────────────────────────────────────────────────────────

read_system_prompt() {
  local agent_file=".pr-review/agent.md"
  if [[ -f "$agent_file" ]]; then
    cat "$agent_file"
  else
    echo "You are a PR review agent. Provide thorough, actionable code reviews with concrete fixes."
  fi
}

# ── Event Handlers ───────────────────────────────────────────────────────────

handle_opened() {
  log "Handling PR opened/synchronize event"
  assemble_context

  local system_prompt
  system_prompt=$(read_system_prompt)

  local user_prompt="Review this pull request.

## PR Metadata
- **Title**: ${PR_TITLE}
- **Author**: ${PR_AUTHOR}
- **Branch**: ${HEAD_BRANCH} -> ${BASE_BRANCH}
- **Description**: ${PR_BODY}

## Diff
\`\`\`diff
${DIFF}
\`\`\`

## Full File Contents
${FILE_CONTENTS}

## Recent Commit History
${COMMIT_HISTORY}

## Existing Review Comments
${EXISTING_COMMENTS}

Run the full 'review' command as defined in your protocol."

  local response
  response=$(call_claude "$system_prompt" "$user_prompt")

  post_comment "$response"
  log "Review posted"
}

handle_synchronize() {
  log "Handling push to PR (incremental re-review)"
  assemble_context

  local system_prompt
  system_prompt=$(read_system_prompt)

  local user_prompt="The developer has pushed new changes to this PR. Run the 'check' command.

## PR Metadata
- **Title**: ${PR_TITLE}
- **Author**: ${PR_AUTHOR}
- **Branch**: ${HEAD_BRANCH} -> ${BASE_BRANCH}

## Current Diff (full)
\`\`\`diff
${DIFF}
\`\`\`

## Full File Contents
${FILE_CONTENTS}

## Previous Review Comments (includes your earlier findings)
${EXISTING_COMMENTS}

Compare the current state against your previous findings. For each:
- RESOLVED: explicitly mark it
- PARTIALLY FIXED: explain what's still missing
- UNRESOLVED: note it's still present
- NEW ISSUE: flag anything introduced by the fixes

End with an updated summary."

  local response
  response=$(call_claude "$system_prompt" "$user_prompt")

  post_comment "$response"
  log "Incremental review posted"
}

handle_comment() {
  log "Handling comment: ${COMMENT_BODY:0:80}..."
  assemble_context

  local system_prompt
  system_prompt=$(read_system_prompt)

  # Extract the command from the comment (strip @review-agent prefix)
  local command
  command=$(echo "$COMMENT_BODY" | sed 's/@review-agent[[:space:]]*//')

  local user_prompt="A developer is asking you a follow-up question on this PR.

## PR Metadata
- **Title**: ${PR_TITLE}
- **Author**: ${PR_AUTHOR}
- **Branch**: ${HEAD_BRANCH} -> ${BASE_BRANCH}

## Current Diff
\`\`\`diff
${DIFF}
\`\`\`

## Full File Contents
${FILE_CONTENTS}

## Conversation History (all previous comments on this PR)
${EXISTING_COMMENTS}

## Developer's Question
${command}

Respond according to your protocol. If they're asking about a specific finding, trace the code to give a precise answer. If they disagree with you, engage with their reasoning."

  local response
  response=$(call_claude "$system_prompt" "$user_prompt")

  post_comment "$response"
  log "Reply posted"
}

# ── Main ─────────────────────────────────────────────────────────────────────

main() {
  log "Event: ${EVENT_TYPE}, PR: #${PR}, Repo: ${REPO}"

  case "${EVENT_TYPE}" in
    opened)
      handle_opened
      ;;
    synchronize)
      handle_synchronize
      ;;
    comment)
      if echo "$COMMENT_BODY" | grep -qi "@review-agent"; then
        handle_comment
      else
        log "Comment doesn't mention @review-agent, skipping"
      fi
      ;;
    *)
      log "Unknown event type: ${EVENT_TYPE}"
      exit 1
      ;;
  esac
}

main
