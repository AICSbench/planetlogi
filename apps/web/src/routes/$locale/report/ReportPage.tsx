import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { Locale } from "@planetlogi/i18n";
import { GalaxyCard, ProficiencyPill, StarRating } from "@planetlogi/ui";
import { apiClient } from "../../../store/authStore";

export function ReportPage({ locale }: { locale: Locale }) {
  const { t } = useTranslation(["training", "common"]);
  const [tab, setTab] = useState<"PROFICIENCY" | "HIGH_SCORE">("PROFICIENCY");
  const [curriculum, setCurriculum] = useState("ALL");
  const [grade, setGrade] = useState(0);

  const profQuery = useQuery({
    queryKey: ["reports", "proficiency", curriculum, grade, locale],
    queryFn: () => apiClient.getProficiencyReport(curriculum, grade, locale),
  });

  const hsQuery = useQuery({
    queryKey: ["reports", "high-score", curriculum, grade, locale],
    queryFn: () => apiClient.getHighScoreReport(curriculum, grade, locale),
  });

  const prof = profQuery.data;
  const hs = hsQuery.data;

  return (
    <div className="space-y-6">
      <GalaxyCard className="space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h1 className="text-2xl font-extrabold text-white">📊 {t("training:reports.title")}</h1>
          <div className="flex rounded-xl bg-[#0B1026] p-1">
            {(["PROFICIENCY", "HIGH_SCORE"] as const).map((k) => (
              <button
                key={k}
                type="button"
                onClick={() => setTab(k)}
                className={`min-h-10 rounded-lg px-4 text-xs font-bold cursor-pointer ${
                  tab === k ? "bg-[#6366F1] text-white" : "text-slate-300"
                }`}
              >
                {k === "PROFICIENCY"
                  ? t("training:reports.tabProficiency")
                  : t("training:reports.tabHighScore")}
              </button>
            ))}
          </div>
        </div>

        {/* Filters: All 6 Curricula & Grades 1-6 */}
        <div className="flex flex-wrap gap-3">
          <select
            value={curriculum}
            onChange={(e) => setCurriculum(e.target.value)}
            aria-label="Curriculum filter"
            className="min-h-11 rounded-xl border border-indigo-400/30 bg-[#0B1026] px-3 text-sm text-white"
          >
            <option value="ALL">{t("training:reports.allCurricula")}</option>
            <option value="KURIKULUM_MERDEKA">Kurikulum Merdeka</option>
            <option value="SINGAPORE_MOE">Singapore Math MOE</option>
            <option value="CAMBRIDGE_0096">Cambridge Primary 0096</option>
            <option value="CHALLENGING_MATHS">Challenging Maths</option>
            <option value="HOTS_WHEEL">HOTS Constellation</option>
            <option value="EXPLORER_SERIES">Real-World Explorer Series</option>
          </select>

          <select
            value={grade}
            onChange={(e) => setGrade(Number(e.target.value))}
            aria-label="Grade filter"
            className="min-h-11 rounded-xl border border-indigo-400/30 bg-[#0B1026] px-3 text-sm text-white"
          >
            <option value={0}>{t("training:reports.allGrades")}</option>
            {[1, 2, 3, 4, 5, 6].map((g) => (
              <option key={g} value={g}>
                {t("common:common.grade", { grade: g })}
              </option>
            ))}
          </select>
        </div>
      </GalaxyCard>

      {tab === "PROFICIENCY" && prof ? (
        <>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <GalaxyCard className="border-emerald-400/40 text-center">
              <p className="text-3xl font-extrabold text-emerald-300 planetlogi-tabular">
                {prof.masteryCount}
              </p>
              <p className="text-xs font-bold text-slate-300">{t("common:proficiency.mastery")}</p>
            </GalaxyCard>
            <GalaxyCard className="border-blue-400/40 text-center">
              <p className="text-3xl font-extrabold text-blue-300 planetlogi-tabular">
                {prof.competentCount}
              </p>
              <p className="text-xs font-bold text-slate-300">{t("common:proficiency.competent")}</p>
            </GalaxyCard>
            <GalaxyCard className="border-amber-400/40 text-center">
              <p className="text-3xl font-extrabold text-amber-300 planetlogi-tabular">
                {prof.beginningCount}
              </p>
              <p className="text-xs font-bold text-slate-300">{t("common:proficiency.beginning")}</p>
            </GalaxyCard>
            <GalaxyCard className="border-slate-500/40 text-center">
              <p className="text-3xl font-extrabold text-slate-300 planetlogi-tabular">
                {prof.incompleteCount}
              </p>
              <p className="text-xs font-bold text-slate-300">
                {t("common:proficiency.incomplete")}
              </p>
            </GalaxyCard>
          </div>

          <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
            {prof.skills.map((sk) => (
              <GalaxyCard key={sk.code} className="flex items-center justify-between gap-3">
                <div className="space-y-1">
                  <p className="text-sm font-bold text-white">{sk.name}</p>
                  <div className="flex flex-wrap items-center gap-2 text-xs text-slate-300">
                    <ProficiencyPill
                      tier={sk.proficiencyTier}
                      label={t(`common:proficiency.${sk.proficiencyTier.toLowerCase()}`)}
                      pct={sk.proficiencyPct}
                    />
                    <span>{t("common:common.grade", { grade: sk.grade })}</span>
                  </div>
                </div>
                <Link
                  to="/$locale/training"
                  params={{ locale }}
                  className="rounded-xl bg-[#6366F1] px-3 py-2 text-xs font-bold text-white hover:bg-[#5558E6]"
                >
                  {t("training:reports.practiceNow")}
                </Link>
              </GalaxyCard>
            ))}
          </div>
        </>
      ) : null}

      {tab === "HIGH_SCORE" && hs ? (
        <div className="space-y-4">
          {(
            [
              ["tier9To10", hs.tier9To10, 3],
              ["tier6To8", hs.tier6To8, 2],
              ["tier4To5", hs.tier4To5, 1],
              ["tier1To3", hs.tier1To3, 0],
              ["unplayed", hs.unplayed, 0],
            ] as const
          ).map(([key, list, stars]) => (
            <GalaxyCard key={key} className="space-y-3">
              <div className="flex items-center justify-between">
                <h2 className="text-base font-extrabold text-white">
                  {t(`training:reports.${key}`)} ({list.length})
                </h2>
                {key !== "unplayed" ? <StarRating stars={stars} /> : null}
              </div>
              <div className="flex flex-wrap gap-2">
                {list.map((sk) => (
                  <span
                    key={sk.code}
                    className="rounded-xl border border-indigo-400/20 bg-[#1E2A55] px-3 py-1.5 text-xs font-semibold text-slate-200"
                  >
                    {sk.name} ({sk.bestSessionScore}/10)
                  </span>
                ))}
              </div>
            </GalaxyCard>
          ))}
        </div>
      ) : null}
    </div>
  );
}
