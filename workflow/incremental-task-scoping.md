# Incremental Task Scoping

Break substantial work into the smallest set of coherent tasks that improves
reviewability, reduces risk, or enables useful progress independently.
Avoid both oversized tasks and unnecessary task fragmentation.

## When to split

Split work when one or more of these conditions apply:
- A meaningful part can be implemented, verified, and reviewed independently.
- The work contains distinct outcomes with different acceptance criteria.
- A step resolves significant uncertainty before a larger implementation begins.
- Separating changes reduces regression risk or makes failures easier to diagnose.
- Different steps have real dependencies or can be completed independently.
- The change is large enough that reviewing it as one unit would be difficult.

Do not split work merely because it touches multiple files, layers, or technical
steps. Keep tightly coupled changes together when splitting would create artificial
boundaries or provide no meaningful review or verification benefit.

## Define each task

Each task should describe:
- One coherent outcome.
- The scope and important constraints.
- Observable acceptance criteria.
- Appropriate tests or other verification.
- Dependencies, when they genuinely constrain execution.

Keep the number of tasks proportional to the work. A small, routine change should
usually remain one task. Do not create separate tasks for trivial edits, routine
substeps, or every file touched. Reassess the breakdown after investigation if the
actual complexity differs from the initial estimate.

## Preserve repository coherence

Every completed task should leave the repository in a coherent, reviewable state.
Where applicable, it should build and pass the relevant tests. Avoid intermediate
states that leave the main code path broken, required interfaces inconsistent,
or unrelated functionality unusable.

Order dependent tasks so that each step leaves a valid state. Use compatibility
layers, incremental migrations, feature flags, or temporary adapters when they
make intermediate states safe and the extra complexity is justified.

If work cannot be divided into independently coherent outcomes without excessive
scaffolding or artificial boundaries, keep the tightly coupled changes in one
task. Do not pretend that an unsafe intermediate state is independently complete.

## Review and completion

Make each task small enough for a focused review, but large enough to deliver
a meaningful change. Verify its acceptance criteria before treating it as complete.
Commit completed task changes according to `commit-policy.md`.

If implementation reveals additional independent work, create or update the
appropriate issue or task instead of silently expanding scope. Record dependencies
when they affect execution order. Keep the overall breakdown as simple as possible.
