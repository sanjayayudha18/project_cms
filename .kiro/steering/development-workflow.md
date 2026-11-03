---
inclusion: auto
description: Development workflow guidelines for planning, TDD, code review, and commit pipeline
---

# Development Workflow

> This rule extends the git workflow rule with the full feature development process that happens before git operations.

The Feature Implementation Workflow describes the development pipeline: planning, TDD, code review, and then committing to git.

## Feature Implementation Workflow

1. **Plan First**
   - Use **planner** agent to create implementation plan
   - Identify dependencies and risks
   - Break down into phases

   - **Every `requirements.md` MUST be authored via the planner agent** (`invoke_sub_agent` name `planner`, the "/planner"). Do not write or rewrite a `requirements.md` directly in the main session.
     - Applies to new specs and to material revisions (new/changed requirements) under `.kiro/specs/<feature>/` and `.claude/sdlc/<feature>/`.
     - Pass the planner the feature intent, the relevant `project-context.md` rules, and the target path. Review its output, then save it to the correct location (see the spec-location memory for CMS2).
     - Typo or formatting fixes may be edited directly.
     - After requirements, `design.md` and `tasks.md` still follow their own rules (`tasks.md` must carry the Model Split from `model-recommendation.md`).

   - **Every `design.md` MUST be authored via the architect agent** (`invoke_sub_agent` name `architect`).
     - If the design adds or changes tables, columns, indexes, or queries, run **database-reviewer** on the schema section after architect finishes.
     - Pass the architect the approved `requirements.md`, the relevant `project-context.md` / `tech.md` / `structure.md` rules, and the target path.
     - Small single-layer changes with no new schema may be written directly in the main session.
   - **Every `task.md` / `tasks.md` MUST be authored via the planner agent** (`invoke_sub_agent` name `planner`), then checked by **tdd-guide** for task order.
     - Pass the planner the approved `requirements.md` and `design.md`, plus `model-recommendation.md`, so it produces per-task `Model:` + `Effort:` notes and the Model Split (Sonnet-runnable vs Opus-required).
     - tdd-guide verifies tests come before implementation in each task and that the 80% coverage target is covered.
     - If writing tasks exposes an undecided design question, stop and revise `design.md` (via architect) first. Do not decide it inside `tasks.md`.
     - Checkbox status updates and small edits to an existing task file may be done directly.

2. **TDD Approach**
   - Use **tdd-guide** agent
   - Write tests first (RED)
   - Implement to pass tests (GREEN)
   - Refactor (IMPROVE)
   - Verify 80%+ coverage

3. **Code Review**
   - Use **code-reviewer** agent immediately after writing code
   - Address CRITICAL and HIGH issues
   - Fix MEDIUM issues when possible

4. **Commit & Push**
   - Detailed commit messages
   - Follow conventional commits format
   - See the git workflow rule for commit message format and PR process
