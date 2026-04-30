import * as m from '$lib/paraglide/messages';

interface GameTypeLike {
  slug: string;
  name: string;
  description: string | null;
}

// Game type name/description are stored in the backend registry as plain
// strings (English, the seeded defaults). The catalogue at
// `frontend/messages/{locale}.json` carries the localized copy, keyed by slug
// under `game_<slug_snake>_{name,description}`. If a slug lacks catalogue
// entries (e.g. a brand-new game type not yet translated) we fall back to the
// raw registry strings.
export function localizeGameType(gt: GameTypeLike): { name: string; description: string | null } {
  switch (gt.slug) {
    case 'meme-freestyle':
      return {
        name: m.game_meme_freestyle_name(),
        description: m.game_meme_freestyle_description()
      };
    case 'meme-showdown':
      return {
        name: m.game_meme_showdown_name(),
        description: m.game_meme_showdown_description()
      };
    case 'prompt-freestyle':
      return {
        name: m.game_prompt_freestyle_name(),
        description: m.game_prompt_freestyle_description()
      };
    case 'prompt-showdown':
      return {
        name: m.game_prompt_showdown_name(),
        description: m.game_prompt_showdown_description()
      };
    default:
      return { name: gt.name, description: gt.description };
  }
}
