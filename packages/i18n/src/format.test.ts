import { describe, expect, it } from "vitest";
import { normalizeLocale } from "./config";
import { formatCurrency, formatDecimal, formatNumber, parseStudentDecimal } from "./format";

describe("@planetlogi/i18n", () => {
  it("normalizes locale strings with en fallback", () => {
    expect(normalizeLocale("id-ID")).toBe("id");
    expect(normalizeLocale("en-GB")).toBe("en");
    expect(normalizeLocale("fr-FR")).toBe("en");
    expect(normalizeLocale(undefined)).toBe("en");
  });

  it("formats numbers and decimals per locale", () => {
    expect(formatNumber(1250, "en")).toBe("1,250");
    expect(formatNumber(1250, "id")).toBe("1.250");
    expect(formatDecimal(3.5, "en")).toBe("3.5");
    expect(formatDecimal(3.5, "id")).toBe("3,5");
  });

  it("formats currency per locale", () => {
    expect(formatCurrency(15000, "id")).toContain("15.000");
    expect(formatCurrency(12.5, "en")).toBe("$12.50");
  });

  it("accepts both dot and comma decimals from student keyboards", () => {
    expect(parseStudentDecimal("3.5")).toBe(3.5);
    expect(parseStudentDecimal("3,5")).toBe(3.5);
    expect(parseStudentDecimal(" 42 ")).toBe(42);
    expect(parseStudentDecimal("1,234.5")).toBeNull();
    expect(parseStudentDecimal("abc")).toBeNull();
  });
});
