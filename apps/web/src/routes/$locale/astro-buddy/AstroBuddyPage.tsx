import { useEffect, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Check, Sparkles } from "lucide-react";
import type { Locale } from "@planetlogi/i18n";
import type { AstroBuddyConfig } from "@planetlogi/api-client";
import { GalaxyButton, GalaxyCard } from "@planetlogi/ui";
import { AstroBuddyAvatar } from "../../../components/astro-buddy/AstroBuddyAvatar";
import { apiClient, useAuthStore } from "../../../store/authStore";

const BODY_SHAPES = ["round", "star", "pear", "cube"] as const;
const HORNS_STYLES = ["antennae", "cat_ears", "comet_horns", "halo"] as const;
const EYES_STYLES = ["bright", "starry", "cyclops", "visor"] as const;
const MOUTH_STYLES = ["smile", "grin", "cat", "whistle"] as const;

export function AstroBuddyPage({ locale }: { locale: Locale }) {
  const { t } = useTranslation("stories");
  const user = useAuthStore((s) => s.user);
  const firstName = (user?.displayName ?? "Raka").split(" ")[0] ?? "Raka";

  const optionsQuery = useQuery({
    queryKey: ["customization-options", locale],
    queryFn: () => apiClient.getCustomizationOptions(locale),
  });

  const [draft, setDraft] = useState<AstroBuddyConfig>({
    bodyShape: "round",
    colorHex: "#6366F1",
    hornsStyle: "antennae",
    eyesStyle: "bright",
    mouthStyle: "smile",
    adjective: "Starry",
    fullName: `Starry ${firstName}`,
    avatarPackId: "space_pets",
    avatarItemId: "astro_fox",
  });
  const [savedToast, setSavedToast] = useState(false);

  useEffect(() => {
    if (optionsQuery.data?.buddy) {
      setDraft(optionsQuery.data.buddy);
    }
  }, [optionsQuery.data]);

  const saveMut = useMutation({
    mutationFn: (cfg: AstroBuddyConfig) => apiClient.saveAstroBuddy(cfg),
    onSuccess: (saved) => {
      setDraft(saved);
      setSavedToast(true);
      window.setTimeout(() => setSavedToast(false), 3000);
    },
  });

  const colors = optionsQuery.data?.colors ?? ["#6366F1", "#22D3EE", "#10B981", "#FBBF24", "#EC4899"];
  const adjectives = optionsQuery.data?.adjectives ?? ["Starry", "Cosmic", "Turbo", "Nova"];
  const packs = optionsQuery.data?.avatarPacks ?? [];

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-extrabold text-white">{t("customization.buddyTitle")}</h1>
        <p className="mt-1 text-sm text-slate-300">{t("customization.buddySubtitle")}</p>
      </div>

      <div className="grid gap-6 lg:grid-cols-3">
        {/* Live Preview */}
        <GalaxyCard className="flex flex-col items-center justify-center space-y-4 py-8 text-center">
          <p className="text-xs font-bold uppercase tracking-wider text-cyan-300">
            {t("customization.livePreview")}
          </p>
          <AstroBuddyAvatar config={draft} size={180} />
          <h2 className="text-2xl font-extrabold text-white">{draft.adjective} {firstName}</h2>
          <GalaxyButton
            onClick={() =>
              saveMut.mutate({ ...draft, fullName: `${draft.adjective} ${firstName}` })
            }
            disabled={saveMut.isPending}
          >
            <Sparkles className="h-4 w-4 mr-1.5" />
            {t("customization.saveButton")}
          </GalaxyButton>
          {savedToast ? (
            <p className="flex items-center gap-1 text-sm font-bold text-emerald-300">
              <Check className="h-4 w-4" /> {t("customization.savedToast")}
            </p>
          ) : null}
        </GalaxyCard>

        {/* Controls */}
        <GalaxyCard className="lg:col-span-2 space-y-5">
          {/* Body Shape */}
          <div>
            <p className="mb-2 text-xs font-bold uppercase tracking-wider text-slate-300">
              {t("customization.bodyShape")}
            </p>
            <div className="flex flex-wrap gap-2">
              {BODY_SHAPES.map((s) => (
                <button
                  key={s}
                  type="button"
                  onClick={() => setDraft((d) => ({ ...d, bodyShape: s }))}
                  className={`min-h-11 rounded-xl px-4 text-sm font-bold cursor-pointer ${
                    draft.bodyShape === s ? "bg-[#6366F1] text-white" : "bg-white/10 text-slate-200"
                  }`}
                >
                  {t(`customization.shapes.${s}`)}
                </button>
              ))}
            </div>
          </div>

          {/* 15 Cosmic Colors */}
          <div>
            <p className="mb-2 text-xs font-bold uppercase tracking-wider text-slate-300">
              {t("customization.cosmicColor")}
            </p>
            <div className="flex flex-wrap gap-2">
              {colors.map((hex) => (
                <button
                  key={hex}
                  type="button"
                  aria-label={hex}
                  onClick={() => setDraft((d) => ({ ...d, colorHex: hex }))}
                  className={`h-11 w-11 rounded-full cursor-pointer transition-transform ${
                    draft.colorHex === hex ? "scale-110 ring-4 ring-white" : "opacity-85 hover:opacity-100"
                  }`}
                  style={{ backgroundColor: hex }}
                />
              ))}
            </div>
          </div>

          {/* Horns / Ears */}
          <div>
            <p className="mb-2 text-xs font-bold uppercase tracking-wider text-slate-300">
              {t("customization.hornsStyle")}
            </p>
            <div className="flex flex-wrap gap-2">
              {HORNS_STYLES.map((h) => (
                <button
                  key={h}
                  type="button"
                  onClick={() => setDraft((d) => ({ ...d, hornsStyle: h }))}
                  className={`min-h-11 rounded-xl px-4 text-sm font-bold cursor-pointer ${
                    draft.hornsStyle === h ? "bg-[#6366F1] text-white" : "bg-white/10 text-slate-200"
                  }`}
                >
                  {t(`customization.horns.${h}`)}
                </button>
              ))}
            </div>
          </div>

          {/* Eyes & Mouth */}
          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <p className="mb-2 text-xs font-bold uppercase tracking-wider text-slate-300">
                {t("customization.eyesStyle")}
              </p>
              <div className="flex flex-wrap gap-2">
                {EYES_STYLES.map((e) => (
                  <button
                    key={e}
                    type="button"
                    onClick={() => setDraft((d) => ({ ...d, eyesStyle: e }))}
                    className={`min-h-11 rounded-xl px-3 text-xs font-bold cursor-pointer ${
                      draft.eyesStyle === e ? "bg-cyan-500 text-slate-950" : "bg-white/10 text-slate-200"
                    }`}
                  >
                    {t(`customization.eyes.${e}`)}
                  </button>
                ))}
              </div>
            </div>
            <div>
              <p className="mb-2 text-xs font-bold uppercase tracking-wider text-slate-300">
                {t("customization.mouthStyle")}
              </p>
              <div className="flex flex-wrap gap-2">
                {MOUTH_STYLES.map((m) => (
                  <button
                    key={m}
                    type="button"
                    onClick={() => setDraft((d) => ({ ...d, mouthStyle: m }))}
                    className={`min-h-11 rounded-xl px-3 text-xs font-bold cursor-pointer ${
                      draft.mouthStyle === m ? "bg-cyan-500 text-slate-950" : "bg-white/10 text-slate-200"
                    }`}
                  >
                    {t(`customization.mouths.${m}`)}
                  </button>
                ))}
              </div>
            </div>
          </div>

          {/* Safe 2-Part Cosmic Name Selector */}
          <div>
            <label className="block mb-2 text-xs font-bold uppercase tracking-wider text-slate-300">
              {t("customization.nameGenerator")} — {t("customization.adjectiveLabel")}
            </label>
            <select
              value={draft.adjective}
              onChange={(e) => setDraft((d) => ({ ...d, adjective: e.target.value }))}
              className="min-h-11 rounded-xl border border-indigo-400/35 bg-[#0B1026] px-4 text-sm font-bold text-white"
            >
              {adjectives.map((adj) => (
                <option key={adj} value={adj}>
                  {adj} {firstName}
                </option>
              ))}
            </select>
          </div>
        </GalaxyCard>
      </div>

      {/* 8 Theme Avatar Packs */}
      <GalaxyCard className="space-y-4">
        <h2 className="text-xl font-extrabold text-white">{t("customization.avatarPacks")}</h2>
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {packs.map((pack) => (
            <div key={pack.id} className="rounded-2xl border border-indigo-400/20 bg-[#0B1026]/80 p-3 space-y-2">
              <p className="text-sm font-bold text-cyan-200">{pack.name}</p>
              <div className="flex flex-wrap gap-1.5">
                {pack.items.map((item) => (
                  <button
                    key={item}
                    type="button"
                    onClick={() => setDraft((d) => ({ ...d, avatarPackId: pack.id, avatarItemId: item }))}
                    className={`min-h-10 rounded-lg px-2.5 text-xs font-semibold cursor-pointer ${
                      draft.avatarItemId === item
                        ? "bg-[#6366F1] text-white"
                        : "bg-white/10 text-slate-300 hover:bg-white/15"
                    }`}
                  >
                    {item.replace(/_/g, " ")}
                  </button>
                ))}
              </div>
            </div>
          ))}
        </div>
      </GalaxyCard>
    </div>
  );
}
