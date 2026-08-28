---
description: Simple explanations of complex technical concepts
---

# ELI5 — Explain Like I'm 5

A reference guide for breaking down complex technical concepts into simple, non-technical language. Use this when onboarding teammates, explaining to stakeholders, or debugging misunderstandings.

## How to Use This

1. **When explaining a concept**, find it below or use the template at the end
2. **Keep it concrete**: use real examples from the product
3. **Avoid jargon**: if you must use a term, explain it in one sentence first
4. **Use analogies**: relate to everyday things (bank accounts, mailboxes, recipes)
5. **Test understanding**: ask "does that make sense?" and listen for the actual confusion

---

## When to Use Analogies

**Good analogies**:
- "A database is like a filing cabinet"
- "An API is like a restaurant waiter"
- "Authorization is like a keycard to rooms"

**Bad analogies**:
- "Recursion is like a fractal" (not simpler)
- "Blockchain is like a chain of blocks" (circular)
- Analogies that only work if you already understand the concept

**Rule**: If the analogy requires more explanation than the original concept, skip it.

---

## Glossary (Quick Reference)

| Term | One-liner |
|------|-----------|
| **JWT** | A tamper-proof badge that proves you're logged in |
| **LDAP** | Company's login system (like AD in Windows) |
| **Redis** | Super-fast temporary storage (like a sticky note) |
| **Idempotent** | Doing it twice = doing it once (safe to retry) |
| **Transaction** | All-or-nothing: either everything happens or nothing does |
| **Replica lag** | The delay before changes show up in the read copy |
| **Atomic** | Unbreakable — can't be interrupted halfway |
| **Rate limit** | "You can ask 100 times per minute, not 10,000" |
| **Blacklist** | A list of bad things to reject (e.g., revoked tokens) |
| **RBAC** | Role-Based Access Control — what you can do depends on your job title |

---

## Read the Source Material

Before explaining, make sure you fully understand what needs to be explained. This could be:
- **Code**: Read the relevant code files. Understand what the code does at a high level before translating.
- **A concept**: Break it into its core components.
- **An error message**: Understand the root cause, not just the surface text.
- **A technical document**: Extract the key points that matter.
- **Anything else**: Identify the essential "what" and "why."

## Craft the Explanation

Follow these principles, scaled to the audience:

### Structure
1. **Start with the "what"** — one sentence that captures the essence
2. **Use an analogy** — connect to something the audience already knows
3. **Fill in details** — add layers only as appropriate for the audience level
4. **End with the "so what"** — why does this matter to them specifically?

### Language Calibration

For **simple audiences** (young ages, non-technical roles, family):
- No jargon. Zero. If a technical term is essential, define it immediately.
- One idea per sentence.
- Concrete over abstract. "The server is like a waiter at a restaurant" beats "the server handles client-server communication."
- Use "you" and "your" — make it personal.

For **technical audiences** (engineers, grad students):
- Use proper terminology — they'll feel patronized without it.
- Focus on the *interesting* parts: trade-offs, edge cases, design decisions.
- Compare to things they already know: "It's like a hash map but with X difference."
- Be concise — respect their existing knowledge.

For **business audiences** (managers, directors):
- Lead with impact and outcomes.
- Quantify where possible.
- Skip implementation details unless asked.
- Frame in terms of decisions: "This means we should..."

End of ELI5 guide. Copy/paste explanations, or use the template to explain new concepts.
