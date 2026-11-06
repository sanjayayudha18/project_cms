---
name: cms-run-wave
description: Dispatch one wave from a CMS2 spec's "## Task Dependency Graph" — one new Claude session per task, each on the model + effort its `_Model:` line names, each following the cms-task-runner agent. Use when the user says "jalankan wave N <spec>" / "run wave N".
---

Run this from an Opus session: a new session can only use a model the same as or cheaper than its caller, so Opus tasks need an Opus caller.

Input: spec folder (e.g. `.kiro/specs/vendor-pks/`) + wave number. Missing → ask.

1. Read `tasks.md`: the JSON block under "## Task Dependency Graph" gives the wave's task ids.
2. Gate: every task of earlier waves must be `[x]`. Not → list the open ones and stop (unless the user says to go anyway).
3. Per task id, find its model + effort: the leaf's `_Model:` line, else its parent's (e.g. `1.1` inherits parent 1 = Opus/High; `1.3` has its own Sonnet/Medium). Map: Opus → `claude-opus-5-5`, Sonnet → `claude-sonnet-5-5`, Haiku → `claude-haiku-5-5`; effort Low/Medium/High → `low`/`medium`/`high`. Skip (and tell the user) a task that is already `[x]` or is the user's manual check.
4. Per task, pick recommended skills from the "Skill map" in `.claude/agents/cms-task-runner.md` by what the task block touches (files, tests, UI, master data).
5. Show the user a table (task · title · model · effort · skills) and wait for OK. The user may add/remove skills per task; use their final list.
6. On OK: `mark_chapter` "Wave N <spec>", then one `start_session` per task: `initiation: "user_asked"`, `context: "fresh"`, `use_worktree: false` (later waves need these uncommitted files), `model`, `effort`, `name: "<spec>-t<id>"` (dots → dashes), title `"<spec> task <id>: <short title>"`. Prompt:
   > Act as the `cms-task-runner` agent: read and follow `.claude/agents/cms-task-runner.md` yourself (do not spawn it as a subagent). spec = `<spec>`, task = `<id>`, skills = `<approved list>` (invoke these before coding). Same-wave tasks run in parallel in this folder: touch only this task's files, commit nothing, switch no branch.
   `background`: the spec, that this is wave N of its dependency graph, and that the user commits manually afterwards.
7. When sessions report back: check each tick in `tasks.md` + its report, summarise the wave (done / blocked / skipped tests), and propose the next wave. Never commit.
