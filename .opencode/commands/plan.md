---
description: Create CMS implementation plan with risk assessment
agent: planner
subtask: true
---

# Plan Command

Create a detailed implementation plan for: $ARGUMENTS

Follow the CMS planner agent rules (`.opencode/agent/planner.md` / `.opencode/prompts/agents/planner.txt`).

## Your Task

1. **Read project context** — `.opencode/opencode.md` is source of truth
2. **Restate requirements** — clarify what will be built and out of scope
3. **Inspect the codebase** — use real paths, modules, and existing patterns
4. **Identify risks** — auth, money, maker-checker, migrations, replica routing, contracts
5. **Create step plan** — phased, file-specific, independently verifiable
6. **Wait for confirmation** — MUST receive user approval before any implementation

## Hard Rules

- Do not invent endpoints, tables, columns, env vars, modules, or dependencies
- Flag new table/column/module/dependency for approval before planning them in
- Planning is read-only: no file edits until the user says yes / proceed
- Preserve unrelated worktree changes

## Output

Use the CMS planner plan format (Outcome, Scope, Evidence, Steps, Testing, Risks, Definition of Done).

**WAITING FOR CONFIRMATION**: Proceed with this plan? (yes / no / modify)
