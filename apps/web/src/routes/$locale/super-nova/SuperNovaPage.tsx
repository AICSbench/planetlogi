import { useEffect, useRef, useState, type MouseEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Eye, Gem, History, Pause, Play, Zap } from "lucide-react";
import type { Locale } from "@planetlogi/i18n";
import type { SuperSightSession, SuperWarpStart } from "@planetlogi/api-client";
import { GalaxyButton, GalaxyCard } from "@planetlogi/ui";
import { apiClient } from "../../../store/authStore";

// Normalized (0..1) coordinates of the five difference targets, matching the
// server-side Super Sight puzzle layout.
const INTERACTIVE_TARGETS = [
  { id: 1, nx: 0.18, ny: 0.25, cx: 72, cy: 65 },
  { id: 2, nx: 0.5, ny: 0.22, cx: 200, cy: 57 },
  { id: 3, nx: 0.82, ny: 0.28, cx: 328, cy: 73 },
  { id: 4, nx: 0.3, ny: 0.72, cx: 120, cy: 187 },
  { id: 5, nx: 0.74, ny: 0.75, cx: 296, cy: 195 },
];

function CosmicSceneSVG({
  variant,
  foundIds,
  onTapNormalized,
  targetLabel,
}: {
  variant: "A" | "B";
  foundIds?: number[];
  onTapNormalized?: (x: number, y: number) => void;
  targetLabel?: (id: number) => string;
}) {
  function handleClick(e: MouseEvent<SVGSVGElement>) {
    if (!onTapNormalized) return;
    const rect = e.currentTarget.getBoundingClientRect();
    const nx = (e.clientX - rect.left) / rect.width;
    const ny = (e.clientY - rect.top) / rect.height;
    onTapNormalized(Math.max(0, Math.min(1, nx)), Math.max(0, Math.min(1, ny)));
  }

  const found = new Set(foundIds ?? []);

  // Keyboard-operable hotspots over the five difference targets so the Super
  // Sight canvas is fully usable without a pointer (CR-15). The dashed
  // outline is rendered only while a hotspot has keyboard focus, so it never
  // leaks the remaining target locations to sighted players.
  const keyboardTargets = INTERACTIVE_TARGETS.filter((tgt) => onTapNormalized && !found.has(tgt.id));

  return (
    <svg
      viewBox="0 0 400 260"
      onClick={handleClick}
      className={`w-full rounded-2xl border border-indigo-400/30 bg-[#0B1026] select-none ${
        onTapNormalized ? "cursor-crosshair" : ""
      }`}
    >
      {/* Background stars */}
      <circle cx="40" cy="30" r="2" fill="#E2E8F0" />
      <circle cx="140" cy="40" r="1.8" fill="#E2E8F0" />
      <circle cx="280" cy="35" r="2.2" fill="#E2E8F0" />
      <circle cx="360" cy="50" r="2" fill="#E2E8F0" />

      {/* Target 1 (0.18, 0.25 -> 72, 65): Ringed Planet color/moon diff */}
      <g>
        <circle cx="72" cy="65" r="22" fill={variant === "A" ? "#6366F1" : "#22D3EE"} />
        <ellipse cx="72" cy="65" rx="34" ry="8" fill="none" stroke="#FBBF24" strokeWidth="3" />
        {found.has(1) ? <circle cx="72" cy="65" r="28" fill="none" stroke="#10B981" strokeWidth="4" /> : null}
      </g>

      {/* Target 2 (0.50, 0.22 -> 200, 57): Star count / golden comet */}
      <g>
        <polygon
          points="200,36 206,52 222,52 209,62 214,77 200,67 186,77 191,62 178,52 194,52"
          fill={variant === "A" ? "#FBBF24" : "#EC4899"}
        />
        {found.has(2) ? <circle cx="200" cy="57" r="26" fill="none" stroke="#10B981" strokeWidth="4" /> : null}
      </g>

      {/* Target 3 (0.82, 0.28 -> 328, 73): Rocket window */}
      <g>
        <rect x="312" y="50" width="32" height="46" rx="14" fill="#E2E8F0" />
        <circle cx="328" cy="70" r="9" fill={variant === "A" ? "#3B82F6" : "#F97316"} />
        <polygon points="312,86 300,102 312,96" fill="#EF4444" />
        <polygon points="344,86 356,102 344,96" fill="#EF4444" />
        {found.has(3) ? <circle cx="328" cy="73" r="28" fill="none" stroke="#10B981" strokeWidth="4" /> : null}
      </g>

      {/* Moon terrain */}
      <path d="M0 210 Q100 170 200 205 T400 195 L400 260 L0 260 Z" fill="#1E2A55" />

      {/* Target 4 (0.30, 0.72 -> 120, 187): Observatory Dome Antenna */}
      <g>
        <path d="M95 205 A25 25 0 0 1 145 205 Z" fill="#A855F7" />
        {variant === "B" ? (
          <line x1="120" y1="180" x2="120" y2="155" stroke="#22D3EE" strokeWidth="4" strokeLinecap="round" />
        ) : null}
        {found.has(4) ? <circle cx="120" cy="187" r="28" fill="none" stroke="#10B981" strokeWidth="4" /> : null}
      </g>

      {/* Target 5 (0.74, 0.75 -> 296, 195): Rover Flag */}
      <g>
        <rect x="275" y="185" width="42" height="22" rx="6" fill="#10B981" />
        <circle cx="285" cy="210" r="7" fill="#94A3B8" />
        <circle cx="307" cy="210" r="7" fill="#94A3B8" />
        {variant === "A" ? (
          <polygon points="296,185 296,162 314,170 296,178" fill="#FBBF24" />
        ) : (
          <circle cx="296" cy="172" r="9" fill="#F43F5E" />
        )}
        {found.has(5) ? <circle cx="296" cy="195" r="28" fill="none" stroke="#10B981" strokeWidth="4" /> : null}
      </g>

      {/* Focus-only outline for the keyboard hotspots */}
      <style>{`
        .sight-hotspot:focus-visible {
          stroke: #22d3ee;
        }
      `}</style>
      {/* Keyboard hotspots: focusable overlays over the remaining targets */}
      {keyboardTargets.map((tgt) => (
        <rect
          key={tgt.id}
          className="sight-hotspot"
          x={tgt.cx - 32}
          y={tgt.cy - 22}
          width={64}
          height={44}
          rx={10}
          fill="transparent"
          strokeDasharray="5 5"
          strokeWidth="2"
          role="button"
          tabIndex={0}
          aria-label={targetLabel ? targetLabel(tgt.id) : `Target ${tgt.id}`}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault();
              onTapNormalized?.(tgt.nx, tgt.ny);
            }
          }}
        />
      ))}
    </svg>
  );
}

