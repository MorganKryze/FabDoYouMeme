// frontend/src/lib/content/tonePools.ts

/**
 * Tone-aware content pools.
 *
 * Each slot (e.g. `home_greeting`) has five buckets, one per tone level.
 * Each bucket holds paired `{ h1, subline }` greetings.
 *
 * `{username}` is substituted at render time by the consumer component —
 * this file stores raw template strings only.
 *
 * Adding a new tone-aware surface:
 *   1. Add a new key to `SlotId`.
 *   2. Fill in five buckets of paired content.
 *   3. In the target component, call `pickForSlot('<slot>', tone.level)` on mount.
 */

export type ToneLevel = 0 | 1 | 2 | 3 | 4;

export const TONE_LABELS = {
  0: 'Cozy',
  1: 'Warm',
  2: 'Playful',
  3: 'Cheeky',
  4: 'Chaos',
} as const satisfies Record<ToneLevel, string>;

export const DEFAULT_TONE: ToneLevel = 2;

export type TonePair = { h1: string; subline: string };
export type TonePool = Record<ToneLevel, TonePair[]>;

export type SlotId = 'home_greeting';

export const tonePools: Record<SlotId, TonePool> = {
  home_greeting: {
    // ─── Level 0 — Cozy ─────────────────────────────────────
    0: [
      { h1: 'Welcome back to the bench, {username}.', subline: 'Your tools are where you left them.' },
      { h1: 'Good to see you again, {username}.',     subline: 'The lab’s been quiet without you.' },
      { h1: 'Hey, {username}. Glad you’re here.', subline: 'Pull up a chair. Something’s brewing.' },
      { h1: 'The bench is warm, {username}.',         subline: 'Just like you left it.' },
      { h1: 'Settle in, {username}.',                 subline: 'Tonight’s memes are handcrafted.' },
      { h1: 'Welcome home, {username}.',              subline: 'Grab a seat by the caption mill.' },
      { h1: 'Right on time, {username}.',             subline: 'The lab’s just waking up.' },
      { h1: 'Take your coat off, {username}.',        subline: 'We’ve got the kettle on and captions warming.' },
    ],

    // ─── Level 1 — Warm ─────────────────────────────────────
    1: [
      { h1: 'Hey there, {username}.',                 subline: 'Pick a room or spin up a new one.' },
      { h1: 'Back for more, {username}?',             subline: "Let’s see what we’re building tonight." },
      { h1: 'Good to have you, {username}.',          subline: 'Rooms, games, the usual.' },
      { h1: '{username}, welcome in.',                subline: 'What are we making today?' },
      { h1: 'There you are, {username}.',             subline: "The lobby’s open. Go ahead." },
      { h1: "Look who’s around, {username}.",    subline: 'Ready when you are.' },
      { h1: '{username}, ready to go?',               subline: 'Jump in with a code or host your own.' },
      { h1: 'Welcome back, {username}.',              subline: "Let’s turn some images into decisions." },
    ],

    // ─── Level 2 — Playful (default) ────────────────────────
    2: [
      { h1: "Look who’s back, {username}.",      subline: "The memes aren’t going to caption themselves." },
      { h1: '{username}, the bench missed you.',      subline: 'Possibly. Hard to say with benches.' },
      { h1: 'There you are, {username}.',             subline: 'We were about to start without you. (Kidding.)' },
      { h1: 'Hey {username}, ready to commit some comedy crimes?', subline: 'Misdemeanors only. We run a clean lab.' },
      { h1: '{username}! You made it.',               subline: 'And only slightly late, by our generous standards.' },
      { h1: 'Welcome back, {username}.',              subline: 'Somewhere, a punchline is waiting for you specifically.' },
      { h1: '{username} returns.',                    subline: 'The lab buzzes with renewed mischief.' },
      { h1: 'Got a minute to ruin a stranger’s image, {username}?', subline: 'In a loving way. Always in a loving way.' },
    ],

    // ─── Level 3 — Cheeky ───────────────────────────────────
    3: [
      { h1: "Oh, {username}. We knew you couldn’t stay away.", subline: 'The meme-shaped hole in your life is just that big.' },
      { h1: '{username}, back again.',                subline: 'Your productivity salutes your captioning career. From afar.' },
      { h1: 'Look who crawled out of the lobby, {username}.', subline: 'Sit down. Your captions are going to betray you today.' },
      { h1: '{username}! Just in time for disappointment.', subline: 'Of your friends, specifically. In you.' },
      { h1: 'Hey {username}, ready to lose?',         subline: 'I mean "play." Play. That’s the word.' },
      { h1: '{username}, welcome back to the Pit.',   subline: 'I mean, the Lab. The Lab. The same thing, mostly.' },
      { h1: "Look, it’s {username}.",            subline: 'A face only a caption could love.' },
      { h1: '{username}, you absolute legend.',       subline: "Legendarily here, at least. Punctually mediocre." },
    ],

    // ─── Level 4 — Chaos ────────────────────────────────────
    4: [
      { h1: '{username}. Finally. We were starting to worry.', subline: "Well, I wasn’t. But the lobby was." },
      { h1: '{username}. The prophecy is fulfilled.', subline: 'You were always going to be here at this exact moment. Statistically.' },
      { h1: 'Ah. {username}. As foretold.',           subline: "By what, we’re not at liberty to say." },
      { h1: '{username}. The bench has been whispering your name.', subline: "It does that. We’re used to it." },
      { h1: '{username}! You survived the in-between time.', subline: "Not everyone did. But let’s not get into that right now." },
      { h1: 'Oh good, {username} is here.',           subline: 'The council of memes has been waiting three minutes. Tops.' },
      { h1: 'You came. We knew you would. {username}, we knew.', subline: 'Please don’t ask how. The captions told us.' },
      { h1: 'Nobody tell {username}, but the kettle is sentient now.', subline: 'It has opinions about tonight’s captions. Most are legally actionable.' },
    ],
  },
};
