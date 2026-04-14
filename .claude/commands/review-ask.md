Follow-up question about the review: $ARGUMENTS

## Steps

1. Load state from `.pr-review/reviews/`.
2. If the question references a finding (e.g. "finding 2" or "#2"), load that finding's full context and discussion history.
3. Read the relevant code files.
4. Answer the question using the loaded skill knowledge and code tracing.
5. If the question leads to a finding status change (e.g. developer convinces you to retract), update the state.
6. Add to the finding's discussion thread if applicable.
7. Save state.
