# Device-Aware Development

## Available devices

- Google Pixel 7 Pro: usually available, including during commutes and
  short periods of spare time.
- M1 MacBook Pro: stays at home, is shared, and may not always be available.

Use these constraints when they are relevant to the current work session.
Do not assume a device is available unless the conversation or environment
establishes that it is.

## Choosing suitable work

Prefer bounded tasks on the phone when practical, such as:

- Reviewing code, diffs, task descriptions, and documentation.
- Investigating issues and gathering evidence.
- Refining acceptance criteria and implementation plans.
- Making small, low-risk edits that can be reviewed and verified.

Defer laptop-dependent work when the required environment is unavailable.
Examples include local builds, substantial test runs, debugger or simulator
work, and large repository changes that are impractical to review or validate
on the phone.

Do not assume that all coding must be deferred on the phone. Assess the actual
scope, tooling, risk, and ability to verify the result.

## Device constraints in work items

- Do not add a required device metadata field.
- Record a device requirement in the task description only when it materially
  constrains execution or verification.
- Do not add device information to tasks merely for completeness.
- Device suitability can inform planning, but it does not create a stored
  priority or change the work-item lifecycle.

## Cross-device handoffs

When work must continue on another device:

1. Leave the work item and repository in a coherent, understandable state.
2. Record findings, decisions, changes made, and unresolved questions in the
   relevant task or issue.
3. State the next concrete action and identify any step that requires the
   other device.
4. Preserve uncommitted changes carefully; do not discard work or claim it is
   safely available on another device unless that has been verified.
5. Avoid repeating investigations or work already documented.

Keep handoff notes concise and proportional to the amount of work performed.
