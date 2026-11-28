export const SUPPORTED_LOCALES = ["en", "id"] as const;
export type Locale = (typeof SUPPORTED_LOCALES)[number];
export const DEFAULT_LOCALE: Locale = "en";
export const LOCALE_COOKIE_NAME = "PLANETLOGI_LOCALE";

export const I18N_NAMESPACES = [
  "common",
  "auth",
  "dashboard",
  "daily-orbit",
  "training",
  "player",
  "academy",
  "astro-quiz",
  "portals",
] as const;
export type I18nNamespace = (typeof I18N_NAMESPACES)[number];

export function isSupportedLocale(value: string | undefined | null): value is Locale {
  return value === "en" || value === "id";
}

export function normalizeLocale(value: string | undefined | null): Locale {
  if (!value) return DEFAULT_LOCALE;
  const lower = value.toLowerCase().trim();
  if (lower === "id" || lower.startsWith("id-") || lower.startsWith("id_")) return "id";
  if (lower === "en" || lower.startsWith("en-") || lower.startsWith("en_")) return "en";
  return DEFAULT_LOCALE;
}
