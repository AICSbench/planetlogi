import type { Locale } from "./config";

const INTL_LOCALE_MAP: Record<Locale, string> = {
  en: "en-US",
  id: "id-ID",
};

export function formatNumber(value: number, locale: Locale): string {
  return new Intl.NumberFormat(INTL_LOCALE_MAP[locale]).format(value);
}

export function formatDecimal(value: number, locale: Locale, maxFractionDigits = 2): string {
  return new Intl.NumberFormat(INTL_LOCALE_MAP[locale], {
    maximumFractionDigits: maxFractionDigits,
  }).format(value);
}

export function formatCurrency(amount: number, locale: Locale): string {
  if (locale === "id") {
    return new Intl.NumberFormat("id-ID", {
      style: "currency",
      currency: "IDR",
      maximumFractionDigits: 0,
    }).format(amount);
  }
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 2,
  }).format(amount);
}

export interface StudentNumberParseOptions {
  mode?: "decimal" | "integer";
  locale?: Locale;
}

/**
 * Parses student numeric input tolerantly across English (3.5) and Indonesian (3,5)
 * conventions while distinguishing or rejecting ambiguous thousand-grouped integers
 * such as "1,250" (en) and "1.250" (id).
 */
export function parseStudentDecimal(
  raw: string,
  options?: StudentNumberParseOptions
): number | null {
  const cleaned = raw.trim();
  if (!cleaned) return null;

  const mode = options?.mode ?? "decimal";
  const locale = options?.locale;

  if (mode === "integer") {
    let intStr = cleaned;
    if (locale === "en" && /^-?[1-9]\d{0,2}(,\d{3})+$/.test(cleaned)) {
      intStr = cleaned.replace(/,/g, "");
    } else if (locale === "id" && /^-?[1-9]\d{0,2}(\.\d{3})+$/.test(cleaned)) {
      intStr = cleaned.replace(/\./g, "");
    }
    if (!/^-?\d+$/.test(intStr)) {
      return null;
    }
    const parsedInt = Number(intStr);
    return Number.isSafeInteger(parsedInt) ? parsedInt : null;
  }

  const hasDot = cleaned.includes(".");
  const hasComma = cleaned.includes(",");
  if (hasDot && hasComma) {
    return null;
  }

  if (locale === "en" && hasComma && /^-?[1-9]\d{0,2}(,\d{3})+$/.test(cleaned)) {
    const grouped = Number(cleaned.replace(/,/g, ""));
    return Number.isFinite(grouped) ? grouped : null;
  }
  if (locale === "id" && hasDot && /^-?[1-9]\d{0,2}(\.\d{3})+$/.test(cleaned)) {
    const grouped = Number(cleaned.replace(/\./g, ""));
    return Number.isFinite(grouped) ? grouped : null;
  }

  // Without explicit locale context, reject ambiguous thousand-grouped forms like "1,250" or "1.250"
  if (!locale && /^-?[1-9]\d{0,2}[.,]\d{3}$/.test(cleaned)) {
    return null;
  }

  const normalized = hasComma ? cleaned.replace(",", ".") : cleaned;
  if (!/^-?\d+(\.\d+)?$/.test(normalized)) {
    return null;
  }
  const num = Number(normalized);
  return Number.isFinite(num) ? num : null;
}
