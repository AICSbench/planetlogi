import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Check, Copy, Crown, Users } from "lucide-react";
import type { Locale } from "@planetlogi/i18n";
import { GalaxyButton, GalaxyCard } from "@planetlogi/ui";
import { AstroBuddyAvatar } from "../../../components/astro-buddy/AstroBuddyAvatar";
import { apiClient, useAuthStore } from "../../../store/authStore";

export function StarCrewPage({ locale }: { locale: Locale }) {
  const { t } = useTranslation("social");
  const qc = useQueryClient();
  const user = useAuthStore((s) => s.user);
  const grade = user?.grade ?? 1;

  const [joinCode, setJoinCode] = useState("");
  const [copied, setCopied] = useState(false);
  const [notice, setNotice] = useState<{ kind: "ok" | "err"; text: string } | null>(null);

  const crewQuery = useQuery({
    queryKey: ["star-crew", grade, locale],
    queryFn: () => apiClient.getStarCrew(grade),
  });

  const joinMut = useMutation({
    mutationFn: (code: string) => apiClient.joinStarCrew(code, grade),
    onSuccess: (crew) => {
      qc.setQueryData(["star-crew", grade, locale], crew);
      setJoinCode("");
      setNotice({ kind: "ok", text: t("crew.joinSuccess") });
    },
    onError: () => setNotice({ kind: "err", text: t("crew.joinError") }),
  });

  const leaveMut = useMutation({
    mutationFn: () => apiClient.leaveStarCrew(grade),
    onSuccess: (crew) => {
      qc.setQueryData(["star-crew", grade, locale], crew);
      setNotice(null);
    },
  });

  const crew = crewQuery.data;

  async function handleCopy() {
    if (!crew) return;
    try {
      await navigator.clipboard.writeText(crew.formattedCode);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      // clipboard fallback ignored in headless browsers
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-extrabold text-white">{t("crew.title")}</h1>
        <p className="mt-1 text-sm text-slate-300">{t("crew.subtitle")}</p>
      </div>

      {notice ? (
        <div
          className={`rounded-xl px-4 py-3 text-sm font-semibold ${
            notice.kind === "ok"
              ? "bg-emerald-500/20 text-emerald-200"
              : "bg-rose-500/20 text-rose-200"
          }`}
        >
          {notice.text}
        </div>
      ) : null}

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Crew Identity & 9-Digit Code */}
        <GalaxyCard className="space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-xl font-extrabold text-white">{crew?.name ?? "Star Crew"}</h2>
            <span className="rounded-xl bg-amber-500/20 px-3 py-1.5 text-sm font-extrabold text-amber-200 planetlogi-tabular">
              {t("crew.weeklyTotal")}: {crew?.weeklyTotalSp ?? 0} SP
            </span>
          </div>

          <div className="rounded-2xl border border-cyan-400/35 bg-[#0B1026] p-4 flex flex-wrap items-center justify-between gap-3">
            <div>
              <p className="text-xs font-bold uppercase tracking-wider text-cyan-300">
                {t("crew.yourCode")}
              </p>
              <p className="mt-1 text-2xl font-extrabold tracking-widest text-white planetlogi-tabular">
                {crew?.formattedCode ?? "---"}
              </p>
            </div>
            <GalaxyButton variant="secondary" onClick={handleCopy}>
              {copied ? <Check className="h-4 w-4 mr-1" /> : <Copy className="h-4 w-4 mr-1" />}
              {copied ? t("crew.copied") : t("crew.copyCode")}
            </GalaxyButton>
          </div>

          <div className="space-y-2">
            <p className="text-xs font-bold uppercase tracking-wider text-slate-300 flex items-center gap-1.5">
              <Users className="h-4 w-4" /> {t("crew.membersTitle")}
            </p>
            {(crew?.members ?? []).map((m) => (
              <div
                key={m.studentId}
                className="flex items-center justify-between rounded-xl bg-white/5 px-3 py-2"
              >
                <div className="flex items-center gap-2.5">
                  <AstroBuddyAvatar config={{ colorHex: m.buddyColor }} size={40} />
                  <div>
                    <p className="text-sm font-bold text-white flex items-center gap-1.5">
                      {m.maskedName}
                      {m.isLeader ? (
                        <span className="inline-flex items-center gap-1 rounded bg-amber-500/20 px-1.5 py-0.5 text-[10px] font-bold text-amber-300">
                          <Crown className="h-3 w-3" /> {t("crew.leaderBadge")}
                        </span>
                      ) : null}
                    </p>
                  </div>
                </div>
                <span className="text-sm font-extrabold text-cyan-200 planetlogi-tabular">
                  {m.weeklySp} SP
                </span>
              </div>
            ))}
          </div>
        </GalaxyCard>

        {/* Join Friend's Crew */}
        <GalaxyCard className="space-y-4">
          <h2 className="text-xl font-extrabold text-white">{t("crew.joinTitle")}</h2>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              if (joinCode.trim()) joinMut.mutate(joinCode.trim());
            }}
            className="space-y-3"
          >
            <input
              type="text"
              value={joinCode}
              onChange={(e) => setJoinCode(e.target.value)}
              placeholder={t("crew.joinPlaceholder")}
              className="min-h-12 w-full rounded-xl border border-indigo-400/35 bg-[#0B1026] px-4 text-base font-bold text-white"
            />
            <div className="flex flex-wrap gap-2">
              <GalaxyButton type="submit" disabled={joinMut.isPending}>
                {t("crew.joinButton")}
              </GalaxyButton>
              <GalaxyButton
                type="button"
                variant="secondary"
                onClick={() => leaveMut.mutate()}
                disabled={leaveMut.isPending}
              >
                {t("crew.leaveButton")}
              </GalaxyButton>
            </div>
          </form>
        </GalaxyCard>
      </div>
    </div>
  );
}
