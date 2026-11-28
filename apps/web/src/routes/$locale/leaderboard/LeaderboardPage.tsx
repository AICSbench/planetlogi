import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Award, Clock, Trophy } from "lucide-react";
import type { Locale } from "@planetlogi/i18n";
import { GalaxyCard } from "@planetlogi/ui";
import { AstroBuddyAvatar } from "../../../components/astro-buddy/AstroBuddyAvatar";
import { apiClient } from "../../../store/authStore";

export function LeaderboardPage({ locale }: { locale: Locale }) {
  const { t } = useTranslation("social");
  const [selectedCode, setSelectedCode] = useState("HERO_OF_DAY");
  const [scope, setScope] = useState<"CLASS" | "SCHOOL" | "NATIONAL">("NATIONAL");
  const [filterGrade, setFilterGrade] = useState<number>(0);
  const [viewMode, setViewMode] = useState<"TOP50" | "NEARBY">("TOP50");
  const [elapsed, setElapsed] = useState(0);

  const challengesQuery = useQuery({
    queryKey: ["lb-challenges", locale],
    queryFn: () => apiClient.getLeaderboardChallenges(locale),
  });

  const boardQuery = useQuery({
    queryKey: ["lb-board", selectedCode, scope, filterGrade, viewMode, locale],
    queryFn: () =>
      apiClient.getLeaderboardBoard(selectedCode, { scope, filterGrade, viewMode, locale }),
  });

  useEffect(() => {
    const id = window.setInterval(() => setElapsed((e) => e + 1), 1000);
    return () => window.clearInterval(id);
  }, []);

  const baseRefresh = boardQuery.data?.nextRefreshSec ?? 300;
  const countdown = Math.max(1, baseRefresh - (elapsed % baseRefresh));

  const challenges = challengesQuery.data?.challenges ?? [];
  const openChallenges = challenges.filter((c) => c.category === "OPEN");
  const seasonalChallenges = challenges.filter((c) => c.category === "SEASONAL");
  const board = boardQuery.data;

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-3xl font-extrabold text-white">{t("leaderboard.title")}</h1>
          <p className="mt-1 text-sm text-slate-300">{t("leaderboard.subtitle")}</p>
        </div>
        <span className="flex items-center gap-2 rounded-xl border border-cyan-400/35 bg-cyan-500/15 px-3.5 py-2 text-xs font-bold text-cyan-200 planetlogi-tabular">
          <Clock className="h-4 w-4" /> {t("leaderboard.refreshIn", { sec: countdown })}
        </span>
      </div>

      {/* 6 Open Challenges */}
      <div className="space-y-2">
        <p className="text-xs font-bold uppercase tracking-wider text-indigo-300">
          {t("leaderboard.openSection")}
        </p>
        <div className="grid gap-2.5 grid-cols-2 sm:grid-cols-3 lg:grid-cols-6">
          {openChallenges.map((c) => (
            <button
              key={c.code}
              type="button"
              onClick={() => setSelectedCode(c.code)}
              className={`min-h-20 rounded-2xl border p-3 text-left transition-colors cursor-pointer ${
                selectedCode === c.code
                  ? "border-cyan-400 bg-[#6366F1]/35 text-white"
                  : "border-indigo-400/20 bg-[#131B3A]/85 text-slate-200 hover:bg-[#1E2A55]"
              }`}
            >
              <span className="text-xl">{c.icon}</span>
              <p className="mt-1 text-xs font-extrabold leading-snug">{c.title}</p>
            </button>
          ))}
        </div>
      </div>

      {/* 4 Seasonal Challenges */}
      <div className="space-y-2">
        <p className="text-xs font-bold uppercase tracking-wider text-amber-300">
          {t("leaderboard.seasonalSection")}
        </p>
        <div className="grid gap-2.5 grid-cols-2 lg:grid-cols-4">
          {seasonalChallenges.map((c) => (
            <button
              key={c.code}
              type="button"
              onClick={() => setSelectedCode(c.code)}
              className={`min-h-20 rounded-2xl border p-3 text-left transition-colors cursor-pointer ${
                selectedCode === c.code
                  ? "border-amber-300 bg-amber-500/25 text-white"
                  : "border-amber-400/20 bg-[#131B3A]/85 text-slate-200 hover:bg-[#1E2A55]"
              }`}
            >
              <span className="text-xl">{c.icon}</span>
              <p className="mt-1 text-xs font-extrabold leading-snug">{c.title}</p>
            </button>
          ))}
        </div>
      </div>

      {/* Filters + Drill-down Board */}
      <div className="grid gap-6 lg:grid-cols-3">
        <GalaxyCard className="lg:col-span-2 space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <h2 className="text-xl font-extrabold text-white">
                {board?.challenge.icon} {board?.challenge.title}
              </h2>
              <p className="text-xs text-slate-300">{board?.challenge.description}</p>
            </div>
            {board && board.currentRank > 0 ? (
              <span className="rounded-xl bg-indigo-500/25 px-3 py-1.5 text-sm font-extrabold text-cyan-200 planetlogi-tabular">
                {t("leaderboard.yourRank", { rank: board.currentRank })}
              </span>
            ) : null}
          </div>

          {/* Scope, Grade & ViewMode controls */}
          <div className="flex flex-wrap items-center gap-2">
            {(["CLASS", "SCHOOL", "NATIONAL"] as const).map((sc) => (
              <button
                key={sc}
                type="button"
                onClick={() => setScope(sc)}
                className={`min-h-11 rounded-xl px-3.5 text-xs font-bold cursor-pointer ${
                  scope === sc ? "bg-[#6366F1] text-white" : "bg-white/10 text-slate-200"
                }`}
              >
                {t(`leaderboard.scopes.${sc}`)}
              </button>
            ))}

            <select
              value={filterGrade}
              onChange={(e) => setFilterGrade(Number(e.target.value))}
              className="min-h-11 rounded-xl border border-indigo-400/30 bg-[#0B1026] px-3 text-xs font-bold text-white"
            >
              <option value={0}>{t("leaderboard.allGrades")}</option>
              {[1, 2, 3, 4, 5, 6].map((g) => (
                <option key={g} value={g}>
                  {t("leaderboard.gradeOption", { grade: g })}
                </option>
              ))}
            </select>

            <div className="ml-auto flex gap-1.5">
              <button
                type="button"
                onClick={() => setViewMode("TOP50")}
                className={`min-h-11 rounded-xl px-3 text-xs font-bold cursor-pointer ${
                  viewMode === "TOP50" ? "bg-cyan-500 text-slate-950" : "bg-white/10 text-slate-200"
                }`}
              >
                {t("leaderboard.top50")}
              </button>
              <button
                type="button"
                onClick={() => setViewMode("NEARBY")}
                className={`min-h-11 rounded-xl px-3 text-xs font-bold cursor-pointer ${
                  viewMode === "NEARBY" ? "bg-cyan-500 text-slate-950" : "bg-white/10 text-slate-200"
                }`}
              >
                {t("leaderboard.nearby")}
              </button>
            </div>
          </div>

          {/* Table */}
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead>
                <tr className="border-b border-indigo-400/20 text-xs uppercase text-slate-400">
                  <th className="py-2 pr-3">{t("leaderboard.rankCol")}</th>
                  <th className="py-2 pr-3">{t("leaderboard.pilotCol")}</th>
                  <th className="py-2 pr-3">{t("leaderboard.gradeCol")}</th>
                  <th className="py-2 text-right">{t("leaderboard.scoreCol")}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-indigo-400/10">
                {(board?.entries ?? []).slice(0, 20).map((row) => (
                  <tr
                    key={`${row.rank}-${row.studentId}`}
                    className={row.isCurrent ? "bg-indigo-500/20 font-bold" : ""}
                  >
                    <td className="py-2.5 pr-3 font-extrabold text-amber-300 planetlogi-tabular">
                      #{row.rank}
                    </td>
                    <td className="py-2.5 pr-3">
                      <div className="flex items-center gap-2">
                        <AstroBuddyAvatar config={{ colorHex: row.buddyColor }} size={32} />
                        <span className="text-white">{row.maskedName}</span>
                        {row.isCurrent ? (
                          <span className="rounded bg-cyan-500/25 px-1.5 py-0.5 text-[10px] font-extrabold text-cyan-200">
                            {t("leaderboard.youBadge")}
                          </span>
                        ) : null}
                      </div>
                    </td>
                    <td className="py-2.5 pr-3 text-slate-300">G{row.grade}</td>
                    <td className="py-2.5 text-right font-extrabold text-cyan-200 planetlogi-tabular">
                      {row.score} {board?.challenge.metricLabel}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </GalaxyCard>

        {/* Hall of Fame Archive */}
        <GalaxyCard className="space-y-4">
          <div className="flex items-center gap-2">
            <Trophy className="h-5 w-5 text-amber-300" />
            <h2 className="text-lg font-extrabold text-white">{t("leaderboard.hallOfFame")}</h2>
          </div>
          <div className="space-y-3">
            {(board?.hallOfFame ?? []).map((hof) => (
              <div
                key={hof.season}
                className="flex items-center justify-between rounded-xl border border-amber-400/20 bg-amber-500/10 p-3"
              >
                <div className="flex items-center gap-2.5">
                  <Award className="h-5 w-5 text-amber-300" />
                  <div>
                    <p className="text-sm font-bold text-white">{hof.winnerName}</p>
                    <p className="text-xs text-slate-300">
                      {hof.season} • G{hof.grade}
                    </p>
                  </div>
                </div>
                <span className="text-sm font-extrabold text-amber-200 planetlogi-tabular">
                  {hof.score}
                </span>
              </div>
            ))}
          </div>
        </GalaxyCard>
      </div>
    </div>
  );
}
