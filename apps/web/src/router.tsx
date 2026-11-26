import { useEffect } from "react";
import {
  Outlet,
  createRootRoute,
  createRoute,
  createRouter,
  redirect,
  useParams,
} from "@tanstack/react-router";
import { normalizeLocale, type Locale } from "@planetlogi/i18n";
import { GalaxyCard } from "@planetlogi/ui";
import { useTranslation } from "react-i18next";
import { detectInitialLocale, setAppLocale } from "./i18n/init";
import { TopNavShell } from "./components/shell/TopNavShell";
import { LoginPage } from "./routes/$locale/(auth)/login/LoginPage";
import { DashboardPreviewPage } from "./routes/$locale/dashboard/DashboardPreviewPage";
import { DailyOrbitPage } from "./routes/$locale/daily-orbit/DailyOrbitPage";
import { StreakPage } from "./routes/$locale/streak/StreakPage";
import { TrainingPage } from "./routes/$locale/training/TrainingPage";
import { ReportPage } from "./routes/$locale/report/ReportPage";
import { ExpLevelPage } from "./routes/$locale/exp-level/ExpLevelPage";
import { SpaceAcademyPage } from "./routes/$locale/academy/SpaceAcademyPage";
import { AstroQuizPage } from "./routes/$locale/astro-quiz/AstroQuizPage";
import { AstroQuizPrintPage } from "./routes/$locale/astro-quiz/AstroQuizPrintPage";
import { AssignmentsPage } from "./routes/$locale/assignments/AssignmentsPage";
import { ProfileSettingsPage } from "./routes/$locale/profile-settings/ProfileSettingsPage";
import { ParentPortalPage } from "./routes/$locale/parent/ParentPortalPage";
import { HelpPage } from "./routes/$locale/help/HelpPage";
import { useAuthStore } from "./store/authStore";

const FEATURE_META: Record<string, { titleKey: string; phase: number }> = {
  "super-nova": { titleKey: "apps.dailyOrbit", phase: 3 },
  stories: { titleKey: "apps.planetStories", phase: 3 },
  "star-duels": { titleKey: "apps.starDuels", phase: 3 },
  leaderboard: { titleKey: "nav.leaderboard", phase: 3 },
  achievement: { titleKey: "nav.achievement", phase: 3 },
  events: { titleKey: "nav.events", phase: 3 },
  "brain-galaxy": { titleKey: "apps.brainGalaxy", phase: 4 },
};

function RootLayout() {
  const hydrate = useAuthStore((s) => s.hydrate);
  useEffect(() => {
    void hydrate();
  }, [hydrate]);
  return <Outlet />;
}

function LocaleLayout() {
  const params = useParams({ from: "/$locale" });
  const locale: Locale = normalizeLocale(params.locale);

  useEffect(() => {
    void setAppLocale(locale);
  }, [locale]);

  return <Outlet />;
}

function ShellLayout() {
  const params = useParams({ from: "/$locale" });
  const locale: Locale = normalizeLocale(params.locale);
  return (
    <TopNavShell locale={locale}>
      <Outlet />
    </TopNavShell>
  );
}

function FeaturePlaceholderRoute() {
  const { _splat } = useParams({ from: "/$locale/shell/$" });
  const { t } = useTranslation("common");
  const slug = (_splat ?? "").split("/")[0] ?? "";
  const meta = FEATURE_META[slug] ?? { titleKey: "brand", phase: 3 };

  return (
    <GalaxyCard className="mx-auto max-w-2xl text-center py-12 space-y-2">
      <h1 className="text-2xl font-extrabold text-white">{t(meta.titleKey)}</h1>
      <p className="text-sm text-slate-300">{t("common.comingInPhase", { phase: meta.phase })}</p>
    </GalaxyCard>
  );
}

const rootRoute = createRootRoute({ component: RootLayout });

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  beforeLoad: () => {
    const locale = detectInitialLocale();
    throw redirect({ to: "/$locale/login", params: { locale } });
  },
});

const localeRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/$locale",
  component: LocaleLayout,
});

const loginRoute = createRoute({
  getParentRoute: () => localeRoute,
  path: "/login",
  component: function LoginRouteView() {
    const { locale } = useParams({ from: "/$locale/login" });
    return <LoginPage locale={normalizeLocale(locale)} />;
  },
});

