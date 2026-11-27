import { useState } from "react";
import { useTranslation } from "react-i18next";
import { formatNumber, type Locale } from "@planetlogi/i18n";
import {
  CurrencyPill,
  DifficultyMeter,
  GalaxyCard,
  ProficiencyPill,
  StarRating,
  getPlanetForGrade,
} from "@planetlogi/ui";
import { useAuthStore } from "../../../store/authStore";

type PillarKey = "challenges" | "training" | "academy" | "fun";

export function DashboardPreviewPage({ locale }: { locale: Locale }) {
  const { t } = useTranslation(["dashboard", "common"]);
  const user = useAuthStore((s) => s.user);
  const [pillar, setPillar] = useState<PillarKey>("challenges");

  const grade = user?.grade ?? 1;
  const planet = getPlanetForGrade(grade);
  const name = user?.displayName ?? "Raka";

  return (
    <div className="grid grid-cols-1 gap-6 lg:grid-cols-12">
      {/* Left Student Profile Card */}
      <aside className="lg:col-span-3 space-y-4">
        <GalaxyCard className="space-y-4">
          <div className="flex items-center gap-3.5">
            <div
              className="flex h-14 w-14 items-center justify-center rounded-2xl text-2xl font-extrabold text-white shadow-lg"
              style={{ backgroundColor: planet.accentHex }}
            >
              {name.slice(0, 1).toUpperCase()}
            </div>
            <div>
              <h1 className="text-lg font-extrabold text-white">
                {t("dashboard:welcome", { name })}
              </h1>
              <p className="text-xs font-semibold text-cyan-300">
                {t("common:common.grade", { grade })} •{" "}
                {t("common:common.level", { level: user?.pilotLevel ?? 2 })}
              </p>
            </div>
          </div>
          <p className="rounded-xl bg-[#1E2A55]/80 px-3 py-2 text-xs font-medium text-slate-200">
            🪐 {t("dashboard:planetLabel", { planet: planet.name[locale] })}
          </p>
          <div className="flex flex-wrap gap-2">
            <ProficiencyPill tier="MASTERY" label={t("common:proficiency.mastery")} pct={88} />
            <StarRating stars={3} label={t("common:a11y.stars", { stars: 3 })} />
            <DifficultyMeter level={3} label={t("common:a11y.difficulty", { level: 3 })} />
          </div>
        </GalaxyCard>
      </aside>

      {/* Center 4-Pillar Hub */}
      <section className="lg:col-span-6 space-y-4">
        <div
          role="tablist"
          aria-label={t("dashboard:pillarsLabel")}
          className="grid grid-cols-2 gap-2 sm:grid-cols-4"
        >
          {(["challenges", "training", "academy", "fun"] as const).map((key) => (
            <button
              key={key}
              role="tab"
              type="button"
              aria-selected={pillar === key}
              onClick={() => setPillar(key)}
              className={`min-h-12 rounded-2xl px-3 text-sm font-bold transition-colors cursor-pointer ${
                pillar === key
                  ? "bg-[#6366F1] text-white shadow-lg shadow-indigo-500/30"
                  : "border border-indigo-400/20 bg-[#131B3A] text-slate-200 hover:bg-[#1E2A55]"
              }`}
            >
              {t(`dashboard:pillars.${key}`)}
            </button>
          ))}
        </div>

        {pillar === "challenges" ? (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <GalaxyCard>
              <h2 className="text-lg font-bold text-[#FBBF24]">
                🪐 {t("dashboard:cards.dailyOrbitTitle")}
              </h2>
              <p className="mt-1.5 text-sm text-slate-300">{t("dashboard:cards.dailyOrbitDesc")}</p>
            </GalaxyCard>
            <GalaxyCard>
              <h2 className="text-lg font-bold text-[#22D3EE]">
                ⚡ {t("dashboard:cards.superNovaTitle")}
              </h2>
              <p className="mt-1.5 text-sm text-slate-300">{t("dashboard:cards.superNovaDesc")}</p>
            </GalaxyCard>
            <GalaxyCard className="sm:col-span-2">
              <h2 className="text-lg font-bold text-indigo-300">
                ⚔️ {t("dashboard:cards.starDuelsTitle")}
              </h2>
              <p className="mt-1.5 text-sm text-slate-300">{t("dashboard:cards.starDuelsDesc")}</p>
            </GalaxyCard>
          </div>
        ) : null}

        {pillar === "training" ? (
          <GalaxyCard>
            <h2 className="text-lg font-bold text-emerald-300">
              🌌 {t("dashboard:cards.galaxyTrainingTitle")}
            </h2>
            <p className="mt-1.5 text-sm text-slate-300">
              {t("dashboard:cards.galaxyTrainingDesc")}
            </p>
          </GalaxyCard>
        ) : null}

        {pillar === "academy" ? (
          <GalaxyCard>
            <h2 className="text-lg font-bold text-cyan-300">
              🚀 {t("dashboard:cards.spaceAcademyTitle")}
            </h2>
            <p className="mt-1.5 text-sm text-slate-300">
              {t("dashboard:cards.spaceAcademyDesc")}
            </p>
          </GalaxyCard>
        ) : null}

        {pillar === "fun" ? (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <GalaxyCard>
              <h2 className="text-lg font-bold text-amber-300">
                📖 {t("dashboard:cards.planetStoriesTitle")}
              </h2>
              <p className="mt-1.5 text-sm text-slate-300">
                {t("dashboard:cards.planetStoriesDesc")}
              </p>
            </GalaxyCard>
            <GalaxyCard>
              <h2 className="text-lg font-bold text-pink-300">
                🧠 {t("dashboard:cards.brainGalaxyTitle")}
              </h2>
              <p className="mt-1.5 text-sm text-slate-300">
                {t("dashboard:cards.brainGalaxyDesc")}
              </p>
            </GalaxyCard>
          </div>
        ) : null}

        <p className="rounded-xl border border-indigo-400/20 bg-[#131B3A]/60 px-4 py-2.5 text-xs text-slate-300">
          ✨ {t("dashboard:status.foundationReady")}
        </p>
      </section>

      {/* Right Currency Summary Panel */}
      <aside className="lg:col-span-3">
        <GalaxyCard className="space-y-3">
          <h2 className="text-sm font-bold uppercase tracking-wider text-slate-300">
            {t("dashboard:wallet.totalSp")} & {t("dashboard:wallet.astroCoins")}
          </h2>
          <div className="flex flex-wrap gap-2">
            <CurrencyPill
              kind="SP"
              value={formatNumber(user ? (user.totalSp ?? 0) : 12, locale)}
              label={t("common:currencies.sp")}
            />
            <CurrencyPill
              kind="AC"
              value={formatNumber(user ? (user.astroCoins ?? 0) : 25, locale)}
              label={t("common:currencies.ac")}
            />
          </div>
          <div className="rounded-xl bg-[#1E2A55]/70 p-3 text-xs text-slate-300">
            ❄️ {t("dashboard:wallet.cryoShields")}: <strong className="text-cyan-200">2 / 3</strong>
          </div>
        </GalaxyCard>
      </aside>
    </div>
  );
}
