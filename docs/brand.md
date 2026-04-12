# Brand & Identity

Living document for the **MemeForge** brand. This is the canonical reference for name, voice, vocabulary, visual direction, and namespace decisions. Update when any of these evolve.

> **Status:** early — name and core metaphor locked in, visual direction and namespace TBD.

---

## Name

**MemeForge**

### Rationale

- **`Meme`** — the content domain. Universal internet vocabulary, no translation needed
- **`Forge`** — the maker/fablab signal. Evokes creation, craft, hammer-and-anvil imagery, and the "take silly things seriously" spirit of maker culture
- Two syllables per word, hard consonants, easy to say aloud
- Previous name `FabDoYouMeme` was long, awkward, and borrowed from the unrelated "Do You Meme?" card game

### What the name must communicate

1. **This is a platform for making memes** (not browsing them)
2. **There is craft here** — it's not a random generator
3. **It's a game** — playful, social, not a professional tool

---

## Brand personality

| Axis                     | Position                                                                                                |
| ------------------------ | ------------------------------------------------------------------------------------------------------- |
| **Serious ↔ Playful**    | 70% playful, 30% serious — the tension _is_ the brand                                                   |
| **Warm ↔ Cold**          | Warm — this is a party game for a community, not a tool                                                 |
| **Minimal ↔ Expressive** | Expressive — memes are inherently maximal; the UI can breathe but the brand voice shouldn't be clinical |
| **Corporate ↔ Indie**    | Indie, self-hosted, community-first. No SaaS energy                                                     |

**Voice guidelines**

- Write like a clever maker friend, not a marketing team
- Lean into the craft metaphor without being precious about it
- Humour is allowed and encouraged, but never punches down
- No corporate jargon, no growth-hacking language, no "users" — call them **forgers**

---

## Vocabulary

The forge metaphor gives a full community lexicon for free. Use these consistently across UI copy, docs, and community spaces:

| Concept                       | Term                                         |
| ----------------------------- | -------------------------------------------- |
| A user                        | **Forger**                                   |
| A game session                | **A forging**                                |
| The studio / create area      | **The Smithy** (or **The Anvil**)            |
| Waiting room / matchmaking    | **Tempering**                                |
| Admin / moderator role        | **Master Smith**                             |
| Leaderboard                   | **Hall of Hammers** or **Forge Masters**     |
| Account / profile             | **Forger's Mark**                            |
| Server / self-hosted instance | **A Forge** (e.g. "which Forge are you on?") |

> **Note:** these are candidates, not locked-in. Pick the ones that feel natural and drop the ones that feel forced. The metaphor shouldn't become a straightjacket.

### Tagline candidates

- _Forge the dankest memes._
- _Hammer out a laugh._
- _Where memes get made._
- _The meme smithy._

Pick one later, once the visual direction is set.

---

## Visual direction (TBD)

Not yet designed. Capturing initial instincts so the future designer has a starting brief:

### Palette hypothesis

A **forge palette** — warm metals and ember glow, offset by cool steel:

- **Ember orange** `#FF6B2C` — primary accent, spark/heat energy
- **Molten red** `#C2410C` — secondary, for hover and emphasis
- **Steel grey** `#3F3F46` — background, surfaces
- **Anvil black** `#18181B` — text, deep backgrounds
- **Cream paper** `#FAF9F6` — light mode / contrast element
- **Spark yellow** `#FCD34D` — highlight, achievements

### Typography hypothesis

"Serious craft × playful content" pairing:

- **Brand / headings:** an industrial slab serif or condensed display face. Character: forged, weighty, confident
- **UI body:** a warm, rounded sans (e.g. Inter, Figtree, Nunito). Character: friendly, readable, no friction

### Iconography hypothesis

- Core symbol: **hammer + anvil**, or **crossed hammers**, or a **spark glyph**
- Motif: molten forms, glow effects, sparks on dark backgrounds
- Avoid: literal flame emojis, stock "meme" iconography (Doge, Stonks, etc.)

### Hero concept

A molten meme image on an anvil, being hammered, sparks flying. Craft meets silliness.

> **Decision point:** commission or DIY? A single afternoon with a designer friend + Figma could produce the core lockup, icon, and palette. See `Open questions` below.

---

## Namespace audit

As of **2026-04-11**.

### Domains

