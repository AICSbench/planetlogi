import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { BatteryCharging, History, Inbox, ShieldCheck, Swords, Users } from "lucide-react";
import type { Locale } from "@planetlogi/i18n";
import type { DuelRecord } from "@planetlogi/api-client";
import { GalaxyButton, GalaxyCard } from "@planetlogi/ui";
import { AstroBuddyAvatar } from "../../../components/astro-buddy/AstroBuddyAvatar";
import { apiClient } from "../../../store/authStore";

export function StarDuelsPage({ locale }: { locale: Locale }) {
  const { t } = useTranslation("social");
  const qc = useQueryClient();

  const [tab, setTab] = useState<"challenge" | "incoming" | "history" | "rules">("challenge");
  const [activeDuel, setActiveDuel] = useState<DuelRecord | null>(null);
  const [answers, setAnswers] = useState<Record<number, Record<string, string>>>({});
  const [completedDuel, setCompletedDuel] = useState<DuelRecord | null>(null);
  const [errMsg, setErrMsg] = useState<string | null>(null);

  const lobbyQuery = useQuery({
    queryKey: ["duels-lobby", locale],
    queryFn: () => apiClient.getDuelLobby(locale),
  });

  const startMut = useMutation({
    mutationFn: (vars: { mode: "CLASSMATE" | "CREW" | "RANDOM"; opponentId?: string }) =>
      apiClient.startDuel(vars.mode, vars.opponentId, locale),
    onSuccess: (res) => {
      setActiveDuel(res.duel);
      setAnswers({});
      setCompletedDuel(null);
      setErrMsg(null);
      void qc.invalidateQueries({ queryKey: ["duels-lobby"] });
    },
    onError: () => setErrMsg(t("duels.noEnergy")),
  });

  const acceptMut = useMutation({
    mutationFn: (id: string) => apiClient.acceptDuel(id, locale),
    onSuccess: (duel) => {
      setActiveDuel(duel);
      setAnswers({});
      setCompletedDuel(null);
      setErrMsg(null);
    },
  });

  const submitMut = useMutation({
    mutationFn: (duelId: string) => {
      const payload: Record<number, Record<string, string>> = {};
      Object.entries(answers).forEach(([idx, fields]) => {
        const trimmed: Record<string, string> = {};
        Object.entries(fields).forEach(([key, val]) => {
          trimmed[key] = val.trim();
        });
        payload[Number(idx)] = trimmed;
      });
      return apiClient.submitDuel(duelId, payload, locale);
    },
    onSuccess: (done) => {
      setActiveDuel(null);
      setCompletedDuel(done);
      void qc.invalidateQueries({ queryKey: ["duels-lobby"] });
    },
  });

  // Result Summary Screen
  if (completedDuel) {
    const resultMsg =
      completedDuel.outcome === "WIN"
        ? t("duels.resultWin", { sp: completedDuel.rewardSp })
        : completedDuel.outcome === "DRAW"
          ? t("duels.resultDraw", { sp: completedDuel.rewardSp })
          : t("duels.resultLoss", { sp: completedDuel.rewardSp });

    return (
      <div className="mx-auto max-w-xl">
        <GalaxyCard className="space-y-5 text-center py-8">
          <Swords className="mx-auto h-10 w-10 text-amber-300" />
          <h1 className="text-2xl font-extrabold text-white">{resultMsg}</h1>
          <div className="grid grid-cols-2 gap-4">
            <div className="rounded-2xl bg-indigo-500/20 p-4">
              <p className="text-xs font-bold text-slate-300">{t("duels.yourScore")}</p>
              <p className="text-3xl font-extrabold text-white planetlogi-tabular">
                {completedDuel.playerScore} / 5
              </p>
            </div>
            <div className="rounded-2xl bg-cyan-500/20 p-4">
              <p className="text-xs font-bold text-slate-300">
                {t("duels.opponentScore", { name: completedDuel.opponent.maskedName })}
              </p>
              <p className="text-3xl font-extrabold text-white planetlogi-tabular">
                {completedDuel.opponentScore} / 5
              </p>
            </div>
          </div>
          <GalaxyButton onClick={() => setCompletedDuel(null)}>{t("duels.backToLobby")}</GalaxyButton>
        </GalaxyCard>
      </div>
    );
  }

  // Active 5-Question Duel Player
  if (activeDuel?.questions) {
    return (
      <div className="mx-auto max-w-3xl space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <p className="text-xs font-bold uppercase tracking-wider text-amber-300">
              {t("duels.vsLabel", { name: activeDuel.opponent.maskedName })}
            </p>
            <h1 className="text-2xl font-extrabold text-white">{t("duels.title")}</h1>
          </div>
          <GalaxyButton variant="secondary" onClick={() => setActiveDuel(null)}>
            {t("duels.backToLobby")}
          </GalaxyButton>
        </div>

        <div className="space-y-3">
          {activeDuel.questions.map((q, idx) => (
            <GalaxyCard key={q.id} className="space-y-3">
              <p className="text-xs font-bold uppercase tracking-wider text-cyan-300">
                {t("duels.questionProgress", { current: idx + 1, total: activeDuel.questions!.length })}
              </p>
              <p className="text-base sm:text-lg font-bold text-white">{q.stem}</p>
              {q.inputType === "mcq" && q.options ? (
                <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
                  {q.options.map((opt) => {
                    const selected = answers[idx]?.choice === opt.key;
                    return (
                      <button
                        key={opt.key}
                        type="button"
                        aria-pressed={selected}
                        onClick={() =>
                          setAnswers((a) => ({ ...a, [idx]: { ...a[idx], choice: opt.key } }))
                        }
                        className={`min-h-11 rounded-xl border px-3 py-2 text-left text-sm font-bold transition-colors cursor-pointer ${
                          selected
                            ? "border-[#6366F1] bg-[#6366F1]/30 text-white"
                            : "border-indigo-400/35 bg-[#0B1026] text-slate-200 hover:border-indigo-300"
                        }`}
                      >
                        <span className="mr-2 inline-flex h-6 w-6 items-center justify-center rounded-full bg-indigo-500/25 text-xs font-extrabold text-white">
                          {opt.key}
                        </span>
                        {opt.label}
                      </button>
                    );
                  })}
                </div>
              ) : (
                <div className="flex flex-wrap items-end gap-4">
                  {(q.parts ?? [{ key: "answer" }]).map((part) => (
                    <div key={part.key} className="space-y-1">
                      {part.label ? (
                        <label
                          htmlFor={`duel-ans-${idx}-${part.key}`}
                          className="block text-xs font-bold uppercase text-cyan-200"
                        >
                          {part.label}
                        </label>
                      ) : null}
                      <div className="flex items-center gap-2">
                        {part.prefix ? (
                          <span className="text-base font-bold text-slate-300">{part.prefix}</span>
                        ) : null}
                        <input
                          id={`duel-ans-${idx}-${part.key}`}
                          type="text"
                          inputMode={q.inputType === "fraction" ? "text" : "decimal"}
                          value={answers[idx]?.[part.key] ?? ""}
                          onChange={(e) =>
                            setAnswers((a) => ({ ...a, [idx]: { ...a[idx], [part.key]: e.target.value } }))
                          }
                          placeholder={q.inputType === "fraction" ? "e.g. 3/4" : "?"}
                          className="min-h-11 w-full sm:w-40 rounded-xl border border-indigo-400/35 bg-[#0B1026] px-4 text-base font-bold text-white"
                        />
                        {part.suffix ? (
                          <span className="text-base font-semibold text-slate-300">{part.suffix}</span>
                        ) : null}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </GalaxyCard>
          ))}
        </div>

        <GalaxyButton onClick={() => submitMut.mutate(activeDuel.id)} disabled={submitMut.isPending}>
          {t("duels.submitDuel")}
        </GalaxyButton>
      </div>
    );
  }

  // Duel Lobby
  const lobby = lobbyQuery.data;
  const energyLeft = lobby?.energyLeft ?? 3;
  const energyMax = lobby?.energyMax ?? 3;

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-3xl font-extrabold text-white">{t("duels.title")}</h1>
          <p className="mt-1 text-sm text-slate-300">{t("duels.subtitle")}</p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <GalaxyCard className="flex items-center gap-3 px-4 py-2.5 border-amber-400/35">
            <BatteryCharging className="h-6 w-6 text-amber-300" />
            <div>
              <p className="text-xs font-semibold text-slate-300">{t("duels.energyTitle")}</p>
              <div className="mt-1 flex items-center gap-1.5">
                {Array.from({ length: energyMax }).map((_, i) => (
                  <span
                    key={i}
                    className={`h-3.5 w-7 rounded-full ${
                      i < energyLeft ? "bg-emerald-400 shadow shadow-emerald-400/60" : "bg-slate-700"
                    }`}
                  />
                ))}
                <span className="ml-1 text-xs font-bold text-white planetlogi-tabular">
                  {t("duels.energyLeft", { left: energyLeft, max: energyMax })}
                </span>
              </div>
            </div>
          </GalaxyCard>

          <Link
            to="/$locale/star-crew"
            params={{ locale }}
            className="flex min-h-11 items-center gap-2 rounded-xl border border-cyan-400/35 bg-cyan-500/15 px-4 text-sm font-bold text-cyan-200 hover:bg-cyan-500/25"
          >
            <Users className="h-4 w-4" /> {t("duels.manageCrew")}
          </Link>
        </div>
      </div>

      {errMsg ? (
        <div className="rounded-xl border border-rose-400/40 bg-rose-500/15 px-4 py-3 text-sm font-semibold text-rose-200">
          {errMsg}
        </div>
      ) : null}

      {/* Lobby Tabs */}
      <div className="flex flex-wrap gap-2">
        {(
          [
            { id: "challenge", label: t("duels.tabs.challenge"), icon: Swords },
            { id: "incoming", label: t("duels.tabs.incoming", { count: lobby?.incoming.length ?? 0 }), icon: Inbox },
            { id: "history", label: t("duels.tabs.history"), icon: History },
            { id: "rules", label: t("duels.tabs.rules"), icon: ShieldCheck },
          ] as const
        ).map((item) => {
          const Icon = item.icon;
          return (
            <button
              key={item.id}
              type="button"
              onClick={() => setTab(item.id)}
              className={`flex min-h-11 items-center gap-2 rounded-xl px-4 text-sm font-bold cursor-pointer ${
                tab === item.id ? "bg-[#6366F1] text-white" : "bg-[#1E2A55] text-slate-200"
              }`}
            >
              <Icon className="h-4 w-4" />
              {item.label}
            </button>
          );
        })}
      </div>

      {tab === "challenge" ? (
        <div className="space-y-4">
          <div className="flex flex-wrap gap-2">
            {(["CLASSMATE", "CREW", "RANDOM"] as const).map((mode) => (
              <GalaxyButton
                key={mode}
                variant={mode === "RANDOM" ? "primary" : "secondary"}
                disabled={energyLeft <= 0 || startMut.isPending}
                onClick={() => startMut.mutate({ mode })}
              >
                ⚔️ {t(`duels.modes.${mode}`)}
              </GalaxyButton>
            ))}
          </div>

          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {(lobby?.opponents ?? []).map((opp) => (
              <GalaxyCard key={opp.id} className="flex items-center justify-between gap-3">
                <div className="flex items-center gap-3">
                  <AstroBuddyAvatar config={{ colorHex: opp.buddyColor, fullName: opp.buddyName }} size={56} />
                  <div>
                    <p className="font-bold text-white">{opp.maskedName}</p>
                    <p className="text-xs text-cyan-300">{opp.buddyName}</p>
                    <p className="text-xs text-slate-400">{opp.weeklySp} SP</p>
                  </div>
                </div>
                <GalaxyButton
                  disabled={energyLeft <= 0 || startMut.isPending}
                  onClick={() => startMut.mutate({ mode: opp.relation, opponentId: opp.id })}
                >
                  {t("duels.launchDuel")}
                </GalaxyButton>
              </GalaxyCard>
            ))}
          </div>
        </div>
      ) : null}

      {tab === "incoming" ? (
        <div className="space-y-3">
          {lobby?.incoming.length ? (
            lobby.incoming.map((inc) => (
              <GalaxyCard key={inc.id} className="flex flex-wrap items-center justify-between gap-4">
                <div>
                  <p className="font-bold text-white">{inc.opponent.maskedName}</p>
                  <p className="text-xs text-slate-300">{inc.opponent.buddyName}</p>
                </div>
                <GalaxyButton onClick={() => acceptMut.mutate(inc.id)} disabled={acceptMut.isPending}>
                  {t("duels.acceptDuel")}
                </GalaxyButton>
              </GalaxyCard>
            ))
          ) : (
            <GalaxyCard className="py-8 text-center text-slate-300">{t("duels.noIncoming")}</GalaxyCard>
          )}
        </div>
      ) : null}

      {tab === "history" ? (
        <div className="space-y-3">
          {lobby?.history.length ? (
            lobby.history.map((h) => (
              <GalaxyCard key={h.id} className="flex flex-wrap items-center justify-between gap-4">
                <div>
                  <p className="font-bold text-white">
                    {t("duels.vsLabel", { name: h.opponent.maskedName })} —{" "}
                    {t(`duels.outcome.${h.outcome}`)}
                  </p>
                  <p className="text-xs text-slate-300">
                    {h.playerScore} - {h.opponentScore}
                  </p>
                </div>
                <span className="rounded-lg bg-emerald-500/20 px-3 py-1 text-xs font-extrabold text-emerald-300">
                  +{h.rewardSp} SP
                </span>
              </GalaxyCard>
            ))
          ) : (
            <GalaxyCard className="py-8 text-center text-slate-300">{t("duels.noHistory")}</GalaxyCard>
          )}
        </div>
      ) : null}

      {tab === "rules" ? (
        <GalaxyCard className="space-y-2">
          {(lobby?.rules ?? []).map((rule, idx) => (
            <p key={idx} className="text-sm text-slate-200">
              • {rule}
            </p>
          ))}
        </GalaxyCard>
      ) : null}
    </div>
  );
}
