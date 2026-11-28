import { Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { Locale } from "@planetlogi/i18n";
import { GalaxyButton, GalaxyCard, ProficiencyPill } from "@planetlogi/ui";
import { apiClient, useAuthStore } from "../../../store/authStore";

export function ParentPortalPage({ locale }: { locale: Locale }) {
  const { t } = useTranslation(["portals", "common", "dashboard"]);

  // The overview API is parent-only; don't fire a request that can only 403 for students.
  const isParent = useAuthStore((s) => s.user?.role === "PARENT");

  const overviewQuery = useQuery({
    queryKey: ["parent-overview", locale],
    queryFn: () => apiClient.getParentOverview(locale),
    enabled: isParent,
  });

  const ov = overviewQuery.data;

  return (
    <div className="mx-auto max-w-5xl space-y-6">
      <GalaxyCard className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-extrabold text-white">🛰️ {t("portals:parentPortal.title")}</h1>
          <p className="text-sm text-slate-300">{t("portals:parentPortal.subtitle")}</p>
        </div>
        <Link to="/$locale/assignments" params={{ locale }}>
          <GalaxyButton variant="gold">{t("portals:parentPortal.quickAssignFocus")} →</GalaxyButton>
        </Link>
      </GalaxyCard>

      {!isParent ? (
        <GalaxyCard>
          <p className="text-sm font-semibold text-amber-200">{t("portals:parentOnlyNotice")}</p>
        </GalaxyCard>
      ) : overviewQuery.isError ? (
        <GalaxyCard className="space-y-3 text-center">
          <p role="alert" className="text-sm font-bold text-red-300">
            {t("portals:parentPortal.loadError")}
          </p>
          <GalaxyButton variant="secondary" onClick={() => void overviewQuery.refetch()}>
            {t("common:common.retry")}
          </GalaxyButton>
        </GalaxyCard>
      ) : overviewQuery.isPending ? (
        <GalaxyCard className="text-center text-slate-300">{t("common:common.loading")}</GalaxyCard>
      ) : null}

      {ov ? (
        <>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <GalaxyCard className="text-center">
              <p className="text-2xl font-extrabold text-orange-300">🔥 {ov.streakDays}d</p>
              <p className="text-xs text-slate-300">{t("common:nav.streak")}</p>
            </GalaxyCard>
            <GalaxyCard className="text-center">
              <p className="text-2xl font-extrabold text-cyan-300">❄️ {ov.cryoShields}/3</p>
              <p className="text-xs text-slate-300">{t("dashboard:wallet.cryoShields")}</p>
            </GalaxyCard>
            <GalaxyCard className="text-center">
              <p className="text-2xl font-extrabold text-emerald-300">{ov.masteryCount}</p>
              <p className="text-xs text-slate-300">{t("common:proficiency.mastery")}</p>
            </GalaxyCard>
            <GalaxyCard className="text-center">
              <p className="text-2xl font-extrabold text-amber-300">{ov.beginningCount}</p>
              <p className="text-xs text-slate-300">{t("common:proficiency.beginning")}</p>
            </GalaxyCard>
          </div>

          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            <GalaxyCard className="space-y-3">
              <h2 className="text-base font-extrabold text-emerald-300">
                {t("portals:parentPortal.topStrengths")}
              </h2>
              {ov.strengthSkills.length === 0 ? (
                <p className="text-xs text-slate-400">
                  {t("portals:parentPortal.noStrengths")}
                </p>
              ) : (
                ov.strengthSkills.map((sk) => (
                  <div key={sk.code} className="flex items-center justify-between text-sm">
                    <span className="font-semibold text-white">{sk.name}</span>
                    <ProficiencyPill
                      tier={sk.proficiencyTier}
                      label={t(`common:proficiency.${sk.proficiencyTier.toLowerCase()}`)}
                      pct={sk.proficiencyPct}
                    />
                  </div>
                ))
              )}
            </GalaxyCard>

            <GalaxyCard className="space-y-3">
              <h2 className="text-base font-extrabold text-amber-300">
                {t("portals:parentPortal.focusAreas")}
              </h2>
              {ov.focusSkills.map((sk) => (
                <div key={sk.code} className="flex items-center justify-between text-sm">
                  <span className="font-semibold text-white">{sk.name}</span>
                  <ProficiencyPill
                    tier={sk.proficiencyTier}
                    label={t(`common:proficiency.${sk.proficiencyTier.toLowerCase()}`)}
                    pct={sk.proficiencyPct}
                  />
                </div>
              ))}
            </GalaxyCard>
          </div>
        </>
      ) : null}
    </div>
  );
}
