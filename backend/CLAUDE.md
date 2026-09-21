## graphify

This project has a knowledge graph at graphify-out/ with god nodes, community structure, and cross-file relationships.

Rules:
- For codebase questions, first run `graphify query "<question>"` when graphify-out/graph.json exists. Use `graphify path "<A>" "<B>"` for relationships and `graphify explain "<concept>"` for focused concepts. These return a scoped subgraph, usually much smaller than GRAPH_REPORT.md or raw grep output.
- If graphify-out/wiki/index.md exists, use it for broad navigation instead of raw source browsing.
- Read graphify-out/GRAPH_REPORT.md only for broad architecture review or when query/path/explain do not surface enough context.
- After modifying code, run `graphify update .` to keep the graph current (AST-only, no API cost).

## sqlc (pinned version + known codegen quirks)

**Pinned version: `sqlc v1.31.1`** (matches the `versions:` header already embedded in every committed `internal/db/*.sql.go` file). Install/verify with:
```
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
sqlc version   # must print v1.31.1
```
Do not run `sqlc generate` with a different installed version — a mismatch was found and fixed 2026-09-18 (see `.claude/sdlc/master-data/plan.md` T1.8 Catatan implementasi for the investigation).

**Known, version-independent codegen quirks** (confirmed reproducible even on the pinned v1.31.1, from unmodified schema/queries — not a version-drift issue):
1. **`audit_logs.ip` column** → sqlc's initialism list capitalizes it as `IP` in some contexts but not others, non-deterministically across runs. **Fixed permanently** via `sql[].gen.go.rename: {ip: "IP"}` in `sqlc.yaml` — do not remove this override.
2. **`user_leaves` table** → sqlc's pluralization engine mis-singularizes it to `UserLeafe` (wrongly applies the knife/knives "-fe" rule to the irregular leaf/leaves). This is a struct-type-name issue, **not fixable via `sql[].gen.go.rename`** (rename only affects column-derived field names, confirmed by testing — table→struct-name uses a separate code path). **After every `sqlc generate`, hand-fix `UserLeafe` → `UserLeave`** in `internal/db/models.go` and any regenerated file under `internal/db/` that references it (currently `approval.sql.go`). `internal/approval/repository.go` and `admin.go` depend on `db.UserLeave` existing.

**Before committing any `sqlc generate` output**: run `git diff --stat backend/internal/db/` and check every changed file is one you expected from your own `queries/*.sql` or `migrations/*.sql` edit. A full regenerate touches every table's struct — unrelated hand-fixes (like #2 above) are easy to accidentally revert if you `git checkout` blindly, and unrelated stale-vs-source drift in files you didn't mean to touch can surface too (seen once in `auth.sql.go` — a `queries/auth.sql` edit that was never regenerated and committed; unrelated to this pin, flagged separately, do not fix opportunistically inside an unrelated change).
