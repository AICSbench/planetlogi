import { useTranslation } from "react-i18next";
import type { Locale } from "@planetlogi/i18n";
import { CurrencyPill, GalaxyCard } from "@planetlogi/ui";
import { useAuthStore } from "../../../store/authStore";

const RANK_BANDS = [
  { min: 1, max: 9, en: "Star Cadet", id: "Kadet Bintang" },
  { min: 10, max: 19, en: "Orbit Scout", id: "Pengintai Orbit" },
  { min: 20, max: 29, en: "Comet Pilot", id: "Pilot Komet" },
  { min: 30, max: 39, en: "Meteor Navigator", id: "Navigator Meteor" },
  { min: 40, max: 49, en: "Planetary Ranger", id: "Penjelajah Planet" },
  { min: 50, max: 59, en: "Solar Captain", id: "Kapten Surya" },
  { min: 60, max: 69, en: "Starlight Ace", id: "Jagoan Cahaya Bintang" },
  { min: 70, max: 79, en: "Galaxy Voyager", id: "Pelayar Galaksi" },
  { min: 80, max: 89, en: "Supernova Guardian", id: "Penjaga Supernova" },
  { min: 90, max: 99, en: "Nebula Admiral", id: "Laksamana Nebula" },
  { min: 100, max: 100, en: "PlanetLogi Grand Commander", id: "Panglima Besar PlanetLogi" },
] as const;

function xpForLevel(level: number): number {
  if (level <= 1) return 4;
  if (level >= 100) return 0;
  const d = level - 1;
  return Math.round(4 + 6 * d + 1.8 * Math.pow(d, 1.55));
}

export function ExpLevelPage({ locale }: { locale: Locale }) {
  const { t } = useTranslation(["training", "common"]);
  const user = useAuthStore((s) => s.user);
  const currentLevel = user?.pilotLevel ?? 2;
  const nextReq = xpForLevel(currentLevel);
  const currentBand =
    RANK_BANDS.find((b) => currentLevel >= b.min && currentLevel <= b.max) ?? RANK_BANDS[0];

  return (
    <div className="mx-auto max-w-4xl space-y-6">
      <GalaxyCard className="space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 className="text-2xl font-extrabold text-white">{t("training:expLevel.title")}</h1>
            <p className="text-sm text-slate-300">{t("training:expLevel.subtitle")}</p>
          </div>
          <CurrencyPill kind="XP" value={t("common:common.level", { level: currentLevel })} />
        </div>

        <div className="rounded-xl bg-[#1E2A55] p-4 space-y-2">
          <div className="flex items-center justify-between text-sm">
            <span className="text-slate-300">{t("training:expLevel.currentRank")}</span>
            <strong className="text-[#FBBF24]">{currentBand[locale]}</strong>
          </div>
          <div className="h-3 w-full overflow-hidden rounded-full bg-[#0B1026]">
            <div className="h-full w-2/5 rounded-full bg-gradient-to-r from-[#22D3EE] to-[#6366F1]" />
          </div>
          <p className="text-xs text-cyan-200">
            {t("training:expLevel.nextLevel", { xp: nextReq, next: Math.min(100, currentLevel + 1) })}
          </p>
        </div>
      </GalaxyCard>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {RANK_BANDS.map((band) => {
          const unlocked = currentLevel >= band.min;
          return (
            <GalaxyCard
              key={band.min}
              className={`space-y-1 ${unlocked ? "border-[#22D3EE]/50" : "opacity-65"}`}
            >
              <p className="text-xs font-bold uppercase text-cyan-300">
                Lvl {band.min}
                {band.max > band.min ? `–${band.max}` : ""}
              </p>
              <p className="text-base font-extrabold text-white">{band[locale]}</p>
            </GalaxyCard>
          );
        })}
      </div>
    </div>
  );
}