| TLD               | Status     | Notes                                                        |
| ----------------- | ---------- | ------------------------------------------------------------ |
| `memeforge.com`   | ❌ Taken   | Registered 2016, parking page at `/lander`. Squatter         |
| `memeforge.org`   | ❌ Taken   | Registered 2016, GoDaddy                                     |
| `memeforge.net`   | ❌ Taken   | Registered 2012, GoDaddy                                     |
| `memeforge.io`    | ⚠️ Taken   | Registered 2025-06-14, blank — **recent speculation, risky** |
| `memeforge.fun`   | ⚠️ Taken   | Registered 2025-12-18 — very recent                          |
| `memeforge.gg`    | ❓ Unknown | Needs manual check                                           |
| `memeforge.app`   | ❓ Unknown | Needs manual check                                           |
| `memeforge.games` | ❓ Unknown | Needs manual check                                           |
| `memeforge.dev`   | ❓ Unknown | Needs manual check                                           |
| `memeforge.party` | ❓ Unknown | Needs manual check                                           |
| `memeforge.club`  | ❓ Unknown | Needs manual check                                           |
| `memeforge.live`  | ❓ Unknown | Needs manual check                                           |

**Preferred TLD (pending verification):** `memeforge.gg` or `memeforge.games`. These signal "game" at the URL level.

**Avoid:** `.io` and `.fun` — both recently grabbed, suggests competing interest in the name.

**Fallback:** if all gaming TLDs are taken, consider `memeforge.party` / `memeforge.club`.

### GitHub

| Target                             | Status                                                     |
| ---------------------------------- | ---------------------------------------------------------- |
| `github.com/MorganKryze/MemeForge` | ✅ Available (your org, your call)                         |
| `github.com/memeforge` (org)       | ❌ Taken, but inactive — 0 repos. Safe to ignore           |
| Competing repos                    | 5 small projects, none with meaningful traction (≤6 stars) |

**Decision:** keep the repo under `MorganKryze/MemeForge`. No org namespace needed.

### Gaming platforms & packages

| Target          | Status                                                                                                                                                      |
| --------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Steam           | ✅ 0 results                                                                                                                                                |
| itch.io         | ⚠️ 1 existing project: "MemeForge" by `animus1` — a Reddit video generator. Different category. Same-named projects are allowed on itch.io but worth noting |
| npm `memeforge` | ❓ Unchecked — verify before publishing any packages                                                                                                        |

### Trademark

No trademark search has been conducted. Not legally required for an OSS self-hosted project under GPLv3, but if the project ever goes commercial:

- USPTO TESS search (US): https://www.uspto.gov/trademarks/search
- EUIPO search (EU): https://www.tmdn.org/tmview/
- Cost to file: ~$250–750 per class, ~6–12 months to grant

**Current recommendation:** no action. Revisit only if commercial fork or trademark dispute emerges.

---

## Brand architecture (future-proofing)

MemeForge is a **multi-game platform**, not a single game. The brand architecture should reflect this:

- **Master brand:** `MemeForge`
- **Game sub-brands:** `MemeForge: Captions`, `MemeForge: Match`, `MemeForge: Vote`, …
- Each game inherits the forge metaphor but may have its own iconography and secondary palette
- The platform identity is what users remember — game names are descriptors

This means: invest in the MemeForge identity first, treat individual game identities as lighter-weight variations.

---

## Open questions

These need user or designer input before locking in:

- [ ] **TLD decision** — run the manual check on `.gg` / `.games` / `.app` / `.party`, pick one, register
- [ ] **Logo direction** — DIY with Figma, or commission? Budget?
- [ ] **Typography** — specific typefaces TBD, need licenses (or pick Google Fonts)
- [ ] **Tagline** — pick one of the four candidates, or workshop more
- [ ] **Vocabulary lock-in** — which of the community terms ship in v1 copy, which stay as flavour text
- [ ] **Mascot?** — does MemeForge have a character? (e.g. a smith, an anvil with eyes) Decide deliberately — don't backfill one later
- [ ] **itch.io namespace** — decide if this ever ships on itch.io; if yes, plan to distinguish from the existing namesake

---

## Change log

- **2026-04-11** — initial draft. Name `MemeForge` selected (previously `FabDoYouMeme`). Namespace audit captured. Visual direction sketched as hypothesis only.
