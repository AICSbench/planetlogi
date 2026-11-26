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

/**
 * Parses student numeric input tolerantly across English (3.5) and Indonesian (3,5)
 * decimal conventions while rejecting ambiguous or non-numeric strings.
 */
export function parseStudentDecimal(raw: string): number | null {
  const cleaned = raw.trim().replace(/\s+/g, "");
  if (!cleaned) return null;

  const hasDot = cleaned.includes(".");
  const hasComma = cleaned.includes(",");
  if (hasDot && hasComma) {
    return null;
  }

  const normalized = hasComma ? cleaned.replace(",", ".") : cleaned;
  if (!/^-?\d+(\.\d+)?$/.test(normalized)) {
    return null;
  }
  const num = Number(normalized);
  return Number.isFinite(num) ? num : null;
}
