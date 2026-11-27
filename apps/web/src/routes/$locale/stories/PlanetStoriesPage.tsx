import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { BookOpen, ChevronLeft, ChevronRight, Lock, Volume2, VolumeX } from "lucide-react";
import type { Locale } from "@planetlogi/i18n";
import type { StoryChapterViewer } from "@planetlogi/api-client";
import { GalaxyButton, GalaxyCard } from "@planetlogi/ui";
import { apiClient, useAuthStore } from "../../../store/authStore";

export function PlanetStoriesPage({ locale }: { locale: Locale }) {
  const { t } = useTranslation("stories");
  const qc = useQueryClient();
  const user = useAuthStore((s) => s.user);
  const [selectedGrade, setSelectedGrade] = useState<number>(user?.grade ?? 1);
  const [viewer, setViewer] = useState<StoryChapterViewer | null>(null);
  const [pageIdx, setPageIdx] = useState(0);
  const [ttsEnabled, setTtsEnabled] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const planetsQuery = useQuery({
    queryKey: ["story-planets", locale],
    queryFn: () => apiClient.getStoryPlanets(locale),
  });

  const openMut = useMutation({
    mutationFn: (code: string) => apiClient.openStoryChapter(code, locale),
    onSuccess: (v) => {
      setViewer(v);
      setPageIdx(Math.max(0, (v.lastPageRead || 1) - 1));
      setErrorMsg(null);
    },
  });

  const unlockMut = useMutation({
    mutationFn: (code: string) => apiClient.unlockStoryChapter(code, locale),
    onSuccess: (v) => {
      setViewer(v);
      setPageIdx(0);
      setErrorMsg(null);
      void qc.invalidateQueries({ queryKey: ["story-planets"] });
    },
    onError: () => setErrorMsg(t("stories.unlockError")),
  });

  const bookmarkMut = useMutation({
    mutationFn: (vars: { code: string; page: number }) =>
      apiClient.bookmarkStoryPage(vars.code, vars.page),
  });

  function goToPage(nextIdx: number) {
    if (!viewer) return;
    const bounded = Math.max(0, Math.min(viewer.pages.length - 1, nextIdx));
    setPageIdx(bounded);
    bookmarkMut.mutate({ code: viewer.chapterCode, page: bounded + 1 });
  }

  // Keyboard navigation in storybook viewer
  useEffect(() => {
    if (!viewer) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "ArrowRight") goToPage(pageIdx + 1);
      if (e.key === "ArrowLeft") goToPage(pageIdx - 1);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  // Web Speech TTS narration when enabled
  useEffect(() => {
    if (!viewer || !ttsEnabled || typeof window === "undefined" || !("speechSynthesis" in window)) {
      return;
    }
    const text = viewer.pages[pageIdx]?.text;
    if (!text) return;
    window.speechSynthesis.cancel();
    const utter = new SpeechSynthesisUtterance(text);
    utter.lang = locale === "id" ? "id-ID" : "en-US";
    utter.rate = 0.95;
    window.speechSynthesis.speak(utter);
    return () => window.speechSynthesis.cancel();
  }, [viewer, pageIdx, ttsEnabled, locale]);

  // 20-Page Digital Storybook Viewer
  if (viewer) {
    const activePage = viewer.pages[pageIdx] ?? viewer.pages[0];
    return (
      <div className="mx-auto max-w-4xl space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <p className="text-xs font-bold uppercase tracking-wider text-cyan-300">{viewer.planetName}</p>
            <h1 className="text-2xl font-extrabold text-white">{viewer.title}</h1>
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => setTtsEnabled((v) => !v)}
              className={`flex min-h-11 items-center gap-2 rounded-xl px-3 text-sm font-bold cursor-pointer ${
                ttsEnabled ? "bg-cyan-500 text-slate-950" : "bg-white/10 text-slate-200"
              }`}
            >
              {ttsEnabled ? <Volume2 className="h-4 w-4" /> : <VolumeX className="h-4 w-4" />}
              {ttsEnabled ? t("stories.ttsOn") : t("stories.ttsOff")}
            </button>
            <GalaxyButton variant="secondary" onClick={() => setViewer(null)}>
              {t("stories.backToWorlds")}
            </GalaxyButton>
          </div>
        </div>

        {/* Illustrated Scene + Dynamic Bilingual Overlay */}
        <GalaxyCard className="overflow-hidden p-0">
          <div
            className="relative flex min-h-72 flex-col justify-between p-6 sm:p-10"
            style={{
              background: `radial-gradient(circle at 25% 25%, ${viewer.accentColor}44, #0B1026 75%)`,
            }}
          >
            {/* Decorative cosmic vector art (text-free so i18n text overlays crisply) */}
            <svg
              viewBox="0 0 600 180"
              className="w-full h-40 opacity-90"
              aria-hidden="true"
            >
              <circle cx={90 + ((pageIdx * 23) % 420)} cy="65" r="34" fill={viewer.accentColor} fillOpacity="0.85" />
              <ellipse
                cx={90 + ((pageIdx * 23) % 420)}
                cy="65"
                rx="55"
                ry="12"
                fill="none"
                stroke="#FBBF24"
                strokeWidth="3"
              />
              <circle cx="520" cy="40" r="14" fill="#22D3EE" fillOpacity="0.7" />
              <path d="M0 165 Q150 120 300 155 T600 140 L600 180 L0 180 Z" fill="#131B3A" />
            </svg>

            <div className="mt-4 rounded-2xl border border-indigo-400/30 bg-[#0B1026]/85 p-5 backdrop-blur">
              <p className="text-xs font-bold uppercase tracking-wider text-amber-300">
                {t("stories.pageOf", { current: pageIdx + 1, total: viewer.pages.length })}
              </p>
              <p className="mt-2 text-lg sm:text-xl font-semibold leading-relaxed text-white">
                {activePage?.text}
              </p>
            </div>
          </div>

          {/* Controls */}
          <div className="flex items-center justify-between border-t border-indigo-400/20 bg-[#131B3A] p-4">
            <GalaxyButton
              variant="secondary"
              disabled={pageIdx <= 0}
              onClick={() => goToPage(pageIdx - 1)}
            >
              <ChevronLeft className="h-4 w-4 mr-1" />
              {t("stories.prevPage")}
            </GalaxyButton>
            <span className="text-sm font-bold text-slate-300 planetlogi-tabular">
              {pageIdx + 1} / {viewer.pages.length}
            </span>
            <GalaxyButton
              disabled={pageIdx >= viewer.pages.length - 1}
              onClick={() => goToPage(pageIdx + 1)}
            >
              {t("stories.nextPage")}
              <ChevronRight className="h-4 w-4 ml-1" />
            </GalaxyButton>
          </div>
        </GalaxyCard>

        {/* Page Thumbnail Strip */}
        <div className="space-y-2">
          <p className="text-xs font-bold uppercase tracking-wider text-slate-400">{t("stories.thumbnails")}</p>
          <div className="flex gap-2 overflow-x-auto pb-2">
            {viewer.pages.map((p, idx) => (
              <button
                key={p.page}
                type="button"
                onClick={() => goToPage(idx)}
                className={`min-h-11 min-w-11 shrink-0 rounded-xl text-xs font-extrabold cursor-pointer ${
                  idx === pageIdx
                    ? "bg-[#6366F1] text-white ring-2 ring-cyan-300"
                    : "bg-[#1E2A55] text-slate-300 hover:bg-[#2A3A73]"
                }`}
              >
                {p.page}
              </button>
            ))}
          </div>
        </div>
      </div>
    );
  }

  // Planet Selector & Chapter Timeline
  const planets = planetsQuery.data?.planets ?? [];
  const activePlanet = planets.find((p) => p.grade === selectedGrade) ?? planets[0];

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-extrabold text-white">{t("stories.title")}</h1>
        <p className="mt-1 text-sm text-slate-300">{t("stories.subtitle")}</p>
      </div>

      {/* 6 Grade Planets Selector */}
      <div className="flex flex-wrap gap-2">
        {[1, 2, 3, 4, 5, 6].map((g) => (
          <button
            key={g}
            type="button"
            onClick={() => setSelectedGrade(g)}
            className={`min-h-11 rounded-xl px-4 text-sm font-bold cursor-pointer ${
              selectedGrade === g
                ? "bg-[#6366F1] text-white shadow shadow-indigo-500/30"
                : "bg-[#1E2A55] text-slate-200 hover:bg-[#2A3A73]"
            }`}
          >
            🪐 {t("stories.gradeWorld", { grade: g })}
          </button>
        ))}
      </div>

      {errorMsg ? (
        <div className="rounded-xl border border-rose-400/40 bg-rose-500/15 px-4 py-3 text-sm font-semibold text-rose-200">
          {errorMsg}
        </div>
      ) : null}

      {activePlanet ? (
        <GalaxyCard className="space-y-4">
          <div className="flex items-center gap-3">
            <span
              className="h-10 w-10 rounded-full shadow-lg"
              style={{ backgroundColor: activePlanet.accentColor }}
            />
            <div>
              <h2 className="text-2xl font-extrabold text-white">{activePlanet.name}</h2>
              <p className="text-xs font-semibold uppercase tracking-wider text-slate-400">
                {t("stories.chapterTimeline")}
              </p>
            </div>
          </div>

          <div className="grid gap-4 md:grid-cols-3">
            {activePlanet.chapters.map((ch) => (
              <div
                key={ch.code}
                className="flex flex-col justify-between rounded-2xl border border-indigo-400/25 bg-[#0B1026]/80 p-4 space-y-4"
              >
                <div className="space-y-2">
                  <div className="flex items-center justify-between">
                    <span className="rounded-lg bg-indigo-500/25 px-2.5 py-1 text-xs font-bold text-indigo-200">
                      #{ch.index}
                    </span>
                    {ch.unlockCostAc === 0 ? (
                      <span className="rounded-lg bg-emerald-500/20 px-2.5 py-1 text-xs font-bold text-emerald-300">
                        {t("stories.freeBadge")}
                      </span>
                    ) : ch.unlocked ? (
                      <span className="rounded-lg bg-cyan-500/20 px-2.5 py-1 text-xs font-bold text-cyan-200">
                        {t("stories.unlockedBadge")}
                      </span>
                    ) : (
                      <span className="flex items-center gap-1 rounded-lg bg-amber-500/20 px-2.5 py-1 text-xs font-bold text-amber-200">
                        <Lock className="h-3 w-3" /> {ch.unlockCostAc} AC
                      </span>
                    )}
                  </div>
                  <h3 className="text-lg font-bold text-white">{ch.title}</h3>
                </div>

                {ch.unlocked ? (
                  <GalaxyButton onClick={() => openMut.mutate(ch.code)} disabled={openMut.isPending}>
                    <BookOpen className="h-4 w-4 mr-1.5" />
                    {ch.lastPageRead > 1
                      ? t("stories.continuePage", { page: ch.lastPageRead, total: ch.pageCount })
                      : t("stories.readAction")}
                  </GalaxyButton>
                ) : (
                  <GalaxyButton
                    variant="secondary"
                    onClick={() => unlockMut.mutate(ch.code)}
                    disabled={unlockMut.isPending}
                  >
                    {t("stories.unlockAction", { cost: ch.unlockCostAc })}
                  </GalaxyButton>
                )}
              </div>
            ))}
          </div>
        </GalaxyCard>
      ) : null}
    </div>
  );
}
