export * from "./components/GalaxyButton";
export * from "./components/GalaxyCard";
export * from "./components/CurrencyPill";
export * from "./components/DifficultyMeter";
export * from "./components/ProficiencyPill";
export * from "./components/StarRating";

export interface PlanetTheme {
  grade: 1 | 2 | 3 | 4 | 5 | 6;
  code: string;
  name: { en: string; id: string };
  accentHex: string;
}

export const GRADE_PLANETS: readonly PlanetTheme[] = [
  { grade: 1, code: "terra_pebble", name: { en: "Terra Pebble", id: "Planet Kerikil" }, accentHex: "#10B981" },
  { grade: 2, code: "tropica", name: { en: "Planet Tropica", id: "Planet Tropika" }, accentHex: "#14B8A6" },
  { grade: 3, code: "aquaris", name: { en: "Planet Aquaris", id: "Planet Akuaris" }, accentHex: "#0EA5E9" },
  { grade: 4, code: "crystalia", name: { en: "Planet Crystalia", id: "Planet Kristalia" }, accentHex: "#A855F7" },
  { grade: 5, code: "lumina", name: { en: "Planet Lumina", id: "Planet Lumina" }, accentHex: "#22D3EE" },
  { grade: 6, code: "solaris", name: { en: "Planet Solaris", id: "Planet Solaris" }, accentHex: "#FBBF24" },
] as const;

export function getPlanetForGrade(grade: number): PlanetTheme {
  const clamped = Math.min(6, Math.max(1, Math.round(grade)));
  return GRADE_PLANETS[clamped - 1] ?? GRADE_PLANETS[0]!;
}
