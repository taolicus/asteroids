# Work Item Metadata

Keep metadata minimal. No metadata field is mandatory.

## Supported fields

### area

- Optional single slug identifying a meaningful project area.
- Reuse existing convention slugs where possible.
- Introduce a new slug only when it provides a useful, recurring distinction.
- Omit the field when it adds no practical value.
- Do not create directory structures based on area values.

### dependencies

- Optional list of work-item filenames that must be completed or resolved
  before the current task can proceed.
- Reference filenames, not titles or paths.
- Keep work-item filenames unique across issues/, tasks/open/, and
  tasks/closed/ so references are unambiguous.
- Each reference must resolve to an existing work item in one of those
  directories.
- Before implementation, inspect the referenced items and determine whether
  their dependencies have been satisfied.
- Preserve filenames when moving items through their lifecycle. If a file
  is renamed or removed, update references to it.
- Omit the field when no dependencies exist.

Use dependencies only for genuine execution constraints, not merely related
work. Put rationale, context, and other constraints in the item description.

## Priority

Do not store priority as work-item metadata. Decide what to work on during
planning conversations, based on current goals, constraints, available time,
device access, and dependencies.

## Example

---
area: authentication
dependencies:
  - define-authentication-contract.md
  - migrate-session-storage.md
---

Both fields are optional. Omit them when they do not add value.
