import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Lock, Sparkles, Star } from "lucide-react";
import type { Locale } from "@planetlogi/i18n";
import type { DailyBadgeCard } from "@planetlogi/api-client";
import { GalaxyButton, GalaxyCard } from "@planetlogi/ui";
import { apiClient } from "../../../store/authStore";

function tierClasses(tier: number): string {
  switch (tier) {
    case 4:
      return "border-cyan-300/60 bg-gradient-to-br from-fuchsia-600/35 via-indigo-600/35 to-cyan-500/35 shadow-lg shadow-cyan-500/20";
    case 3:
      return "border-amber-300/60 bg-gradient-to-br from-amber-500/30 to-yellow-700/30";
    case 2:
      return "border-slate-300/50 bg-gradient-to-br from-slate-400/25 to-slate-700/35";
    case 1:
      return "border-orange-400/50 bg-gradient-to-br from-orange-600/25 to-amber-900/35";
    default:
      return "border-indigo-400/20 bg-[#0B1026]/80 opacity-85";
  }
}

export function AchievementBadgesPage({ locale }: { locale: Locale }) {
  const { t } = useTranslation("stories");
  const qc = useQueryClient();
  const [month, setMonth] = useState("2026-11");
  const [flipped, setFlipped] = useState<Record<string, boolean>>({});
  const [recoverCard, setRecoverCard] = useState<DailyBadgeCard | null>(null);
  const [toast, setToast] = useState<string | null>(null);

  const calendarQuery = useQuery({
    queryKey: ["daily-badges", month, locale],
    queryFn: () => apiClient.getDailyBadgesCalendar(month, locale),
  });

  const recoverMut = useMutation({
    mutationFn: (date: string) => apiClient.recoverDailyBadge(date, locale),
    onSuccess: () => {
      setRecoverCard(null);
      setToast(t("badges.recoveredToast"));
      void qc.invalidateQueries({ queryKey: ["daily-badges"] });
    },
    onError: () => {
      setRecoverCard(null);
      setToast(t("badges.recoverError"));
    },
  });

  const cards = calendarQuery.data?.cards ?? [];

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-3xl font-extrabold text-white">{t("badges.title")}</h1>
          <p className="mt-1 text-sm text-slate-300">{t("badges.subtitle")}</p>
        </div>
        <label className="flex items-center gap-2 text-sm font-semibold text-slate-200">
          <span>{t("badges.monthLabel")}</span>
          <input
            type="month"
            value={month}
            onChange={(e) => setMonth(e.target.value)}
            className="min-h-11 rounded-xl border border-indigo-400/35 bg-[#131B3A] px-3 text-white"
          />
        </label>
      </div>

      {toast ? (
        <div className="rounded-xl border border-cyan-400/40 bg-cyan-500/15 px-4 py-2.5 text-sm font-semibold text-cyan-200">
          {toast}
        </div>
      ) : null}

      <div className="grid gap-3 grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
        {cards.map((card) => {
          const isFlipped = Boolean(flipped[card.date]);
          return (
            <div
              key={card.date}
              onClick={() => {
                if (card.tier > 0) {
                  setFlipped((f) => ({ ...f, [card.date]: !f[card.date] }));
                } else {
                  setRecoverCard(card);
                }
              }}
              role="button"
              tabIndex={0}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  if (card.tier > 0) setFlipped((f) => ({ ...f, [card.date]: !f[card.date] }));
                  else setRecoverCard(card);
                }
              }}
              className={`min-h-40 rounded-2xl border p-3.5 transition-all duration-200 hover:-translate-y-0.5 cursor-pointer flex flex-col justify-between ${tierClasses(
                card.tier
              )}`}
            >
              <div className="flex items-center justify-between">
                <span className="text-xs font-extrabold text-white planetlogi-tabular">
                  #{card.dayOfMonth}
                </span>
                {card.tier > 0 ? (
                  <span className="flex items-center gap-0.5 text-amber-300">
                    {Array.from({ length: card.tier }).map((_, i) => (
                      <Star key={i} className="h-3.5 w-3.5 fill-amber-300" />
                    ))}
                  </span>
                ) : (
                  <Lock className="h-4 w-4 text-slate-400" />
                )}
              </div>

              {card.tier === 0 ? (
                <div className="my-auto text-center space-y-1.5">
                  <p className="text-xs font-semibold text-slate-400">{t("badges.locked")}</p>
                  <span className="inline-block rounded-lg bg-amber-500/20 px-2 py-1 text-[11px] font-bold text-amber-200">
                    {t("badges.recoverAction", { cost: card.recoverCostAc })}
                  </span>
                </div>
              ) : isFlipped ? (
                <div className="my-auto space-y-1">
                  <p className="text-xs font-bold text-cyan-200">{card.triviaTitle}</p>
                  <p className="text-xs leading-snug text-slate-100">{card.triviaFact}</p>
                </div>
              ) : (
                <div className="my-auto text-center space-y-1">
                  <Sparkles className="mx-auto h-6 w-6 text-amber-200" />
                  <p className="text-xs font-extrabold text-white">{card.tierName}</p>
                  <p className="text-[11px] text-slate-300">{t("badges.flipHint")}</p>
                </div>
              )}

              <p className="text-[10px] font-semibold uppercase tracking-wider text-slate-300/80">
                {card.category}
              </p>
            </div>
          );
        })}
      </div>

      {/* 20 AC Recovery Modal */}
      {recoverCard ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4">
          <GalaxyCard className="max-w-md w-full space-y-4">
            <h2 className="text-xl font-extrabold text-white">{t("badges.recoverConfirmTitle")}</h2>
            <p className="text-sm text-slate-300">
              {t("badges.recoverConfirmBody", { date: recoverCard.date })}
            </p>
            <div className="flex justify-end gap-2">
              <GalaxyButton variant="secondary" onClick={() => setRecoverCard(null)}>
                {t("badges.cancel")}
              </GalaxyButton>
              <GalaxyButton
                onClick={() => recoverMut.mutate(recoverCard.date)}
                disabled={recoverMut.isPending}
              >
                {t("badges.recoverConfirm")}
              </GalaxyButton>
            </div>
          </GalaxyCard>
        </div>
      ) : null}
    </div>
  );
}
