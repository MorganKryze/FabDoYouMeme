# Brand & Identity

Living document for the **FabDoYouMeme** brand. This is the canonical reference for name, voice, vocabulary, visual direction, and namespace decisions. Update when any of these evolve.

> **Status:** early — name confirmed, visual direction and namespace TBD.

---

## Name

**FabDoYouMeme**

### Rationale

- **`Fab`** — the fablab / maker-space signal. Grounds the project in its origin: a physical community that builds things together
- **`DoYouMeme`** — part challenge ("do you meme?"), part invitation ("do you, meme"). It frames the game as a dare, not a service
- The name is long, yes — but it's memorable as a phrase. It sounds like something someone would say out loud at a table
- It carries the project's DNA: born in a fablab, meant for in-person groups, unapologetically nerdy

### What the name must communicate

1. **This comes from a maker community** — it's not a corporate product
2. **It's a challenge** — "do you meme?" is a provocation, not a description
3. **It's a game** — the phrasing is playful, social, meant to be spoken aloud

### Trade-offs acknowledged

- The name is long (14 characters, 4 syllables spoken as "fab-do-you-meme")
- It references the existing "Do You Meme?" card game — different product, but name proximity exists
- Abbreviation to **FDYM** works for technical contexts (repo slugs, CLI) but not for brand communication

These are accepted costs. The name's personality outweighs its length.

---

## Brand personality

| Axis                      | Position                                                                                                |
| ------------------------- | ------------------------------------------------------------------------------------------------------- |
| **Serious <> Playful**    | 70% playful, 30% serious — the tension _is_ the brand                                                   |
| **Warm <> Cold**          | Warm — this is a party game for a community, not a tool                                                 |
| **Minimal <> Expressive** | Expressive — memes are inherently maximal; the UI can breathe but the brand voice shouldn't be clinical |
| **Corporate <> Indie**    | Indie, self-hosted, community-first. No SaaS energy                                                     |

**Voice guidelines**

- Write like a clever maker friend, not a marketing team
- Lean into the fablab / DIY origin without being gatekeepy about it
- Humour is allowed and encouraged, but never punches down
- No corporate jargon, no growth-hacking language, no "users" — call them **players** or **makers**

---

## Vocabulary

The fablab origin gives a natural vocabulary rooted in maker culture. Use these consistently across UI copy, docs, and community spaces:

| Concept                       | Term                                     |
| ----------------------------- | ---------------------------------------- |
| A user                        | **Maker** or **Player**                  |
| A game session                | **A session** or **A round**             |
| The studio / create area      | **The Lab** or **The Bench**             |
| Waiting room / matchmaking    | **The Lobby**                            |
| Admin / moderator role        | **Lab Master**                           |
| Leaderboard                   | **Hall of Fame**                         |
| Account / profile             | **Maker Card**                           |
| Server / self-hosted instance | **A Lab** (e.g. "which Lab are you on?") |

> **Note:** these are candidates, not locked-in. Pick the ones that feel natural and drop the ones that feel forced. The fablab metaphor should enhance, not constrain.

### Tagline candidates

- _Fab, do you meme?_
- _Made in the lab. Played at the table._
- _The meme lab._
- _Craft your punchline._
- _Where makers meme._

Pick one later, once the visual direction is set.

---

## Visual direction (TBD)

Not yet designed. Capturing initial instincts so the future designer has a starting brief:

### Palette hypothesis

A **lab palette** — clean workshop surfaces with pops of maker energy:

- **Maker blue** `#2563EB` — primary accent, trust + craft energy
- **Neon green** `#22C55E` — secondary, for success states and highlights
- **Workshop grey** `#3F3F46` — background, surfaces
- **Chalkboard** `#18181B` — text, deep backgrounds
- **Paper white** `#FAF9F6` — light mode / contrast element
- **Caution yellow** `#FACC15` — highlight, achievements, warnings

### Typography hypothesis

"Maker craft x playful content" pairing:

