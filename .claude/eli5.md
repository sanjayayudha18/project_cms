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

## Backend Concepts

### API (Application Programming Interface)

**Simple**: A waiter in a restaurant. You (the customer/frontend) tell the waiter (API) what you want. The waiter tells the kitchen (database) what to cook and brings back your food.

**Why it matters**: Different parts of the app talk to each other through APIs — like a standardized menu, so you know exactly what to order.

### Authentication (Login)

**Simple**: Showing your ID card to prove you are who you say you are.

**In CMS**: Employee logs in with LDAP (company username/password) or vendor logs in with local password. System says "yes, I know you" and gives you a badge (JWT token) that proves you're logged in for your next 10 visits without logging in again.

### Authorization (Permissions)

**Simple**: Even though you showed your ID at the door, you can't go into the executive bathroom — you only have access to certain rooms.

**In CMS**: An operator can see DSRs and create replenishment. An admin can also delete users. A vendor can only see their own cash boxes.

### Database

**Simple**: A giant filing cabinet where the app stores all data (users, money, receipts).

**In CMS**: Tables are like spreadsheets:
- `users` table = rows are people, columns are name/email/role
- `atm_dsr_uploads` table = rows are daily reports, one per ATM per day
- `audit_logs` table = rows are "who did what and when"

### Primary vs Replica Database

**Simple**: You have one main filing cabinet (primary) where everyone writes new stuff. You keep a photocopy (replica) in another room that people only read from when they want reports.

**Why**: If everyone reads from the photocopy, you don't slow down the main cabinet. Updates take a few seconds to copy over — fine for reports, bad for "I just approved this, show me the new balance."

### Migration (Database Change)

**Simple**: Moving to a new house. You pack your stuff, move, unpack, and make sure nothing broke.

**In code**: Adding a new column to a table, or changing a column's type. Has to be safe and reversible if something goes wrong.

### Transaction

**Simple**: A transaction in a store: you pay money, you get stuff, or neither happens. Halfway doesn't count.

**In code**: "Transfer $1000 from account A to account B" — either both happen or neither does. If it crashes halfway, you roll back.

---

## Frontend Concepts

### State

**Simple**: The current state of the app in your browser right now. Are you logged in? Is the dropdown open? What's in your search box?

**Why it matters**: If state gets messy, the app shows wrong data or old data.

### Component

**Simple**: A reusable LEGO brick. A button component is a button you can use anywhere — same code, different labels.

**In CMS**: The login form is a component. The vendor list is a component. They have their own state and logic.

### Hook (React)

**Simple**: A way to "hook into" extra features. `useState` = "remember this value for me." `useEffect` = "do this after the page loads."

### Two-Way Binding / Controlled Input

**Simple**: When you type in a text box, the value updates right away AND the form knows you typed something. Type and see = two-way.

**Bad version**: Type in a box, but the app doesn't know you typed until you click Submit.

---

## Money Concepts

### Numeric (NOT Float)

**Simple**: Store money as cents (integers), never decimals.

- ❌ WRONG: `$12.34` as a decimal number (floats lose precision)
- ✅ RIGHT: `1234` cents (whole number, exact)

**Why**: Floats round weirdly. `0.1 + 0.2` might give `0.30000000001` — nightmares for money.

### Minor Units

**Simple**: Storing money in the smallest unit (cents for USD, satoshis for Bitcoin, sen for IDR).

**In CMS**: IDR 100,000 = 100,000 sen. Store `100000`, always. Display divides by 100 if needed.

### Reconciliation

**Simple**: At the end of the day, you count what you have and check it matches your receipt book.

**In CMS**: "DSR says we had 5M IDR at 3pm. The physical count today showed 4.95M. Where's the 50K?"

### Escrow

**Simple**: A neutral third party holding money until both sides say "ok, release it."

**In CMS**: Corebanking (the bank) holds cash on behalf of CMS. CMS DSR says "I used 2M." Corebanking's escrow file says "yes, 2M left." Match = reconciled.

### Approval (Maker-Checker)

**Simple**: One person can't approve their own purchase. Person A creates invoice. Person B (who didn't create it) approves. Both need to agree.

**Why**: Stops one person from stealing $$$$ and covering it up alone.

---

## CMS-Specific Concepts

### DSR (Daily Status Report)

**Simple**: Every ATM calls home at midnight and says "I have 5M IDR in me right now." That's the DSR.

**What it does**: Tells us the cash position per machine per day. Basis for forecasting ("we need 10M more in 2 days").

### Replenishment

**Simple**: "That ATM is running low. Send a driver with 10M IDR to fill it up."

### CIT (Cash In Transit)

**Simple**: A security van that picks up full cash boxes from ATMs and brings them to the vault, or vice versa.

### Forecast (H+2)

**Simple**: "Looking ahead 2 days. ATM-123 will need 15M IDR. How much should vendor have in stock?"

### Vault

**Simple**: The vendor's safe where they keep all the cash for all the ATMs they service.

### Audit Log

**Simple**: A notebook where the app writes down everything that happened: "2026-08-27 09:15, John approved invoice #555, change: status pending→approved."

**Why**: If someone asks "who deleted that vendor?" you can check the log.

---

## Template: Explain a New Concept

Use this when you encounter a concept that needs explaining:

```
**Simple**: [One sentence, use an everyday analogy]

**Example**: [Real case from CMS or general software]

**Why it matters**: [One sentence on impact]

**Common confusion**: [What people usually get wrong]
```

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

## Example Explanations from CMS2

### "What happens when an operator uploads a DSR?"

1. **Frontend** gets the file from the operator's computer
2. **API** receives it, checks "is this a real operator?" (auth) and "can operators upload DSRs?" (authz)
3. **Service** reads the file, parses the numbers, and checks them (e.g., "is cash positive?")
4. **Database** stores the raw rows + a summary
5. **Audit log** writes "Operator X uploaded DSR for ATM-123 on 2026-08-27"
6. **Frontend** shows "✓ Upload successful!"

If step 2 fails: "Sorry, you don't have permission." If step 3 fails: "This file has bad data in row 5."

### "Why do we store money as integers?"

```
Float:   $1.23 might become $1.2300000001 (oops!)
Integer: 123 cents is always 123 cents
```

One person's $1M loss is someone's $1M gain, so 0.01¢ errors compound.

### "What's maker-checker for?"

Imagine one person can create AND approve an invoice to themselves for $1M. No one would catch it.

With maker-checker: Person A creates it. Person B (auditor/manager) looks at it, sees "$1M??? That's wrong" and rejects it. One person alone can't loot the till.

---

End of ELI5 guide. Copy/paste explanations, or use the template to explain new concepts.
