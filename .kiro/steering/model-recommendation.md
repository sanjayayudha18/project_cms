# Per-Task Claude Model Recommendation (Kiro Steering Doc)

> Every task in a spec task file MUST carry a recommended Claude model tier. This lets whoever runs the task pick the right cost/capability tradeoff instead of defaulting to the most expensive model for trivial work (or the cheapest for risky money/auth work).

---

## The Rule

- Every discrete task in a spec's `task.md` / `tasks.md` gets a **`Model:`** note recommending one of three Claude tiers.
- Recommend by **tier**, not a pinned version string. Anthropic renames releases often (Opus/Sonnet/Haiku generations shift); a tier keeps the guidance stable. Map the tier to whatever the current Opus/Sonnet/Haiku release is at run time.
- One note per top-level task. Sub-tasks inherit the parent's tier unless a sub-task is clearly heavier or lighter, in which case give that sub-task its own note.
- Keep the note to one line, and add a short `— why` clause so the reader understands the call.

---

## The Three Tiers

| Tier | Claude model (map to current release) | Use for |
| ---| ---| --- |
| **Haiku** | Fastest / cheapest tier | Mechanical, low-risk, well-specified work: scaffolding, boilerplate, renames, doc/comment edits, simple config, nav registration, copy changes. Little judgment needed. |
| **Sonnet** | Balanced default tier | The default for most implementation: handlers, services, queries, migrations, React components, hooks, wiring, tests. Real work, but within known patterns. When unsure, pick Sonnet. |
| **Opus** | Highest-capability tier | High-risk or high-ambiguity work where a mistake is expensive: money math, state machines, reconciliation logic, maker-checker/approval flows, auth, data migrations touching live data, cross-module orchestration, or design decisions that ripple. |

**Default to Sonnet.** Reserve Opus for the CMS golden-rule danger zones (money/journal, reconciliation, auth, data migrations, maker-checker — see `project-context.md` Sec 3 rule 7). Reserve Haiku for work with essentially no judgment.

---

## How to Write the Note

**Plural `tasks.md` (Kiro checklist format)** — add as an indented bullet under the task, alongside the `_Requirements:_` line:

```
- [ ] 3. Backend: service layer — state machine, guards, audit
  - ...
  - _Requirements: 2.2, 2.6_
  - _Model: Opus — state-machine transitions + maker-checker guards; a wrong transition corrupts approval state._
```

**Singular `task.md` (hand-written heading format)** — add as a bullet in the task body, next to Test/Demo:

```
### [ ] Task 6 — Approval orchestrator (submit/approve/reject)
- ...
- **Test:** ...
- **Demo:** ...
- **Model:** Opus — maker-checker orchestration + idempotency; money/approval correctness is critical.
```

---

## Quick Heuristic

Ask: *if the model gets this wrong, what breaks?*

- Breaks money, auth, approval, reconciliation, or live data → **Opus**.
- Breaks a feature (fixable, contained, caught by tests) → **Sonnet**.
- Breaks almost nothing / obvious on review → **Haiku**.

---

## Conventions

- Add a `Model:` note to every new task when authoring a spec. A task file without model notes is incomplete.
- Checkpoint / verification tasks (build, lint, test gates) are usually **Sonnet** — they need judgment to triage failures, but rarely Opus-level reasoning.
- Update the note if a task's scope changes materially (e.g. a "simple config" task grows to touch auth → bump to Opus).
