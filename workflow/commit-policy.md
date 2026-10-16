# Commit Policy

Commits are concise historical records of meaningful changes, not implementation
reports. Use a short message that describes the outcome or intent of the change.
Do not use commit messages as a substitute for durable documentation.

Put durable system context in the relevant files under `features/`.
Put reusable workflow lessons in the appropriate files under `workflow/`.
Keep task-specific investigation and decisions with the relevant work item
unless they establish knowledge that should be retained elsewhere.

## Completing tasks

- Commit completed task changes after appropriate verification.
- Push completed task commits when a suitable configured remote is available
  and the current branch can be pushed safely.
- Check the current branch, remotes, and upstream before pushing.
- If the target remote is ambiguous, authentication fails, or the push is rejected,
  report the blocker rather than guessing, force-pushing, or changing remote settings.
- Do not commit unrelated changes or overwrite another contributor's work.
- Inspect the working tree and diff before staging. Stage only the intended changes.
- Run the relevant tests and checks before committing when the environment permits.
- If verification cannot be completed, state that limitation accurately.
- Never claim a commit or push succeeded without verifying the result.

Task lifecycle and Git history are separate: move a task to `tasks/closed/`
only when its acceptance criteria are met. Record any outstanding limitations
or follow-up work explicitly.
