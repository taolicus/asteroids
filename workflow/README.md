# Workflow

## Work management

- issues/: Unresolved observations, problems, questions, and investigations.
- tasks/open/: Actionable work with sufficient context, constraints, and
  acceptance criteria.
- tasks/closed/: Completed work retained for history.
- features/: Durable knowledge about the implemented system.

## Lifecycle

1. Move an issue to tasks/open/ when it becomes actionable.
2. Move a completed task to tasks/closed/.
3. Preserve the work-item file and its context when moving it; do not
   duplicate work items.
4. Update relevant feature documentation when completed work establishes
   durable knowledge.
5. Never move tasks into features/.

Issues need not become tasks until they are sufficiently understood.
Keep priority, area, dependencies, and other metadata out of directory names.

## Implementation

Before implementing work, check tasks/open/ and relevant feature
documentation. If a task is insufficiently specified, investigate and
update it rather than making assumptions.

Record unrelated discoveries as new issues instead of silently expanding
the current task's scope.

## Workflow rules

- See work-item-metadata.md for metadata conventions.
- See device-aware-development.md for device constraints and handoffs.

Place additional workflow and agent-conduct rules in individual Markdown
files alongside this README. Create them only when needed.

## Additional workflow rules
- `workflow/work-item-metadata.md`
- `workflow/device-aware-development.md`
- `workflow/commit-policy.md`
- `workflow/incremental-task-scoping.md`
