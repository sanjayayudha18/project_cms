# Per-Task Claude Model Recommendation (Kiro Steering Doc)

> Every task in a spec task file MUST carry a recommended Claude model tier **and a reasoning effort level**. This lets whoever runs the task pick the right cost/capability tradeoff instead of defaulting to the most expensive model (or the deepest thinking budget) for trivial work — or the cheapest/shallowest for risky money/auth work.

---

## The Rule

- Every discrete task in a spec's `task.md` / `tasks.md` gets a **`Model:`** note recommending one of three Claude tiers **plus one of three effort levels** (e.g. `Model: Sonnet, Effort: Medium`).
- Recommend by **tier**, not a pinned version string. Anthropic renames releases often (Opus/Sonnet/Haiku generations shift); a tier keeps the guidance stable. Map the tier to whatever the current Opus/Sonnet/Haiku release is at run time.
- **Tier** picks the model's raw capability/cost. **Effort** picks how much reasoning budget it spends on that task. They are independent: a cheap tier can still run at high effort on a fiddly problem, and a strong tier can run at low effort on something mechanical.
- One note per top-level task. Sub-tasks inherit the parent's tier and effort unless a sub-task is clearly heavier or lighter, in which case give that sub-task its own note.
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

## The Three Effort Levels

Effort is the reasoning/thinking budget the model spends before and while writing the answer. It is orthogonal to tier: choose it by how much the task rewards deliberation, not by how important the task is.

| Effort | Use for |
| ---| ---|
| **Low** | Single-answer, well-specified work with one obvious path: boilerplate, renames, copy/doc edits, nav registration, mechanical config. More thinking would not change the output. |
| **Medium** | The default. Work with a few moving parts or a couple of viable approaches to weigh: most handlers, services, queries, migrations, components, and tests. |
| **High** | Work that rewards deep deliberation: multi-step reasoning, tricky edge cases, state machines, reconciliation/money math, cross-module orchestration, subtle concurrency, or design decisions with several competing tradeoffs. |

**Default to Medium.** Raise to High when getting it right needs the model to reason through branches and edge cases (this often — but not always — lines up with an Opus tier). Drop to Low when the answer is essentially determined by the spec and extra thinking is wasted cost/latency.

> Tier and effort usually correlate (Haiku/Low, Sonnet/Medium, Opus/High) but decouple them when it helps: a mechanical Opus-tier task can run Low effort, and a fiddly bug on a cheap tier can warrant High effort.

---

## How to Write the Note

**Plural `tasks.md` (Kiro checklist format)** — add as an indented bullet under the task, alongside the `_Requirements:_` line:

```
- [ ] 3. Backend: service layer — state machine, guards, audit
  - ...
  - _Requirements: 2.2, 2.6_
  - _Model: Opus, Effort: High — state-machine transitions + maker-checker guards; a wrong transition corrupts approval state._
```

**Singular `task.md` (hand-written heading format)** — add as a bullet in the task body, next to Test/Demo:

```
### [ ] Task 6 — Approval orchestrator (submit/approve/reject)
- ...
- **Test:** ...
- **Demo:** ...
- **Model:** Opus, **Effort:** High — maker-checker orchestration + idempotency; money/approval correctness is critical.
```

---

## Mandatory Model Split Section (Sonnet vs Opus)

Every `task.md` / `tasks.md` MUST open (right after the title/overview, before the first task) with a **Model Split** section that explicitly separates the tasks Sonnet can run from the tasks that need Opus. The per-task `Model:` note stays; this section is the at-a-glance summary so whoever runs the spec can batch work per model without reading every task.

Rules:
- Two groups, always both present (write `— none` if a group is empty):
  - **Sonnet-runnable** — every task tagged Sonnet or Haiku (Haiku tasks are listed here with a `(Haiku)` marker; they can always run on Sonnet).
  - **Opus-required** — every task tagged Opus. Each entry needs a short reason (which danger zone: money, auth, approval/maker-checker, reconciliation, live-data migration, cross-module orchestration, rippling design decision).
- Every task appears in exactly one group. The group must match the task's own `Model:` note; if they disagree, the task note is wrong or the split is stale. Fix both together.
- If an Opus task has sub-tasks that are plain Sonnet work (e.g. tests, UI wiring under an Opus service task), list those sub-tasks under Sonnet-runnable by sub-task number.
- Note dependencies that cross groups (e.g. "Sonnet 5 needs Opus 3 done first") so the order of execution stays clear.
- Update the split whenever a task is added, removed, or re-tiered.

Format:

```
## Model Split

**Sonnet-runnable**
- 1. Migration: add columns — Sonnet, Low
- 2. Repository queries — Sonnet, Medium
- 5. Frontend list screen — Sonnet, Medium (needs 3)
- 6. Nav registration — Haiku, Low (Haiku)
- 3.4 Service unit tests — Sonnet, Medium

**Opus-required**
- 3. Service state machine + maker-checker guards — Opus, High — approval correctness
- 4. Money aggregation per vendor — Opus, High — money math
```

---

## Quick Heuristic

**Tier** — ask: *if the model gets this wrong, what breaks?*

- Breaks money, auth, approval, reconciliation, or live data → **Opus**.
- Breaks a feature (fixable, contained, caught by tests) → **Sonnet**.
- Breaks almost nothing / obvious on review → **Haiku**.

**Effort** — ask: *would more thinking change the output?*

- Many branches, edge cases, or competing tradeoffs to reason through → **High**.
- A few moving parts or a couple of viable approaches → **Medium**.
- One obvious path, answer determined by the spec → **Low**.

---

## Conventions

- Add a `Model:` **and** `Effort:` note to every new task when authoring a spec. A task file missing either is incomplete.
- A task file without the **Model Split** section (Sonnet-runnable vs Opus-required) is also incomplete.
- Checkpoint / verification tasks (build, lint, test gates) are usually **Sonnet, Medium** — they need judgment to triage failures, but rarely Opus-level reasoning or High effort.
- Update the note if a task's scope changes materially (e.g. a "simple config" task grows to touch auth → bump tier to Opus and effort to High).
