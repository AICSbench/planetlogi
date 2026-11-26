import { useRef, useState, type KeyboardEvent } from "react";
import { Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { formatNumber, type Locale } from "@planetlogi/i18n";
import {
  CurrencyPill,
  GalaxyButton,
  GalaxyCard,
  getPlanetForGrade,
} from "@planetlogi/ui";
import { apiClient, useAuthStore } from "../../../store/authStore";

const PILLARS = ["challenges", "training", "academy", "fun"] as const;
type PillarKey = (typeof PILLARS)[number];

export function DashboardPreviewPage({ locale }: { locale: Locale }) {
  const { t } = useTranslation(["dashboard", "common", "training"]);
  const user = useAuthStore((s) => s.user);
  const [pillar, setPillar] = useState<PillarKey>("challenges");
  const tabRefs = useRef<Record<PillarKey, HTMLButtonElement | null>>({
    challenges: null,
    training: null,
    academy: null,
    fun: null,
  });

  function handleTabKeyDown(e: KeyboardEvent<HTMLButtonElement>, idx: number) {
    let nextIdx: number | null = null;
    if (e.key === "ArrowRight") nextIdx = (idx + 1) % PILLARS.length;
    else if (e.key === "ArrowLeft") nextIdx = (idx - 1 + PILLARS.length) % PILLARS.length;
    else if (e.key === "Home") nextIdx = 0;
    else if (e.key === "End") nextIdx = PILLARS.length - 1;

    if (nextIdx !== null) {
      e.preventDefault();
      const nextKey = PILLARS[nextIdx]!;
      setPillar(nextKey);
      tabRefs.current[nextKey]?.focus();
    }
  }

  const grade = user?.grade ?? 1;
  const planet = getPlanetForGrade(grade);
  const name = user?.displayName ?? "Raka";

  const orbitQuery = useQuery({
    queryKey: ["daily-orbit-today", locale, grade],
    queryFn: () => apiClient.getDailyOrbitToday(locale, grade),
    retry: false,
  });

  const streakQuery = useQuery({
    queryKey: ["streak-status"],
    queryFn: () => apiClient.getStreakStatus(),
    retry: false,
  });

  const todaySp = orbitQuery.data?.todaySpBreakdown.totalToday ?? 0;
  const totalSp = (user ? (user.totalSp ?? 0) : 12) + todaySp;
  const astroCoins = user ? (user.astroCoins ?? 0) : 25;
  const cryoShields = streakQuery.data?.cryoShields ?? 2;

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
              <div className="mt-0.5 flex items-center gap-2 text-xs font-semibold text-cyan-300">
                <span>{t("common:common.grade", { grade })}</span>
                <span>•</span>
                <Link
                  to="/$locale/exp-level"
                  params={{ locale }}
                  className="rounded-full bg-indigo-500/30 px-2 py-0.5 text-indigo-100 hover:bg-indigo-500/50"
                >
                  ⚡ {t("common:common.level", { level: user?.pilotLevel ?? 2 })}
                </Link>
              </div>
            </div>
          </div>

          <p className="rounded-xl bg-[#1E2A55]/80 px-3 py-2 text-xs font-medium text-slate-200">
            🪐 {t("dashboard:planetLabel", { planet: planet.name[locale] })}
          </p>

          <div className="grid grid-cols-2 gap-2">
            <Link to="/$locale/streak" params={{ locale }}>
              <GalaxyButton variant="secondary" className="w-full text-xs">
                🔥 {streakQuery.data?.currentStreak ?? (user ? (user.currentStreak ?? 0) : 3)}d
              </GalaxyButton>
            </Link>
            <Link to="/$locale/report" params={{ locale }}>
              <GalaxyButton variant="secondary" className="w-full text-xs">
                📊 {t("common:nav.report")}
              </GalaxyButton>
            </Link>
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
          {PILLARS.map((key, idx) => (
            <button
              key={key}
              ref={(el) => {
                tabRefs.current[key] = el;
              }}
              id={`pillar-tab-${key}`}
              role="tab"
              type="button"
              tabIndex={pillar === key ? 0 : -1}
              aria-selected={pillar === key}
              aria-controls={`pillar-panel-${key}`}
              onClick={() => setPillar(key)}
              onKeyDown={(e) => handleTabKeyDown(e, idx)}
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

        <div
          role="tabpanel"
          id={`pillar-panel-${pillar}`}
          aria-labelledby={`pillar-tab-${pillar}`}
        >
          {pillar === "challenges" ? (
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <Link to="/$locale/daily-orbit" params={{ locale }} className="block">
                <GalaxyCard className="h-full hover:border-[#FBBF24]/60 transition-colors space-y-2">
                  <div className="flex items-center justify-between">
                    <h2 className="text-lg font-bold text-[#FBBF24]">
                      🪐 {t("dashboard:cards.dailyOrbitTitle")}
                    </h2>
                    <span className="rounded-full bg-amber-400/20 px-2.5 py-0.5 text-xs font-bold text-amber-200 planetlogi-tabular">
                      {orbitQuery.data?.completedCount ?? 0}/10
                    </span>
                  </div>
                  <p className="text-sm text-slate-300">{t("dashboard:cards.dailyOrbitDesc")}</p>
                </GalaxyCard>
              </Link>

              <Link to="/$locale/$" params={{ locale, _splat: "super-nova" }} className="block">
                <GalaxyCard className="h-full hover:border-[#22D3EE]/60 transition-colors space-y-2">
                  <h2 className="text-lg font-bold text-[#22D3EE]">
                    ⚡ {t("dashboard:cards.superNovaTitle")}
                  </h2>
                  <p className="text-sm text-slate-300">{t("dashboard:cards.superNovaDesc")}</p>
                </GalaxyCard>
              </Link>

              <Link
                to="/$locale/$"
                params={{ locale, _splat: "star-duels" }}
                className="block sm:col-span-2"
              >
                <GalaxyCard className="hover:border-indigo-400/60 transition-colors space-y-2">
                  <h2 className="text-lg font-bold text-indigo-300">
                    ⚔️ {t("dashboard:cards.starDuelsTitle")}
                  </h2>
                  <p className="text-sm text-slate-300">{t("dashboard:cards.starDuelsDesc")}</p>
                </GalaxyCard>
              </Link>
            </div>
          ) : null}

          {pillar === "training" ? (
            <Link to="/$locale/training" params={{ locale }} className="block">
              <GalaxyCard className="hover:border-emerald-400/60 transition-colors space-y-2">
                <h2 className="text-lg font-bold text-emerald-300">
                  🌌 {t("dashboard:cards.galaxyTrainingTitle")}
                </h2>
                <p className="text-sm text-slate-300">{t("dashboard:cards.galaxyTrainingDesc")}</p>
                <div className="pt-2">
                  <GalaxyButton variant="primary">{t("training:practiceBtn")} →</GalaxyButton>
                </div>
              </GalaxyCard>
            </Link>
          ) : null}

          {pillar === "academy" ? (
            <Link to="/$locale/$" params={{ locale, _splat: "academy" }} className="block">
              <GalaxyCard className="hover:border-cyan-400/60 transition-colors space-y-2">
                <h2 className="text-lg font-bold text-cyan-300">
                  🚀 {t("dashboard:cards.spaceAcademyTitle")}
                </h2>
                <p className="text-sm text-slate-300">{t("dashboard:cards.spaceAcademyDesc")}</p>
              </GalaxyCard>
            </Link>
          ) : null}

          {pillar === "fun" ? (
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <Link to="/$locale/$" params={{ locale, _splat: "stories" }} className="block">
                <GalaxyCard className="h-full hover:border-amber-400/60 transition-colors space-y-2">
                  <h2 className="text-lg font-bold text-amber-300">
                    📖 {t("dashboard:cards.planetStoriesTitle")}
                  </h2>
                  <p className="text-sm text-slate-300">{t("dashboard:cards.planetStoriesDesc")}</p>
                </GalaxyCard>
              </Link>
              <Link to="/$locale/$" params={{ locale, _splat: "brain-galaxy" }} className="block">
                <GalaxyCard className="h-full hover:border-pink-400/60 transition-colors space-y-2">
                  <h2 className="text-lg font-bold text-pink-300">
                    🧠 {t("dashboard:cards.brainGalaxyTitle")}
                  </h2>
                  <p className="text-sm text-slate-300">{t("dashboard:cards.brainGalaxyDesc")}</p>
                </GalaxyCard>
              </Link>
            </div>
          ) : null}
        </div>
      </section>

      {/* Right Currency Summary Panel */}
      <aside className="lg:col-span-3 space-y-4">
        <GalaxyCard className="space-y-3">
          <h2 className="text-sm font-bold uppercase tracking-wider text-slate-300">
            {t("dashboard:wallet.todaySp")} & {t("dashboard:wallet.astroCoins")}
          </h2>
          <div className="space-y-2">
            <div className="flex items-center justify-between text-xs text-slate-300">
              <span>{t("dashboard:wallet.todaySp")}</span>
              <CurrencyPill
                kind="SP"
                value={`+${formatNumber(todaySp, locale)}`}
                label={t("common:currencies.sp")}
              />
            </div>
            <div className="flex items-center justify-between text-xs text-slate-300">
              <span>{t("dashboard:wallet.totalSp")}</span>
              <CurrencyPill
                kind="SP"
                value={formatNumber(totalSp, locale)}
                label={t("common:currencies.sp")}
              />
            </div>
            <div className="flex items-center justify-between text-xs text-slate-300">
              <span>{t("dashboard:wallet.astroCoins")}</span>
              <CurrencyPill
                kind="AC"
                value={formatNumber(astroCoins, locale)}
                label={t("common:currencies.ac")}
              />
            </div>
          </div>
          <Link
            to="/$locale/streak"
            params={{ locale }}
            className="block rounded-xl bg-[#1E2A55]/70 p-3 text-xs text-slate-300 hover:bg-[#2A3A73]"
          >
            ❄️ {t("dashboard:wallet.cryoShields")}:{" "}
            <strong className="text-cyan-200">{cryoShields} / 3</strong>
          </Link>
        </GalaxyCard>
      </aside>
    </div>
  );
}
