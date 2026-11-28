import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { ShieldCheck } from "lucide-react";
import type { Locale } from "@planetlogi/i18n";
import { GalaxyButton, GalaxyCard } from "@planetlogi/ui";
import { setAppLocale } from "../../../i18n/init";
import { apiClient, useAuthStore } from "../../../store/authStore";

export function ProfileSettingsPage({ locale }: { locale: Locale }) {
  const { t } = useTranslation(["portals", "common"]);
  const queryClient = useQueryClient();
  const isParent = useAuthStore((s) => s.user?.role === "PARENT");

  const bundleQuery = useQuery({
    queryKey: ["profile-bundle"],
    queryFn: () => apiClient.getProfileBundle(),
  });

  const bundle = bundleQuery.data;
  const [otpDraft, setOtpDraft] = useState<boolean | null>(null);
  const [langDraft, setLangDraft] = useState<"en" | "id" | null>(null);
  const [brainCapDraft, setBrainCapDraft] = useState<number | null>(null);
  const [savedToast, setSavedToast] = useState(false);

  const otpEnabled = otpDraft ?? bundle?.settings.otpEnabled ?? false;
  const lang = langDraft ?? bundle?.settings.language ?? locale;
  const brainCap = brainCapDraft ?? bundle?.settings.brainGalaxyDailyCapMin ?? 25;

  const updateMutation = useMutation({
    mutationFn: () =>
      apiClient.updateProfileSettings({
        otpEnabled,
        language: lang,
        brainGalaxyDailyCapMin: brainCap,
      }),
    onSuccess: async (updated) => {
      queryClient.setQueryData(["profile-bundle"], updated);
      setOtpDraft(null);
      setLangDraft(null);
      setBrainCapDraft(null);
      await setAppLocale(updated.settings.language);
      setSavedToast(true);
    },
  });

  return (
    <div className="mx-auto max-w-4xl space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-extrabold text-white">⚙️ {t("portals:profile.title")}</h1>
        <Link to="/$locale/parent" params={{ locale }}>
          <GalaxyButton variant="gold">{t("portals:profile.openParentPortal")} →</GalaxyButton>
        </Link>
      </div>

      {bundle ? (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          <GalaxyCard className="space-y-2">
            <h2 className="text-sm font-bold uppercase text-cyan-300">
              {t("portals:profile.parentCard")}
            </h2>
            <p className="text-lg font-extrabold text-white">{bundle.parent.name}</p>
            <p className="text-xs text-slate-300">{bundle.parent.maskedEmail}</p>
            <p className="text-xs text-slate-300">{bundle.parent.maskedPhone}</p>
          </GalaxyCard>

          <GalaxyCard className="space-y-2">
            <h2 className="text-sm font-bold uppercase text-amber-300">
              {t("portals:profile.childCard")}
            </h2>
            <p className="text-lg font-extrabold text-white">
              {bundle.child.firstName} • {t("common:common.grade", { grade: bundle.child.grade })}
            </p>
            <p className="text-xs text-slate-300">
              {t("portals:profile.schoolLabel")}: {bundle.child.schoolName}
            </p>
            <p className="rounded-lg bg-[#1E2A55] px-2.5 py-1.5 text-xs text-cyan-200">
              {t("portals:profile.promotionNote", { date: bundle.child.nextPromotionDate })}
            </p>
          </GalaxyCard>
        </div>
      ) : null}

      <GalaxyCard className="space-y-4">
        <div className="flex items-center gap-2">
          <ShieldCheck className="h-5 w-5 text-[#22D3EE]" aria-hidden="true" />
          <h2 className="text-lg font-extrabold text-white">
            {t("portals:profile.settingsCard")}
          </h2>
        </div>

        <label className="flex min-h-12 items-start gap-3 rounded-xl border border-indigo-400/20 bg-[#1E2A55]/70 p-3 cursor-pointer">
          <input
            type="checkbox"
            checked={otpEnabled}
            onChange={(e) => setOtpDraft(e.target.checked)}
            className="mt-1"
          />
          <div>
            <p className="text-sm font-bold text-white">{t("portals:profile.otpToggle")}</p>
            <p className="text-xs text-slate-300">{t("portals:profile.otpHelp")}</p>
          </div>
        </label>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <label className="block text-xs font-bold text-slate-200">
              {t("common:language.label")}
            </label>
            <select
              value={lang}
              onChange={(e) => setLangDraft(e.target.value as "en" | "id")}
              className="mt-1 min-h-11 w-full rounded-xl border border-indigo-400/30 bg-[#0B1026] px-3 text-sm text-white"
            >
              <option value="en">{t("common:language.en")}</option>
              <option value="id">{t("common:language.id")}</option>
            </select>
          </div>

          <div>
            <label className="block text-xs font-bold text-slate-200">
              {t("portals:profile.brainCapLabel")}:{" "}
              <strong className="text-cyan-200">{brainCap} min</strong>
            </label>
            <input
              type="range"
              min={0}
              max={25}
              value={brainCap}
              onChange={(e) => setBrainCapDraft(Number(e.target.value))}
              className="mt-3 w-full"
            />
          </div>
        </div>

        {!isParent ? (
          <p className="text-sm font-semibold text-amber-200">{t("portals:parentOnlyNotice")}</p>
        ) : null}

        {savedToast ? (
          <p className="text-sm font-bold text-emerald-300">✓ {t("portals:profile.savedToast")}</p>
        ) : null}

        <GalaxyButton
          onClick={() => updateMutation.mutate()}
          disabled={!isParent || updateMutation.isPending}
        >
          {t("portals:profile.saveSettings")}
        </GalaxyButton>
      </GalaxyCard>

      {/* Subscription Placeholder (Feature-First: 100% Unlocked during Phases 0-4) */}
      <GalaxyCard className="space-y-1.5 border-emerald-400/30">
        <h2 className="text-base font-extrabold text-white">
          {t("portals:profile.subscriptionCard")}
        </h2>
        <p className="text-sm font-semibold text-emerald-300">
          {bundle?.settings.subscriptionPlanName ?? "PlanetLogi Galaxy All-Access"}
        </p>
        <p className="text-xs text-slate-300">{t("portals:profile.featureFirstNotice")}</p>
      </GalaxyCard>
    </div>
  );
}
