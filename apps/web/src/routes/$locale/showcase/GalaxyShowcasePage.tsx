import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Trophy } from "lucide-react";
import type { Locale } from "@planetlogi/i18n";
import { GalaxyCard } from "@planetlogi/ui";
import { AstroBuddyAvatar } from "../../../components/astro-buddy/AstroBuddyAvatar";
import { apiClient, useAuthStore } from "../../../store/authStore";

export function GalaxyShowcasePage({ locale }: { locale: Locale }) {
  const { t } = useTranslation("stories");
  const user = useAuthStore((s) => s.user);
  const [grade, setGrade] = useState<number>(user?.grade ?? 1);

  const showcaseQuery = useQuery({
    queryKey: ["galaxy-showcase", grade, locale],
    queryFn: () => apiClient.getGalaxyShowcase(grade),
  });

  const entries = showcaseQuery.data?.entries ?? [];

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-extrabold text-white">{t("showcase.title")}</h1>
        <p className="mt-1 text-sm text-slate-300">{t("showcase.subtitle")}</p>
      </div>

      <div className="flex flex-wrap gap-2">
        {[1, 2, 3, 4, 5, 6].map((g) => (
          <button
            key={g}
            type="button"
            onClick={() => setGrade(g)}
            className={`min-h-11 rounded-xl px-4 text-sm font-bold cursor-pointer ${
              grade === g ? "bg-[#6366F1] text-white" : "bg-[#1E2A55] text-slate-200"
            }`}
          >
            {t("showcase.gradeFilter", { grade: g })}
          </button>
        ))}
      </div>

      {entries.length === 0 ? (
        <GalaxyCard className="py-10 text-center text-slate-300">{t("showcase.empty")}</GalaxyCard>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {entries.map((entry) => (
            <GalaxyCard
              key={`${entry.rank}-${entry.maskedName}`}
              className="flex flex-col items-center text-center space-y-3"
            >
              <div className="flex w-full items-center justify-between">
                <span className="flex items-center gap-1 rounded-lg bg-amber-500/20 px-2.5 py-1 text-xs font-extrabold text-amber-300">
                  <Trophy className="h-3.5 w-3.5" /> {t("showcase.rankLabel", { rank: entry.rank })}
                </span>
                <span className="rounded-lg bg-indigo-500/25 px-2.5 py-1 text-xs font-bold text-cyan-200 planetlogi-tabular">
                  {t("showcase.spLabel", { sp: entry.starPoints })}
                </span>
              </div>
              <AstroBuddyAvatar config={entry.buddy} size={120} />
              <div>
                <p className="text-lg font-extrabold text-white">{entry.buddy.fullName}</p>
                <p className="text-xs font-semibold text-slate-400">{entry.maskedName}</p>
              </div>
            </GalaxyCard>
          ))}
        </div>
      )}
    </div>
  );
}
