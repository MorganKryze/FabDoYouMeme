// frontend/src/lib/content/tonePools.ts

/**
 * Tone-aware content pools, keyed by locale.
 *
 * Each slot (e.g. `home_greeting`) has five buckets, one per tone level.
 * Each bucket holds paired `{ h1, subline }` greetings.
 *
 * `{username}` is substituted at render time by the consumer component —
 * this file stores raw template strings only.
 *
 * Adding a new tone-aware surface:
 *   1. Add a new key to `SlotId`.
 *   2. Fill in five buckets of paired content for every supported locale.
 *   3. In the target component, call `pickForSlot('<slot>', tone.level)` on mount.
 */

import type { Locale } from '$lib/paraglide/runtime';

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

export const tonePools: Record<Locale, Record<SlotId, TonePool>> = {
  en: {
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
  },
  fr: {
    home_greeting: {
      // ─── Level 0 — Cozy ─────────────────────────────────────
      0: [
        { h1: 'Bon retour au Labo, {username}.',          subline: 'Ton matos est là où tu l’as laissé.' },
        { h1: 'Content de te revoir, {username}.',        subline: 'Le Labo était calme sans toi.' },
        { h1: 'Salut {username}. Ravi que tu passes.',    subline: 'Pose-toi. Quelque chose mijote.' },
        { h1: 'Ton coin est tiède, {username}.',          subline: 'Exactement comme tu l’as laissé.' },
        { h1: 'Installe-toi, {username}.',                subline: 'Ce soir, les memes sont faits main.' },
        { h1: 'Bienvenue chez toi, {username}.',          subline: 'Pose-toi à côté du moulin à captions.' },
        { h1: 'Pile à l’heure, {username}.',              subline: 'Le Labo se réveille tout juste.' },
        { h1: 'Enlève ta veste, {username}.',             subline: 'L’eau est chaude et les captions aussi.' },
      ],

      // ─── Level 1 — Warm ─────────────────────────────────────
      1: [
        { h1: 'Hey, {username}.',                         subline: 'Choisis un salon ou lance-en un nouveau.' },
        { h1: 'De retour, {username} ?',                  subline: 'On regarde ce qu’on fabrique ce soir ?' },
        { h1: 'Ravi de te voir, {username}.',             subline: 'Salons, jeux, la routine.' },
        { h1: '{username}, entre.',                       subline: 'On fabrique quoi aujourd’hui ?' },
        { h1: 'Te voilà, {username}.',                    subline: 'Le salon est ouvert. Vas-y.' },
        { h1: 'Regarde qui passe, {username}.',           subline: 'Prêt quand tu l’es.' },
        { h1: '{username}, prêt à y aller ?',             subline: 'Saute dedans avec un code ou héberge le tien.' },
        { h1: 'Bon retour, {username}.',                  subline: 'On transforme des images en décisions.' },
      ],

      // ─── Level 2 — Playful (default) ────────────────────────
      2: [
        { h1: 'Regarde qui est de retour, {username}.',   subline: 'Les memes ne vont pas se captionner tout seuls.' },
        { h1: '{username}, ton coin t’a manqué.',         subline: 'Peut-être. Difficile à dire avec un coin.' },
        { h1: 'Te voilà, {username}.',                    subline: 'On allait commencer sans toi. (Pour rire.)' },
        { h1: 'Hey {username}, prêt à commettre quelques délits d’humour ?', subline: 'Rien que des délits mineurs. On tient un Labo propre.' },
        { h1: '{username} ! Tu y es.',                    subline: 'Et à peine en retard, selon nos critères indulgents.' },
        { h1: 'Bon retour, {username}.',                  subline: 'Quelque part, une punchline t’attend spécifiquement.' },
        { h1: '{username} revient.',                      subline: 'Le Labo s’agite d’une malice renouvelée.' },
        { h1: 'Une minute pour saboter l’image d’un inconnu, {username} ?', subline: 'Avec amour. Toujours avec amour.' },
      ],

      // ─── Level 3 — Cheeky ───────────────────────────────────
      3: [
        { h1: 'Ah, {username}. On savait que tu ne tiendrais pas.', subline: 'Le vide en forme de meme dans ta vie est juste de cette taille.' },
        { h1: '{username}, encore.',                      subline: 'Ta productivité salue ta carrière de captionneur. De loin.' },
        { h1: 'Regarde qui sort du salon, {username}.',   subline: 'Assieds-toi. Tes captions vont te trahir aujourd’hui.' },
        { h1: '{username} ! Pile à temps pour la déception.', subline: 'De tes potes, spécifiquement. En toi.' },
        { h1: 'Hey {username}, prêt à perdre ?',          subline: 'Enfin, « jouer ». Jouer. C’est le mot.' },
        { h1: '{username}, bienvenue dans la Fosse.',     subline: 'Enfin, le Labo. Le Labo. C’est pareil, en gros.' },
        { h1: 'Tiens, c’est {username}.',                 subline: 'Une tête qu’une caption pourrait aimer.' },
        { h1: '{username}, espèce de légende absolue.',   subline: 'Légendairement présent, au moins. Ponctuellement médiocre.' },
      ],

      // ─── Level 4 — Chaos ────────────────────────────────────
      4: [
        { h1: '{username}. Enfin. On commençait à s’inquiéter.', subline: 'Bon, pas moi. Mais le salon, si.' },
        { h1: '{username}. La prophétie s’accomplit.',    subline: 'Tu étais destiné à être là à cet instant précis. Statistiquement.' },
        { h1: 'Ah. {username}. Comme prédit.',            subline: 'Par qui, on n’est pas en mesure de le dire.' },
        { h1: '{username}. Le coin murmure ton nom.',     subline: 'Ça lui arrive. On a l’habitude.' },
        { h1: '{username} ! Tu as survécu à l’entre-temps.', subline: 'Tout le monde n’y est pas arrivé. Mais n’en parlons pas maintenant.' },
        { h1: 'Ah, parfait, {username} est là.',          subline: 'Le conseil des memes attend depuis trois minutes. Max.' },
        { h1: 'Tu es venu. On savait que tu viendrais, {username}, on le savait.', subline: 'Ne demande pas comment. Les captions nous l’ont dit.' },
        { h1: 'Personne ne dit rien à {username}, mais la bouilloire est devenue sentiente.', subline: 'Elle a des opinions sur les captions de ce soir. La plupart sont légalement attaquables.' },
      ],
    },
  },
};
