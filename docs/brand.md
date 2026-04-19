# Brand & Identity

Living document for the **FabDoYouMeme** brand. This is the canonical reference for name, voice, vocabulary, visual direction, and namespace decisions. Update when any of these evolve.

> **Status:** visual design language validated, logo/mascot/tagline TBD.

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

Pick one later, once the logo direction is set.

---

## Design Philosophy

**Chill, cozy, retro, modern.** A platform that feels warm and inviting — like a game night at a friend's place — without being childish. Visually appealing enough to leave running on a screen in the background. Minimal friction to gameplay fun.

The visual language is **neo-retro capsule design**: pill shapes, thick borders, physical depth on interactive elements, and pastel gradients with grain texture for a tactile "reality" feel — closer to print and film photography than flat digital UI.

**Inspiration reference:** [Wero merchant page](https://sowieso.wero-wallet.eu/nl-en/merchant)

### Core Principles

1. **Cozy but confident** — warm pastel tones, but with strong contrast and bold typography. Not soft/clinical.
2. **Alive, not still** — animated gradient backgrounds, time-of-day awareness, smooth transitions between game states.
3. **Concise over descriptive** — large bold text for gameplay. No paragraphs during play. Statement text, not description text.
4. **Tactile and physical** — buttons that press like real keys, cards that tilt like playing cards, real depth via shadows.
5. **Almost aligned** — layout is clean and intentional, with just enough imperfection to feel human. Movement comes from interaction (hover, click), not from static layout chaos.

---

## Color Palette

### Light/Dark Gradient System

The background is a **multi-color animated gradient** that swaps between a light and a dark palette based on clock band (auto) or explicit user preference. The gradient flows slowly (30s cycle, `background-size: 400% 400%`) and is overlaid with a **grain texture** (SVG `fractalNoise` filter at `opacity: 0.22`) for the tactile "reality" feel.

#### Light — Mint Garden

Fresh, bright, sun-through-the-window energy.

| Role           | Color                     | Hex                                        |
| -------------- | ------------------------- | ------------------------------------------ |
| Gradient stops | Sage, Seafoam, Peach, Sky | `#D4EDDA`, `#B5E2D0`, `#FDDCB5`, `#A8D8EA` |
| Text           | Near black                | `#1A1A1A`                                  |
| Mid text       | Dark grey                 | `#3A3A3A`                                  |
| Muted text     | Transparent dark          | `rgba(26,26,26,0.4)`                       |
| Surface        | White                     | `#FEFEFE`                                  |
| Border heavy   | Dark translucent          | `rgba(26,26,26,0.7)`                       |
| Accent         | Coral                     | `#E76F51`                                  |

#### Dark — Lavender Dusk

Deep, dreamy, creative-studio-at-night energy.

| Role           | Color                              | Hex                                        |
| -------------- | ---------------------------------- | ------------------------------------------ |
| Gradient stops | Deep plum, Indigo, Violet, Teal    | `#2A2040`, `#3D2B5A`, `#4A3470`, `#1B2838` |
| Text           | Light lavender                     | `#F2EBFF`                                  |
| Mid text       | Muted purple                       | `#C8BCE0`                                  |
| Surface        | Lifted dark purple                 | `#3A2D55`                                  |
| Border heavy   | Light translucent                  | `rgba(255,255,255,0.55)`                   |
| Accent         | Soft violet                        | `#C9A6FF`                                  |

### Gradient Animation

```css
background: linear-gradient(135deg, <stops>);
background-size: 400% 400%;
animation: gradientFlow 30s ease-in-out infinite;

@keyframes gradientFlow {
  0% {
    background-position: 0% 50%;
  }
  25% {
    background-position: 50% 100%;
  }
  50% {
    background-position: 100% 50%;
  }
  75% {
    background-position: 50% 0%;
  }
  100% {
    background-position: 0% 50%;
  }
}
```

### Auto Band Logic

```
07:00–19:00 → Mint Garden (light)
19:00–07:00 → Lavender Dusk (dark)
```

Transitions between bands must be **imperceptible** — CSS custom properties updated via JS interval every few minutes, with CSS `transition` on all color values. The user should never see colors "jump."

### Grain Texture

Applied globally as a fixed overlay above the gradient, below content:

```css
position: fixed;
inset: 0;
opacity: 0.22;
pointer-events: none;
background-image: url('data:image/svg+xml,...feTurbulence fractalNoise...');
background-size: 128px 128px;
```

Stays constant across both palettes for visual continuity.

---

## Typography

### Font: Quicksand

| Usage      | Weight | Size                         | Letter-spacing |
| ---------- | ------ | ---------------------------- | -------------- |
| Hero       | 700    | `clamp(4rem, 10vw, 8rem)`    | `-0.04em`      |
| Section    | 700    | `clamp(2rem, 4.5vw, 3.2rem)` | `-0.02em`      |
| Game stage | 700    | `clamp(2.5rem, 6vw, 4rem)`   | `-0.03em`      |
| Card title | 700    | `1.3rem`                     | normal         |
| Body       | 600    | `0.9rem–0.95rem`             | normal         |
| Label      | 700    | `0.65rem–0.72rem`, uppercase | `0.2em`        |

**Character:** Rounded, geometric, playful but not childish. Retro-modern.

### Text Philosophy

- **During gameplay:** Big, bold, concise. "Write your caption" not "It's time to write your caption for this round."
- **Stage transitions:** Statement text only. "Pick the best one." "BobZilla wins!" "+420 points."
- **Information hierarchy:** Size and weight, not color. Important = bigger. Secondary = smaller + muted alpha.

---

## Component Design Language

### Shared Traits

All interactive components share:

- **Pill/capsule shapes** — `border-radius: 999px` on buttons, inputs, toggles, toasts, player rows, nav
- **Thick dark borders** — `2.5px solid rgba(26,26,26,0.7)`
- **Physical depth** — offset `box-shadow` (not blur), e.g. `0 5px 0 rgba(0,0,0,0.22)`
- **White backgrounds** — `#FEFEFE` for components, `rgba(255,255,255,0.82)` for cards
- **Cards use `border-radius: 22px`** — slightly less round than pills, distinguishing containers from controls

### Navigation — Pill Nav

White pill, `2.5px` dark border. Active state uses **underline** (not fill). Shadow: `0 5px 0 rgba(0,0,0,0.12)`.

### Buttons

Three variants, all pill-shaped with `2.5px` dark border:

| Variant                | Background  | Text  | Shadow                     |
| ---------------------- | ----------- | ----- | -------------------------- |
| **Primary (outlined)** | White       | Dark  | `0 5px 0 rgba(0,0,0,0.22)` |
| **Dark (filled)**      | Dark        | White | `0 5px 0 rgba(0,0,0,0.35)` |
| **Ghost**              | Transparent | Dark  | `0 4px 0 rgba(0,0,0,0.1)`  |

**Press physics:** Hover lifts `translateY(-2px)`, shadow grows. Press pushes `translateY(3px)`, shadow shrinks to `1px`. Transition: `0.1s` for immediate tactile feel.

### Button Hover Effects

Five validated effects, each locked to a specific context (mapping confirmed 2026-04-12):

| Style              | Effect                                          | Locked to                                        |
| ------------------ | ----------------------------------------------- | ------------------------------------------------ |
| **Gradient Fill**  | Pastel gradient fades in + shimmers behind text | Primary CTAs (Create Room, Submit Caption)       |
| **Pulse Glow**     | Gradient halo breathes behind button            | Secondary CTAs (Join Room)                       |
| **Color Swap**     | White → dark inversion                          | All standard actions (admin, nav, forms, copy)   |
| **Bounce Expand**  | `scale(1.06)` with elastic spring               | Playful choices (Vote)                           |
| **Rainbow Border** | Border cycles through palette colors            | Reserved — **one button only** (Next Round)      |

Reuse existing implementations in `frontend/src/lib/actions/hoverEffect.ts`. Never inline a new variant in a component — extend the action instead.

### Cards — 3D Physical Card Behavior

Cards use **cursor-tracked 3D transforms** like real playing cards:

- **Hover:** tilt follows cursor (max ±6° Y, ±4° X), lift proportional to distance from center, shadow shifts with tilt direction
- **Click:** snap flat (`scale(0.98)`, `0.08s`) then spring back (`0.4s cubic-bezier(0.22, 1, 0.36, 1)`)
- **Surface:** `rgba(255,255,255,0.82)`, `2.5px` border, `border-radius: 22px`, shadow `0 5px 0 rgba(0,0,0,0.08)`

### Other Components

| Component       | Key traits                                                                                            |
| --------------- | ----------------------------------------------------------------------------------------------------- |
| **Player row**  | Pill-shaped, white, `2.5px` border. Hover: lift + slide right. `44px` avatar circles with dark border |
| **Meme card**   | 3D card physics. Image top, caption bottom. Voted state: dark border + dark offset shadow             |
| **Text input**  | Pill-shaped, `2.5px` border. Focus: border darkens, shadow strengthens                                |
| **Timer**       | Pill, white, dark border. `2.4rem` bold numerals. Pulsing coral dot (`#E8937F`)                       |
| **Pill toggle** | White container, dark border. Active: dark fill + white text                                          |
| **Toast**       | Pill, white, dark border. Emoji + bold text. `fit-content` width                                      |

---

## Animation & Motion

### Background

- **Gradient flow:** `30s` ease-in-out infinite. Viewport pans across 400% gradient.
- **Time shift:** CSS custom properties updated every few minutes. CSS `transition` ensures imperceptible shifts.

### Page Transitions

- **Scroll reveal:** `translateY(20px) → 0`, `opacity 0 → 1`, `0.6s ease`. IntersectionObserver, `threshold: 0.12`.
- **Staggered delays:** Siblings enter with `0.08s` incremental delays.

### Gameplay Stage Transitions

- **Between stages:** fade + scale. Hidden: `opacity: 0, scale(0.93), translateY(16px)`. Visible: `opacity: 1, scale(1), translateY(0)`.
- **Timing:** `0.9s cubic-bezier(0.22, 1, 0.36, 1)` — fast start, gentle landing. Liquid flow, never snapping.

### Interactive Elements

- **Buttons:** `0.1s` press, immediate feedback
- **Cards:** real-time cursor tracking (no transition). Press `0.08s`, release `0.4s` spring
- **Player rows:** `0.3s` slide on hover

### Philosophy

Motion is **ambient when idle** (gradient, time shifts) and **responsive when interactive** (card tilt, button press). The platform should feel different at different times of day, with shifts so gradual the user never notices them.

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

## Brand architecture

FabDoYouMeme is a **multi-game platform**, not a single game. The brand architecture should reflect this:

- **Master brand:** `FabDoYouMeme`
- **Game sub-brands:** `FabDoYouMeme: Captions`, `FabDoYouMeme: Match`, `FabDoYouMeme: Vote`, ...
- Each game inherits the maker/lab theme but may have its own secondary palette
- The platform identity is what players remember — game names are descriptors

Invest in the FabDoYouMeme identity first, treat individual game identities as lighter-weight variations.

---

## Open questions

These need user or designer input before locking in:

- [ ] **Domain audit** — run availability checks for `fabdoyoumeme.*` TLDs
- [ ] **Short alias** — does a redirect domain like `fdym.fr` or similar make sense alongside the full name?
- [ ] **Logo direction** — full lockup (icon + wordmark). Style TBD (abstract, symbolic, illustrative, or mascot-based)
- [ ] **Mascot?** — does FabDoYouMeme have a character? (e.g. a lab assistant, a meme gremlin) Decide deliberately — don't backfill one later
- [ ] **Tagline** — pick one of the five candidates, or workshop more
- [ ] **Vocabulary lock-in** — which of the community terms ship in v1 copy, which stay as flavour text
- [ ] **"Do You Meme?" proximity** — decide if the name similarity to the Fuckjerry card game needs any explicit disclaimer or differentiation in public-facing copy
- [x] **Dark mode toggle** — resolved 2026-04-12, simplified 2026-04-19. Users override the auto clock-based theme via a three-way segmented pill on the profile page (`auto` / `light` / `dark`). Preference is persisted in `localStorage` under `fdym:theme` and hydrated before `TimeBackground` mounts to prevent flash. Implemented in `frontend/src/lib/state/theme.svelte.ts` and `frontend/src/lib/components/ThemeToggle.svelte`.
- [x] **Mobile adaptations** — resolved 2026-04-12. `physCard` branches on `(hover: hover) and (pointer: fine)`. Touch devices get a tap-scale-press fallback (`translateY(1px) scale(0.97)` on `pointerdown`, spring release on `pointerup` using the same `cubic-bezier(0.22, 1, 0.36, 1)` curve). Cursor-tracked 3D tilt is disabled on non-hover devices. No DeviceOrientation, no haptics. Tap targets already ≥44px per the spec.
- [ ] **Sound design** — audio to accompany visual transitions