// Dedicated chromeless A4 Print Worksheet Route (no dark Galaxy navbar!)
const astroQuizPrintRoute = createRoute({
  getParentRoute: () => localeRoute,
  path: "/astro-quiz/print/$paperCode",
  component: function AstroQuizPrintView() {
    const { locale, paperCode } = useParams({ from: "/$locale/astro-quiz/print/$paperCode" });
    return <AstroQuizPrintPage locale={normalizeLocale(locale)} paperCode={paperCode} />;
  },
});

const appShellRoute = createRoute({
  getParentRoute: () => localeRoute,
  id: "shell",
  component: ShellLayout,
});

const dashboardRoute = createRoute({
  getParentRoute: () => appShellRoute,
  path: "/dashboard",
  component: function DashboardRouteView() {
    const { locale } = useParams({ from: "/$locale/shell/dashboard" });
    return <DashboardPreviewPage locale={normalizeLocale(locale)} />;
  },
});

const dailyOrbitRoute = createRoute({
  getParentRoute: () => appShellRoute,
  path: "/daily-orbit",
  component: function DailyOrbitRouteView() {
    const { locale } = useParams({ from: "/$locale/shell/daily-orbit" });
    return <DailyOrbitPage locale={normalizeLocale(locale)} />;
  },
});

const streakRoute = createRoute({
  getParentRoute: () => appShellRoute,
  path: "/streak",
  component: StreakPage,
});

const trainingRoute = createRoute({
  getParentRoute: () => appShellRoute,
  path: "/training",
  component: function TrainingRouteView() {
    const { locale } = useParams({ from: "/$locale/shell/training" });
    return <TrainingPage locale={normalizeLocale(locale)} />;
  },
});

const reportRoute = createRoute({
  getParentRoute: () => appShellRoute,
  path: "/report",
  component: function ReportRouteView() {
    const { locale } = useParams({ from: "/$locale/shell/report" });
    return <ReportPage locale={normalizeLocale(locale)} />;
  },
});

const expLevelRoute = createRoute({
  getParentRoute: () => appShellRoute,
  path: "/exp-level",
  component: function ExpLevelRouteView() {
    const { locale } = useParams({ from: "/$locale/shell/exp-level" });
    return <ExpLevelPage locale={normalizeLocale(locale)} />;
  },
});

const academyRoute = createRoute({
  getParentRoute: () => appShellRoute,
  path: "/academy",
  component: function AcademyRouteView() {
    const { locale } = useParams({ from: "/$locale/shell/academy" });
    return <SpaceAcademyPage locale={normalizeLocale(locale)} />;
  },
});

const astroQuizRoute = createRoute({
  getParentRoute: () => appShellRoute,
  path: "/astro-quiz",
  component: function AstroQuizRouteView() {
    const { locale } = useParams({ from: "/$locale/shell/astro-quiz" });
    return <AstroQuizPage locale={normalizeLocale(locale)} />;
  },
});

const assignmentsRoute = createRoute({
  getParentRoute: () => appShellRoute,
  path: "/assignments",
  component: function AssignmentsRouteView() {
    const { locale } = useParams({ from: "/$locale/shell/assignments" });
    return <AssignmentsPage locale={normalizeLocale(locale)} />;
  },
});

const profileSettingsRoute = createRoute({
  getParentRoute: () => appShellRoute,
  path: "/profile-settings",
  component: function ProfileSettingsRouteView() {
    const { locale } = useParams({ from: "/$locale/shell/profile-settings" });
    return <ProfileSettingsPage locale={normalizeLocale(locale)} />;
  },
});

const parentPortalRoute = createRoute({
  getParentRoute: () => appShellRoute,
  path: "/parent",
  component: function ParentPortalRouteView() {
    const { locale } = useParams({ from: "/$locale/shell/parent" });
    return <ParentPortalPage locale={normalizeLocale(locale)} />;
  },
});

const helpRoute = createRoute({
  getParentRoute: () => appShellRoute,
  path: "/help",
  component: HelpPage,
});

const featureSplatRoute = createRoute({
  getParentRoute: () => appShellRoute,
  path: "/$",
  component: FeaturePlaceholderRoute,
});

const routeTree = rootRoute.addChildren([
  indexRoute,
  localeRoute.addChildren([
    loginRoute,
    astroQuizPrintRoute,
    appShellRoute.addChildren([
      dashboardRoute,
      dailyOrbitRoute,
      streakRoute,
      trainingRoute,
      reportRoute,
      expLevelRoute,
      academyRoute,
      astroQuizRoute,
      assignmentsRoute,
      profileSettingsRoute,
      parentPortalRoute,
      helpRoute,
      featureSplatRoute,
    ]),
  ]),
]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