export function SuperNovaPage({ locale }: { locale: Locale }) {
  const { t } = useTranslation("stories");
  const qc = useQueryClient();

  const [isPractice, setIsPractice] = useState(false);
  const [sightSess, setSightSess] = useState<SuperSightSession | null>(null);
  const [sightFeedback, setSightFeedback] = useState<"hit" | "miss" | null>(null);
  const [sightTimeLeft, setSightTimeLeft] = useState(300);

  const [warpSess, setWarpSess] = useState<SuperWarpStart | null>(null);
  const [warpIdx, setWarpIdx] = useState(0);
  const [warpInput, setWarpInput] = useState("");
  const [warpAnswers, setWarpAnswers] = useState<Record<number, Record<string, string>>>({});
  const [warpGlobalSec, setWarpGlobalSec] = useState(120);
  const [warpQSec, setWarpQSec] = useState(10);
  const [pauseQueued, setPauseQueued] = useState(false);
  const [isPaused, setIsPaused] = useState(false);
  const [warpResult, setWarpResult] = useState<number | null>(null);
  const [hubPage, setHubPage] = useState(1);

  // Ref mirror so the timer effect can merge the current unsubmitted input
  // when the global deadline auto-finishes the session.
  const warpInputRef = useRef("");
  useEffect(() => {
    warpInputRef.current = warpInput;
  }, [warpInput]);

  const hubQuery = useQuery({
    queryKey: ["super-nova-hub", locale, hubPage],
    queryFn: () => apiClient.getSuperNovaHub(locale, hubPage),
  });

  const startSightMut = useMutation({
    mutationFn: () => apiClient.startSuperSight(isPractice, locale),
    onSuccess: (s) => {
      setSightSess(s);
      setSightTimeLeft(s.timeLimitSec);
      setSightFeedback(null);
    },
  });

  const tapSightMut = useMutation({
    mutationFn: (coords: { x: number; y: number }) =>
      apiClient.tapSuperSight(sightSess!.sessionId, coords.x, coords.y, locale),
    onSuccess: (res) => {
      setSightSess(res.session);
      setSightFeedback(res.hit ? "hit" : "miss");
      if (res.session.completed) {
        void qc.invalidateQueries({ queryKey: ["super-nova-hub"] });
      }
    },
  });

  const startWarpMut = useMutation({
    mutationFn: () => apiClient.startSuperWarp(isPractice, locale),
    onSuccess: (w) => {
      setWarpSess(w);
      setWarpIdx(0);
      setWarpInput("");
      setWarpAnswers({});
      setWarpGlobalSec(w.durationSec);
      setWarpQSec(w.autoSkipSec);
      setPauseQueued(false);
      setIsPaused(false);
      setWarpResult(null);
    },
  });

  const finishWarpMut = useMutation({
    mutationFn: (ans: Record<number, Record<string, string>>) =>
      apiClient.finishSuperWarp(warpSess!.sessionId, ans, locale),
    onSuccess: (res) => {
      setWarpResult(res.score);
      void qc.invalidateQueries({ queryKey: ["super-nova-hub"] });
    },
  });

  // Finish the warp, merging the current unsubmitted input so a student who
  // hits Finish (or runs out of time) still keeps their typed answer.
  function finishWarpNow() {
    if (!warpSess || warpResult !== null) return;
    const current = warpSess.questions[warpIdx];
    const typed = warpInputRef.current.trim();
    const merged =
      current && typed
        ? { ...warpAnswers, [current.index]: { answer: typed } }
        : warpAnswers;
    finishWarpMut.mutate(merged);
  }

  // Ref mirror so the timer interval can call the latest finish logic without
  // re-creating itself on every keystroke.
  const finishWarpNowRef = useRef<() => void>(() => {});
  useEffect(() => {
    finishWarpNowRef.current = finishWarpNow;
  });

  // Super Sight countdown
  useEffect(() => {
    if (!sightSess || sightSess.completed || sightTimeLeft <= 0) return;
    const id = window.setInterval(() => setSightTimeLeft((s) => Math.max(0, s - 1)), 1000);
    return () => window.clearInterval(id);
  }, [sightSess, sightTimeLeft]);

  // Super Warp dual timer (2-min global + 10s per question)
  useEffect(() => {
    if (!warpSess || warpResult !== null || isPaused) return;
    const id = window.setInterval(() => {
      setWarpGlobalSec((g) => {
        if (g <= 1) {
          finishWarpNowRef.current();
          return 0;
        }
        return g - 1;
      });
      setWarpQSec((q) => {
        if (q <= 1) {
          // Skip the expired question FIRST, then honor a queued pause, so the
          // resumed session grants the fresh auto-skip timer to the NEXT
          // question — never a full timer to the already-expired one.
          setWarpIdx((curr) => {
            const next = curr + 1;
            if (next >= warpSess.questions.length) {
              finishWarpNowRef.current();
              return curr;
            }
            if (pauseQueued) {
              setIsPaused(true);
              setPauseQueued(false);
            }
            setWarpInput("");
            return next;
          });
          return warpSess.autoSkipSec;
        }
        return q - 1;
      });
    }, 1000);
    return () => window.clearInterval(id);
    // Deps keep only primitives + the stable session object: the mutable
    // answers and the mutation object are reached through finishWarpNowRef,
    // so submitting cannot tear down and re-create the interval (drift).
  }, [warpSess, warpResult, isPaused, pauseQueued]);

  function submitWarpQuestion() {
    if (!warpSess) return;
    const current = warpSess.questions[warpIdx];
    if (!current) return;
    const nextAnswers = { ...warpAnswers, [current.index]: { answer: warpInput.trim() } };
    setWarpAnswers(nextAnswers);
    setWarpInput("");
    if (warpIdx + 1 >= warpSess.questions.length) {
      finishWarpMut.mutate(nextAnswers);
      return;
    }
    if (pauseQueued) {
      setIsPaused(true);
      setPauseQueued(false);
    }
    setWarpIdx(warpIdx + 1);
    setWarpQSec(warpSess.autoSkipSec);
  }

  // Active Super Sight Screen
  if (sightSess) {
    return (
      <div className="space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 className="text-2xl font-extrabold text-white">{sightSess.title}</h1>
            <p className="text-sm text-cyan-200">{t("superNova.sightTapHint")}</p>
          </div>
          <div className="flex items-center gap-3">
            <span className="rounded-xl bg-emerald-500/20 px-3 py-2 text-sm font-bold text-emerald-200">
              {t("superNova.sightFound", { found: sightSess.foundCount })}
            </span>
            <span className="rounded-xl bg-rose-500/20 px-3 py-2 text-sm font-bold text-rose-200">
              {t("superNova.sightWrong", { wrong: sightSess.wrongCount, max: sightSess.maxWrong })}
            </span>
            <span className="rounded-xl bg-indigo-500/20 px-3 py-2 text-sm font-bold text-white planetlogi-tabular">
              {Math.floor(sightTimeLeft / 60)}:{String(sightTimeLeft % 60).padStart(2, "0")}
            </span>
          </div>
        </div>

        {sightFeedback ? (
          <div
            className={`rounded-xl px-4 py-2 text-sm font-semibold ${
              sightFeedback === "hit"
                ? "bg-emerald-500/20 text-emerald-200"
                : "bg-amber-500/20 text-amber-200"
            }`}
          >
            {sightFeedback === "hit" ? t("superNova.sightHit") : t("superNova.sightMiss")}
          </div>
        ) : null}

        <div className="grid gap-4 md:grid-cols-2">
          <GalaxyCard className="space-y-2">
            <p className="text-xs font-bold uppercase tracking-wider text-slate-300">
              {t("superNova.sightSceneA")}
            </p>
            <CosmicSceneSVG variant="A" />
          </GalaxyCard>
          <GalaxyCard className="space-y-2">
            <p className="text-xs font-bold uppercase tracking-wider text-cyan-300">
              {t("superNova.sightSceneB")}
            </p>
            <CosmicSceneSVG
              variant="B"
              foundIds={sightSess.foundPoints.map((p) => p.id)}
              targetLabel={(id) => t("superNova.sightTarget", { n: id })}
              onTapNormalized={(x, y) => {
                if (!sightSess.completed && !tapSightMut.isPending) {
                  tapSightMut.mutate({ x, y });
                }
              }}
            />
          </GalaxyCard>
        </div>

        {sightSess.completed ? (
          <GalaxyCard className="flex flex-wrap items-center justify-between gap-4 border-emerald-400/40 bg-emerald-500/10">
            <div>
              <h2 className="text-xl font-extrabold text-emerald-200">{t("superNova.sightComplete")}</h2>
              <p className="text-sm text-white">+{sightSess.score} S</p>
            </div>
            <GalaxyButton onClick={() => setSightSess(null)}>{t("superNova.backToHub")}</GalaxyButton>
          </GalaxyCard>
        ) : (
          <GalaxyButton variant="secondary" onClick={() => setSightSess(null)}>
            {t("superNova.backToHub")}
          </GalaxyButton>
        )}
      </div>
    );
  }

  // Active Super Warp Screen
  if (warpSess) {
    const activeQ = warpSess.questions[warpIdx];
    return (
      <div className="mx-auto max-w-2xl space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h1 className="text-2xl font-extrabold text-white">{t("superNova.warpTitle")}</h1>
          <div className="flex items-center gap-2">
            <span className="rounded-xl bg-indigo-500/25 px-3 py-1.5 text-sm font-bold text-white planetlogi-tabular">
              {t("superNova.warpTimeLeft")}: {Math.floor(warpGlobalSec / 60)}:
              {String(warpGlobalSec % 60).padStart(2, "0")}
            </span>
            <span className="rounded-xl bg-amber-500/25 px-3 py-1.5 text-sm font-bold text-amber-200 planetlogi-tabular">
              {t("superNova.warpQuestionTimer")}: {warpQSec}s
            </span>
          </div>
        </div>

        {warpResult !== null ? (
          <GalaxyCard className="space-y-4 text-center py-8">
            <h2 className="text-2xl font-extrabold text-emerald-300">{t("superNova.warpResultTitle")}</h2>
            <p className="text-lg font-bold text-white">
              {t("superNova.warpResultScore", { score: warpResult })}
            </p>
            <GalaxyButton onClick={() => setWarpSess(null)}>{t("superNova.backToHub")}</GalaxyButton>
          </GalaxyCard>
        ) : isPaused ? (
          <GalaxyCard className="space-y-4 text-center py-8">
            <h2 className="text-xl font-extrabold text-cyan-200">{t("superNova.warpPausedTitle")}</h2>
            <p className="text-sm text-slate-300">{t("superNova.warpPausedBody")}</p>
            <GalaxyButton onClick={() => setIsPaused(false)}>
              <Play className="h-4 w-4 mr-1" /> {t("superNova.warpResume")}
            </GalaxyButton>
          </GalaxyCard>
        ) : activeQ ? (
          <GalaxyCard className="space-y-4">
            <p className="text-xs font-bold uppercase tracking-wider text-cyan-300">
              #{activeQ.index} / {warpSess.questions.length}
            </p>
            <p className="text-xl font-bold text-white">{activeQ.question.stem}</p>
            <form
              onSubmit={(e) => {
                e.preventDefault();
                submitWarpQuestion();
              }}
              className="flex gap-2"
            >
              <input
                type="text"
                value={warpInput}
                onChange={(e) => setWarpInput(e.target.value)}
                aria-label={t("superNova.answerLabel")}
                autoFocus
                className="min-h-12 flex-1 rounded-xl border border-indigo-400/35 bg-[#0B1026] px-4 text-lg font-bold text-white"
              />
              <GalaxyButton type="submit">{t("superNova.submitAnswer")}</GalaxyButton>
            </form>
            <div className="flex flex-wrap items-center justify-between gap-2 pt-2">
              <button
                type="button"
                onClick={() => setPauseQueued((p) => !p)}
                className="flex min-h-11 items-center gap-2 rounded-xl border border-indigo-400/30 bg-white/5 px-3 text-sm font-semibold text-indigo-200 hover:bg-white/10 cursor-pointer"
              >
                <Pause className="h-4 w-4" />
                {pauseQueued ? t("superNova.warpPauseQueued") : t("superNova.warpPauseAfter")}
              </button>
              <GalaxyButton variant="secondary" onClick={finishWarpNow}>
                {t("superNova.warpFinish")}
              </GalaxyButton>
            </div>
          </GalaxyCard>
        ) : null}
      </div>
    );
  }

  // Main Super Nova Hub
  const hub = hubQuery.data;
  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-3xl font-extrabold text-white">{t("superNova.title")}</h1>
          <p className="mt-1 text-sm text-slate-300">{t("superNova.subtitle")}</p>
          <p className="mt-1 text-xs text-cyan-300">{t("superNova.openHours")}</p>
        </div>
        <GalaxyCard className="flex items-center gap-3 px-5 py-3 border-cyan-400/35">
          <Gem className="h-7 w-7 text-[#22D3EE]" />
          <div>
            <p className="text-xs font-semibold text-slate-300">{t("superNova.weeklyDiamond")}</p>
            <p className="text-2xl font-extrabold text-white planetlogi-tabular">
              {hub?.weeklyDiamondScore ?? 0} S
            </p>
          </div>
        </GalaxyCard>
      </div>

      {/* Practice vs Challenge Switcher */}
      <div className="flex flex-wrap items-center gap-3">
        <button
          type="button"
          onClick={() => setIsPractice(false)}
          className={`min-h-11 rounded-xl px-4 text-sm font-bold cursor-pointer ${
            !isPractice ? "bg-[#6366F1] text-white" : "bg-white/10 text-slate-300"
          }`}
        >
          🏆 {t("superNova.challengeMode")}
        </button>
        <button
          type="button"
          onClick={() => setIsPractice(true)}
          className={`min-h-11 rounded-xl px-4 text-sm font-bold cursor-pointer ${
            isPractice ? "bg-emerald-600 text-white" : "bg-white/10 text-slate-300"
          }`}
        >
          🧪 {t("superNova.practiceMode")}
        </button>
        {isPractice ? <span className="text-xs text-emerald-300">{t("superNova.practiceHint")}</span> : null}
      </div>

      {/* Two Game Mode Cards */}
      <div className="grid gap-4 md:grid-cols-2">
        <GalaxyCard className="space-y-4">
          <div className="flex items-center gap-3">
            <span className="flex h-12 w-12 items-center justify-center rounded-2xl bg-cyan-500/20 text-cyan-300">
              <Eye className="h-6 w-6" />
            </span>
            <div>
              <h2 className="text-xl font-extrabold text-white">{t("superNova.sightTitle")}</h2>
              <p className="text-sm text-slate-300">{t("superNova.sightDesc")}</p>
            </div>
          </div>
          <GalaxyButton onClick={() => startSightMut.mutate()} disabled={startSightMut.isPending}>
            {t("superNova.sightStart")}
          </GalaxyButton>
        </GalaxyCard>

        <GalaxyCard className="space-y-4">
          <div className="flex items-center gap-3">
            <span className="flex h-12 w-12 items-center justify-center rounded-2xl bg-amber-500/20 text-amber-300">
              <Zap className="h-6 w-6" />
            </span>
            <div>
              <h2 className="text-xl font-extrabold text-white">{t("superNova.warpTitle")}</h2>
              <p className="text-sm text-slate-300">{t("superNova.warpDesc")}</p>
            </div>
          </div>
          <GalaxyButton onClick={() => startWarpMut.mutate()} disabled={startWarpMut.isPending}>
            {t("superNova.warpStart")}
          </GalaxyButton>
        </GalaxyCard>
      </div>

      {/* Score History */}
      <GalaxyCard className="space-y-3">
        <div className="flex items-center gap-2">
          <History className="h-5 w-5 text-indigo-300" />
          <h2 className="text-lg font-bold text-white">{t("superNova.historyTitle")}</h2>
        </div>
        {hub?.history?.length ? (
          <>
            <div className="divide-y divide-indigo-400/15">
              {hub.history.map((h) => (
                <div key={h.id} className="flex items-center justify-between py-2.5 text-sm">
                  <span className="font-semibold text-white">
                    {h.mode === "SUPER_SIGHT" ? t("superNova.sightTitle") : t("superNova.warpTitle")}{" "}
                    {h.isPractice ? `(${t("superNova.practiceMode")})` : ""}
                  </span>
                  <span className="font-bold text-cyan-300 planetlogi-tabular">+{h.score} S</span>
                </div>
              ))}
            </div>
            {hub.historyPages && hub.historyPages > 1 ? (
              <div className="flex flex-wrap items-center justify-between gap-2 pt-1">
                <GalaxyButton
                  variant="secondary"
                  disabled={(hub.historyPage ?? 1) >= hub.historyPages}
                  onClick={() => setHubPage((p) => p + 1)}
                >
                  {t("superNova.historyOlder")}
                </GalaxyButton>
                <span className="text-xs font-semibold text-slate-400 planetlogi-tabular">
                  {t("superNova.historyPageLabel", {
                    page: hub.historyPage ?? 1,
                    total: hub.historyPages,
                  })}
                </span>
                <GalaxyButton
                  variant="secondary"
                  disabled={(hub.historyPage ?? 1) <= 1}
                  onClick={() => setHubPage((p) => Math.max(1, p - 1))}
                >
                  {t("superNova.historyNewer")}
                </GalaxyButton>
              </div>
            ) : null}
          </>
        ) : (
          <p className="text-sm text-slate-400">{t("superNova.historyEmpty")}</p>
        )}
      </GalaxyCard>
    </div>
  );
}
