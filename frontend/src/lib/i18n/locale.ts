import { env as publicEnv } from '$env/dynamic/public';
import { locales, isLocale as paraglideIsLocale } from '$lib/paraglide/runtime';

export const SUPPORTED_LOCALES = locales;
export type Locale = (typeof locales)[number];

export function isLocale(v: unknown): v is Locale {
  return paraglideIsLocale(v);
}

export function defaultLocale(): Locale {
  const raw = publicEnv.PUBLIC_DEFAULT_LOCALE;
  return isLocale(raw) ? raw : 'en';
}
