# RBAC Design Diagrams — Hierarki & Multi-Layer Approval (Internal)

> Diagram UML (format Mermaid, kompatibel mermaid.live) untuk desain RBAC + hierarki + approval maker-checker.
> Companion `task.md` di folder yang sama. Scope: internal CIMB dulu.
>
> Cara pakai di mermaid.live: copy isi satu blok kode (tanpa pagar ```` ``` ````) ke editor.

---

## 1. Class Diagram — Struktur RBAC, Hierarki & Approval

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
    User "0..*" --> "0..1" User : supervisor_id (hierarki)
    ApprovalRequest "1" --> "1" User : maker_id
    ApprovalRequest "1" *-- "1..*" ApprovalStep : steps
    ApprovalStep "0..*" --> "1" User : assigned_approver_id
    ApprovalStep "0..*" --> "0..1" User : acted_by_id
    ApprovalDelegation "0..*" --> "1" User : from_user_id
    ApprovalDelegation "0..*" --> "1" User : to_user_id
    UserLeave "0..*" --> "1" User : user_id
    AuditLog "0..*" --> "1" User : actor_id
    ApprovalPolicy ..> ApprovalRequest : menentukan required_level
```

---

## 2. Sequence Diagram — Alur Approval Bertingkat (dengan fallback cuti)

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
    Pol-->>Orch: required_level (mis. L3)
    Orch->>Res: buildChain(maker.supervisor_id .. required_level)
    Res-->>Orch: [step L2, step L3]
    Orch->>DB: create request(pending) + steps
    Orch->>Aud: write(action=submit, before=null, after=request)
    Orch-->>API: request_id, status=pending
    API-->>Maker: 201 { id, status }

    Note over Approver,Res: Tiap step naik bertingkat
    Approver->>API: POST /{id}/approve
    API->>Orch: Approve(request_id, actor)
    Orch->>Res: effectiveApprover(step)
    alt approver sedang cuti (user_leaves overlap now)
        Res->>Res: cari delegate aktif (approval_delegations)
        Res-->>Orch: delegate (guard: delegate != maker)
    else approver tersedia
        Res-->>Orch: approver langsung
    end
    Orch->>Orch: validasi actor == approver efektif AND actor != maker
    Orch->>DB: step approved
    Orch->>Aud: write(action=approve, step_level)
    alt masih ada step di atas
        Orch-->>API: status=pending (naik ke step berikut)
    else step terakhir
        Orch->>DB: request approved
        Orch->>Aud: write(action=final_approve)
        Orch-->>API: status=approved (efek dokumen berlaku)
    end
    API-->>Approver: 200 { status }
```

---

## 3. State Diagram — Lifecycle ApprovalRequest

```mermaid
stateDiagram-v2
    [*] --> draft
    draft --> pending : submit (policy -> required_level, generate steps)
    pending --> pending : approve step (masih ada tingkat di atas)
    pending --> approved : approve step terakhir
    pending --> rejected : reject di step mana pun
    approved --> [*]
    rejected --> [*]
```

---

## Catatan Desain

- **Role vs Hierarki**: `Role` menentukan kapabilitas (apa yang boleh dilakukan); `supervisor_id` + `approval_level` menentukan struktur pelaporan (siapa lapor ke siapa). Dua konsep terpisah.
- **Pohon murni**: satu user punya paling banyak satu atasan langsung (`supervisor_id`). Multi-atasan (matriks) di luar scope.
- **Threshold**: `ApprovalPolicy` memetakan `(document_type, rentang nilai) -> required_level`, configurable admin.
- **Fallback block leave**: approver cuti (`UserLeave` overlap sekarang) dialihkan ke delegate aktif (`ApprovalDelegation`), dengan guard `delegate != maker`.
- **Non-negosiable**: maker != checker, tiap transisi tulis `AuditLog` (who/what/before/after/ip), write ke primary, idempotent per `(document_type, document_id)`.
