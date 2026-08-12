## Agent skills

### Grill me

- Trigger: say **grill me** / use skill `grill-me` → runs `grilling`.
- Local copies: `.agents/skills/grill-me`, `.agents/skills/grilling`.

### Templates from gift-bot

Do **not** reinvent agent docs, Docker/bot scaffolding, Mini App patterns, or issue-tracker layout when a template already exists in gift-bot.

**Canonical template source (sibling checkout):**

- Path: `../gift-bot` (absolute: `/Users/reqiewu/Projects/Pets/gift-bot`)
- Index: `../gift-bot/AGENTS.md`

**Copy or adapt from there when needed:**

| Need | Look in gift-bot |
| ---- | ---------------- |
| Issue tracker (`.scratch/`) | `docs/agents/issue-tracker.md` |
| Triage labels | `docs/agents/triage-labels.md` |
| Domain / CONTEXT + ADR | `docs/agents/domain.md`, `CONTEXT.md`, `docs/adr/` |
| External catalog + Mini App | `docs/adr/0001-…`, `.cursor/rules/external-catalog-and-mini-app.mdc` |
| Other engineering skills | `.agents/skills/*` (tdd, domain-modeling, code-review, …) |
| Bot / Compose / Mini App HTTP | `cmd/gift-bot`, `docker-compose.yml`, `internal/miniapp/`, `web/` |

When borrowing: **read** the gift-bot file first, then copy only what order-bot needs into this repo (or link in docs). Prefer adapting over dumping the whole gift-bot tree.
