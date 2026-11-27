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
import { useAuthStore } from "./store/authStore";

const FEATURE_META: Record<string, { titleKey: string; phase: number }> = {
  "daily-orbit": { titleKey: "apps.dailyOrbit", phase: 1 },
  streak: { titleKey: "nav.streak", phase: 1 },
  training: { titleKey: "apps.galaxyTraining", phase: 1 },
  report: { titleKey: "nav.report", phase: 1 },
  academy: { titleKey: "apps.spaceAcademy", phase: 2 },
  "astro-quiz": { titleKey: "apps.astroQuiz", phase: 2 },
  assignments: { titleKey: "apps.assignments", phase: 2 },
  "profile-settings": { titleKey: "nav.settings", phase: 2 },
  help: { titleKey: "nav.help", phase: 2 },
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
  const meta = FEATURE_META[slug] ?? { titleKey: "brand", phase: 1 };

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

const featureSplatRoute = createRoute({
  getParentRoute: () => appShellRoute,
  path: "/$",
  component: FeaturePlaceholderRoute,
});

const routeTree = rootRoute.addChildren([
  indexRoute,
  localeRoute.addChildren([
    loginRoute,
    appShellRoute.addChildren([dashboardRoute, featureSplatRoute]),
  ]),
]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
