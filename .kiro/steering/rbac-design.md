---
inclusion: auto
name: rbac-design
description: RBAC hierarchy and multi-layer maker-checker approval design for CMS internal — data model, approval flow, and request lifecycle diagrams. Use when working on RBAC, roles, approval, maker-checker, supervisor hierarchy, delegation, or audit logging.
---

# RBAC Design — Hierarki & Multi-Layer Approval (Internal)

> UML diagrams (Mermaid, mermaid.live compatible) for the RBAC + hierarchy + maker-checker approval design.
> Source spec: `.kiro/specs/RBAC-Setup/task.md`. Scope: internal CIMB first (vendor later).

## Design Principles

- **Role vs Hierarchy are separate concerns.** `Role` = capability (what a user may do). `supervisor_id` + `approval_level` = reporting structure (who reports to whom).
- **Pure tree hierarchy.** Each user has at most one direct supervisor (`supervisor_id`). Multi-supervisor (matrix) is out of scope.
- **Threshold-driven levels.** `approval_policies` maps `(document_type, amount range) -> required_level`, admin-configurable.
- **Block-leave fallback.** An approver on leave (`user_leaves` overlapping now) is routed to an active delegate (`approval_delegations`), guarded by `delegate != maker`.
- **Non-negotiable.** maker != checker, every transition writes `audit_logs` (who/what/before/after/ip), writes go to primary, idempotent per `(document_type, document_id)`.

---

## 1. Class Diagram — RBAC, Hierarchy & Approval

```mermaid
classDiagram
    class Role {
        +bigint id
        +string role
        +string description
    }

    class User {
        +bigint id
        +bigint role_id
        +bigint supervisor_id
        +int approval_level
        +bigint vendor_id
        +bigint vendor_branch_id
        +bool is_karyawan
        +string auth_source
        +bool is_active
    }

    class ApprovalPolicy {
        +bigint id
        +string document_type
        +numeric min_amount
        +numeric max_amount
        +int required_level
        +bool is_active
    }

    class ApprovalRequest {
        +bigint id
        +bigint maker_id
        +string document_type
        +bigint document_id
        +numeric amount
        +int required_level
        +string status
    }

    class ApprovalStep {
        +bigint id
        +bigint request_id
        +int step_level
        +bigint assigned_approver_id
        +bigint acted_by_id
        +string status
        +timestamptz acted_at
    }

    class ApprovalDelegation {
        +bigint id
        +bigint from_user_id
        +bigint to_user_id
        +timestamptz start_at
        +timestamptz end_at
        +string reason
    }

    class UserLeave {
        +bigint id
        +bigint user_id
        +timestamptz start_at
        +timestamptz end_at
        +string reason
    }

    class AuditLog {
        +bigint id
        +bigint actor_id
        +string action
        +string entity_type
        +bigint entity_id
        +jsonb before
        +jsonb after
        +string ip
        +timestamptz created_at
    }

    User "1" --> "1" Role : role_id
    User "0..*" --> "0..1" User : supervisor_id (hierarchy)
    ApprovalRequest "1" --> "1" User : maker_id
    ApprovalRequest "1" *-- "1..*" ApprovalStep : steps
    ApprovalStep "0..*" --> "1" User : assigned_approver_id
    ApprovalStep "0..*" --> "0..1" User : acted_by_id
    ApprovalDelegation "0..*" --> "1" User : from_user_id
    ApprovalDelegation "0..*" --> "1" User : to_user_id
    UserLeave "0..*" --> "1" User : user_id
    AuditLog "0..*" --> "1" User : actor_id
    ApprovalPolicy ..> ApprovalRequest : determines required_level
```

---

## 2. Sequence Diagram — Multi-Layer Approval (with leave fallback)

```mermaid
sequenceDiagram
    actor Maker
    participant API as ApprovalHandler
    participant Orch as ApprovalService
    participant Pol as ApprovalPolicy
    participant Res as ApproverResolver
    participant DB as approval_requests / steps
    participant Aud as AuditLog
    actor Approver

    Maker->>API: POST /submit (document_type, document_id, amount)
    API->>Orch: SubmitForApproval(...)
    Orch->>Pol: lookup(document_type, amount)
    Pol-->>Orch: required_level (e.g. L3)
    Orch->>Res: buildChain(maker.supervisor_id .. required_level)
    Res-->>Orch: [step L2, step L3]
    Orch->>DB: create request(pending) + steps
    Orch->>Aud: write(action=submit, before=null, after=request)
    Orch-->>API: request_id, status=pending
    API-->>Maker: 201 { id, status }

    Note over Approver,Res: Each step rises through the chain
    Approver->>API: POST /{id}/approve
    API->>Orch: Approve(request_id, actor)
    Orch->>Res: effectiveApprover(step)
    alt approver on leave (user_leaves overlaps now)
        Res->>Res: find active delegate (approval_delegations)
        Res-->>Orch: delegate (guard: delegate != maker)
    else approver available
        Res-->>Orch: direct approver
    end
    Orch->>Orch: validate actor == effective approver AND actor != maker
    Orch->>DB: step approved
    Orch->>Aud: write(action=approve, step_level)
    alt higher step remains
        Orch-->>API: status=pending (advance to next step)
    else final step
        Orch->>DB: request approved
        Orch->>Aud: write(action=final_approve)
        Orch-->>API: status=approved (document effect applies)
    end
    API-->>Approver: 200 { status }
```

---

## 3. State Diagram — ApprovalRequest Lifecycle

```mermaid
stateDiagram-v2
    [*] --> draft
    draft --> pending : submit (policy -> required_level, generate steps)
    pending --> pending : approve step (higher level remains)
    pending --> approved : approve final step
    pending --> rejected : reject at any step
    approved --> [*]
    rejected --> [*]
```