- **Brand / headings:** a condensed grotesque or industrial sans-serif. Character: workshop signage, confident, slightly rough
- **UI body:** a warm, rounded sans (e.g. Inter, Figtree, Nunito). Character: friendly, readable, no friction

### Iconography hypothesis

- Core symbol: something that combines **making** and **memes** — a beaker with a laughing face, a 3D printer extruding a meme, crossed tools over a speech bubble
- Motif: blueprint lines, workshop textures, maker-space energy
- Avoid: literal meme iconography (Doge, Stonks, etc.), overly techy/startup aesthetics

### Hero concept

A workshop bench with meme materials being assembled — scissors, glue, printed templates, absurd imagery being cut and pasted. Physical craft meets digital silliness.

> **Decision point:** commission or DIY? A single afternoon with a designer friend + Figma could produce the core lockup, icon, and palette. See `Open questions` below.

---

## Namespace audit

As of **2026-04-12**.

### Domains

Not yet audited for `fabdoyoumeme.*`.

**To investigate:**

| TLD                  | Why check                              |
| -------------------- | -------------------------------------- |
| `fabdoyoumeme.fr`    | French ccTLD, matches community origin |
| `fabdoyoumeme.games` | Literal "this is a game" signal        |
| `fabdoyoumeme.party` | Genre-perfect for party games          |
| `fabdoyoumeme.eu`    | European scope                         |
| `fabdoyoumeme.com`   | Long shot — check availability         |

**Note:** the name is long as a domain. Consider whether a **short alias** (e.g. `fdym.fr`) makes sense as a redirect alongside the full canonical domain.

**Current decision:** subdomain on an existing domain for now — no new TLD purchased yet.

### GitHub

| Target                                | Status                           |
| ------------------------------------- | -------------------------------- |
| `github.com/MorganKryze/FabDoYouMeme` | ✅ Current repo — already in use |

No namespace conflict. The repo name matches the brand.

### Gaming platforms & packages

| Target  | Status                          |
| ------- | ------------------------------- |
| Steam   | ❓ Unchecked for `FabDoYouMeme` |
| itch.io | ❓ Unchecked                    |
| npm     | ❓ Unchecked                    |

### Trademark

No trademark search has been conducted. The name "Do You Meme?" is trademarked by Fuckjerry Inc. for their card game. FabDoYouMeme is a distinct name (prefix + single compound), different product category (digital self-hosted platform vs. physical card game), and non-commercial (GPLv3). Risk is low but worth noting.

**Current recommendation:** no action. Revisit only if commercial fork or trademark dispute emerges.

---

## Brand architecture (future-proofing)

FabDoYouMeme is a **multi-game platform**, not a single game. The brand architecture should reflect this:

- **Master brand:** `FabDoYouMeme`
- **Game sub-brands:** `FabDoYouMeme: Captions`, `FabDoYouMeme: Match`, `FabDoYouMeme: Vote`, ...
- Each game inherits the maker/lab theme but may have its own secondary palette
- The platform identity is what players remember — game names are descriptors

This means: invest in the FabDoYouMeme identity first, treat individual game identities as lighter-weight variations.

---

## Open questions

These need user or designer input before locking in:

- [ ] **Domain audit** — run availability checks for `fabdoyoumeme.*` TLDs
- [ ] **Short alias** — does a redirect domain like `fdym.fr` or similar make sense alongside the full name?
- [ ] **Logo direction** — DIY with Figma, or commission? Budget?
- [ ] **Typography** — specific typefaces TBD, need licenses (or pick Google Fonts)
- [ ] **Tagline** — pick one of the five candidates, or workshop more
- [ ] **Vocabulary lock-in** — which of the community terms ship in v1 copy, which stay as flavour text
- [ ] **Mascot?** — does FabDoYouMeme have a character? (e.g. a lab assistant, a meme gremlin) Decide deliberately — don't backfill one later
- [ ] **"Do You Meme?" proximity** — decide if the name similarity to the Fuckjerry card game needs any explicit disclaimer or differentiation in public-facing copy
- [ ] **Palette direction** — clean lab (blues/greens) or warm workshop (embers/oranges)? These set very different moods
