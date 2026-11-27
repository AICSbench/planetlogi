import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import {
  DEFAULT_LOCALE,
  I18N_NAMESPACES,
  LOCALE_COOKIE_NAME,
  normalizeLocale,
  resources,
  type Locale,
} from "@planetlogi/i18n";

function readLocaleCookie(): Locale | null {
  if (typeof document === "undefined") return null;
  const match = document.cookie
    .split("; ")
    .find((row) => row.startsWith(`${LOCALE_COOKIE_NAME}=`));
  if (!match) return null;
  return normalizeLocale(match.split("=")[1]);
}

export function persistLocaleCookie(locale: Locale): void {
  if (typeof document === "undefined") return;
  document.cookie = `${LOCALE_COOKIE_NAME}=${locale}; path=/; max-age=${60 * 60 * 24 * 365}; SameSite=Lax`;
  document.documentElement.lang = locale;
}

export function detectInitialLocale(): Locale {
  if (typeof window !== "undefined") {
    const firstSeg = window.location.pathname.split("/").filter(Boolean)[0];
    if (firstSeg === "en" || firstSeg === "id") return firstSeg;
  }
  const cookieLocale = readLocaleCookie();
  if (cookieLocale) return cookieLocale;
  if (typeof navigator !== "undefined") {
    return normalizeLocale(navigator.language);
  }
  return DEFAULT_LOCALE;
}

if (!i18n.isInitialized) {
  const initialLocale = detectInitialLocale();
  void i18n.use(initReactI18next).init({
    resources,
    lng: initialLocale,
    fallbackLng: DEFAULT_LOCALE,
    defaultNS: "common",
    ns: [...I18N_NAMESPACES],
    interpolation: { escapeValue: false },
  });
  persistLocaleCookie(initialLocale);
}

export async function setAppLocale(locale: Locale): Promise<void> {
  persistLocaleCookie(locale);
  await i18n.changeLanguage(locale);
}

export default i18n;
