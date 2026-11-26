import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Bell, Flame, Shield } from "lucide-react";
import { GalaxyButton, GalaxyCard } from "@planetlogi/ui";
import { apiClient } from "../../../store/authStore";

const PET_EMOJIS = ["🥚", "✨", "🐣", "🦊", "🔥🦅"] as const;

export function StreakPage() {
  const { t } = useTranslation("daily-orbit");
  const queryClient = useQueryClient();

  const statusQuery = useQuery({
    queryKey: ["streak-status"],
    queryFn: () => apiClient.getStreakStatus(),
  });

  const calQuery = useQuery({
    queryKey: ["streak-calendar"],
    queryFn: () => apiClient.getStreakCalendar(),
  });

  const reminderMutation = useMutation({
    mutationFn: (enabled: boolean) => apiClient.setStreakReminder(enabled, "19:00"),
    onSuccess: (next) => {
      queryClient.setQueryData(["streak-status"], next);
    },
  });

  const status = statusQuery.data;
  const days = calQuery.data?.days ?? [];
  const petEmoji = PET_EMOJIS[Math.min(4, Math.max(0, (status?.novaPetStage ?? 1) - 1))];
  const pct = Math.min(
    100,
    Math.round(((status?.todayCorrectCount ?? 0) / (status?.dailyMilestoneTarget ?? 5)) * 100)
  );

  return (
    <div className="mx-auto max-w-4xl space-y-6">
      <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
        {/* Nova Pet Growth Tracker */}
        <GalaxyCard className="space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-2xl font-extrabold text-white">{t("streak.title")}</h1>
              <p className="mt-1 text-sm text-slate-300">{t("streak.subtitle")}</p>
            </div>
            <span className="text-5xl" aria-hidden="true">
              {petEmoji}
            </span>
          </div>

          <div className="flex items-center gap-3">
            <span className="inline-flex items-center gap-1.5 rounded-full bg-orange-500/20 border border-orange-400/40 px-4 py-1.5 text-base font-extrabold text-orange-300">
              <Flame className="h-5 w-5 text-[#F97316]" aria-hidden="true" />
              {t("streak.currentDays", { count: status?.currentStreak ?? 3 })}
            </span>
            <span className="text-xs font-semibold text-slate-300">
              {t("streak.longestDays", { count: status?.longestStreak ?? 3 })}
            </span>
          </div>

          <div className="space-y-1.5">
            <div className="flex justify-between text-xs font-semibold text-cyan-200">
              <span>
                {t("streak.todayProgress", {
                  current: status?.todayCorrectCount ?? 0,
                  target: status?.dailyMilestoneTarget ?? 5,
                })}
              </span>
              <span>{t("streak.petStage", { stage: status?.novaPetStage ?? 2 })}</span>
            </div>
            <div className="h-3 w-full overflow-hidden rounded-full bg-[#0B1026]">
              <div
                className="h-full rounded-full bg-gradient-to-r from-[#F97316] to-[#FBBF24] transition-all"
                style={{ width: `${pct}%` }}
              />
            </div>
          </div>

          <GalaxyButton
            variant={status?.reminderEnabled ? "secondary" : "primary"}
            onClick={() => reminderMutation.mutate(!status?.reminderEnabled)}
          >
            <Bell className="h-4 w-4" aria-hidden="true" />
            <span>
              {status?.reminderEnabled
                ? t("streak.reminderOn", { time: status.reminderTimeLocal ?? "19:00" })
                : t("streak.reminderOff")}
            </span>
          </GalaxyButton>
        </GalaxyCard>

        {/* Cryo Shields Panel */}
        <GalaxyCard className="space-y-4">
          <div className="flex items-center gap-2">
            <Shield className="h-6 w-6 text-[#22D3EE]" aria-hidden="true" />
            <h2 className="text-xl font-extrabold text-white">{t("streak.cryoTitle")}</h2>
          </div>
          <p className="text-sm text-slate-300">{t("streak.cryoDesc")}</p>
          <div className="flex items-center gap-3 pt-2">
            {[1, 2, 3].map((slot) => {
              const filled = slot <= (status?.cryoShields ?? 2);
              return (
                <div
                  key={slot}
                  className={`flex h-20 flex-1 flex-col items-center justify-center rounded-2xl border-2 text-2xl ${
                    filled
                      ? "border-[#22D3EE] bg-cyan-400/15 text-cyan-200"
                      : "border-slate-700 bg-[#0B1026] text-slate-600"
                  }`}
                >
                  <span>❄️</span>
                  <span className="mt-1 text-[11px] font-bold">
                    {filled ? "ACTIVE" : "EMPTY"}
                  </span>
                </div>
              );
            })}
          </div>
        </GalaxyCard>
      </div>

      {/* Monthly Orbit Calendar */}
      <GalaxyCard className="space-y-4">
        <h2 className="text-lg font-extrabold text-white">{t("streak.calendarTitle")}</h2>
        {days.length === 0 ? (
          <p className="text-sm text-slate-400">
            Complete 5 Daily Orbit questions today to light up your first star on this month&apos;s
            calendar!
          </p>
        ) : (
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-4 md:grid-cols-7">
            {days.map((d) => (
              <div
                key={d.date}
                className="rounded-xl border border-indigo-400/20 bg-[#1E2A55] p-2.5 text-center text-xs"
              >
                <p className="font-mono text-slate-300">{d.date}</p>
                <p className="mt-1 text-base">
                  {d.status === "COMPLETED" ? "🔥" : d.status === "FROZEN" ? "❄️" : "○"}
                </p>
                <p className="font-semibold text-white">
                  {d.status === "COMPLETED"
                    ? t("streak.statusCompleted")
                    : d.status === "FROZEN"
                      ? t("streak.statusFrozen")
                      : t("streak.statusMissed")}
                </p>
              </div>
            ))}
          </div>
        )}
      </GalaxyCard>
    </div>
  );
}
