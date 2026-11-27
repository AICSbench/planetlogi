import { useEffect, useState, type ReactNode } from "react";
import { Link, useNavigate, useRouterState } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import {
  Bell,
  ChevronDown,
  Flame,
  Gift,
  Globe,
  Grid3x3,
  MoreVertical,
  Rocket,
  WifiOff,
} from "lucide-react";
import type { Locale } from "@planetlogi/i18n";
import { setAppLocale } from "../../i18n/init";
import { useAuthStore } from "../../store/authStore";

interface TopNavShellProps {
  locale: Locale;
  children: ReactNode;
}

const LAUNCHER_APPS = [
  { key: "spaceAcademy", path: "academy", icon: "🚀" },
  { key: "starDuels", path: "star-duels", icon: "⚔️" },
  { key: "dailyOrbit", path: "daily-orbit", icon: "🪐" },
  { key: "sundayMeteor", path: "events", icon: "☄️" },
  { key: "astroQuiz", path: "astro-quiz", icon: "📝" },
  { key: "assignments", path: "assignments", icon: "📋" },
  { key: "brainGalaxy", path: "brain-galaxy", icon: "🧠" },
  { key: "planetStories", path: "stories", icon: "📖" },
  { key: "galaxyTraining", path: "training", icon: "🌌" },
] as const;

export function TopNavShell({ locale, children }: TopNavShellProps) {
  const { t } = useTranslation("common");
  const navigate = useNavigate();
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const { user, logout } = useAuthStore();

  const [openMenu, setOpenMenu] = useState<
    "subject" | "notif" | "launcher" | "account" | "overflow" | null
  >(null);
  const [isOffline, setIsOffline] = useState<boolean>(
    typeof navigator !== "undefined" ? !navigator.onLine : false
  );

  useEffect(() => {
    const onOnline = () => setIsOffline(false);
    const onOffline = () => setIsOffline(true);
    window.addEventListener("online", onOnline);
    window.addEventListener("offline", onOffline);
    return () => {
      window.removeEventListener("online", onOnline);
      window.removeEventListener("offline", onOffline);
    };
  }, []);

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpenMenu(null);
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  const primaryLinks = [
    { label: t("nav.home"), slug: "dashboard" },
    { label: t("nav.report"), slug: "report" },
    { label: t("nav.leaderboard"), slug: "leaderboard" },
    { label: t("nav.achievement"), slug: "achievement" },
    { label: t("nav.events"), slug: "events" },
  ] as const;

  async function handleLocaleSwitch(nextLocale: Locale) {
    await setAppLocale(nextLocale);
    setOpenMenu(null);
    const segments = pathname.split("/").filter(Boolean);
    const rest = segments.slice(1).join("/");
    if (!rest || rest === "dashboard") {
      void navigate({ to: "/$locale/dashboard", params: { locale: nextLocale } });
    } else if (rest === "login") {
      void navigate({ to: "/$locale/login", params: { locale: nextLocale } });
    } else {
      void navigate({ to: "/$locale/$", params: { locale: nextLocale, _splat: rest } });
    }
  }

  async function handleLogout() {
    setOpenMenu(null);
    await logout();
    void navigate({ to: "/$locale/login", params: { locale } });
  }

  return (
    <div className="min-h-screen planetlogi-galaxy-bg text-slate-100 flex flex-col">
      {/* Persistent Responsive Top Navigation Bar (docs/02 §2.1) */}
      <header className="sticky top-0 z-40 border-b border-indigo-400/20 bg-[#131B3A]/95 backdrop-blur">
        <div className="mx-auto flex h-16 max-w-[1440px] items-center justify-between gap-2 px-3 sm:px-5">
          {/* Left Cluster: Logo + Subject Switcher */}
          <div className="flex items-center gap-2 sm:gap-3">
            <Link
              to="/$locale/dashboard"
              params={{ locale }}
              className="flex min-h-11 items-center gap-2 rounded-xl px-2 font-extrabold tracking-tight text-white hover:bg-white/5"
            >
              <span className="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-[#22D3EE] to-[#6366F1] text-white shadow">
                <Rocket className="h-5 w-5" aria-hidden="true" />
              </span>
              <span className="text-lg">{t("brand")}</span>
            </Link>

            <div className="relative">
              <button
                type="button"
                aria-expanded={openMenu === "subject"}
                onClick={() => setOpenMenu(openMenu === "subject" ? null : "subject")}
                className="flex min-h-11 items-center gap-1.5 rounded-xl border border-indigo-400/25 bg-[#1E2A55] px-3 py-1.5 text-sm font-semibold text-indigo-100 hover:bg-[#2A3A73] cursor-pointer"
              >
                <span>{t("nav.subjectMath")}</span>
                <ChevronDown className="h-4 w-4" aria-hidden="true" />
              </button>
              {openMenu === "subject" ? (
                <div
                  role="menu"
                  className="absolute left-0 mt-2 w-56 rounded-2xl border border-indigo-400/25 bg-[#131B3A] p-2 shadow-2xl"
                >
                  <button
                    type="button"
                    onClick={() => setOpenMenu(null)}
                    className="flex w-full min-h-11 items-center justify-between rounded-xl bg-[#6366F1]/25 px-3 text-sm font-semibold text-white"
                  >
                    <span>🪐 {t("nav.subjectMath")}</span>
                    <span className="text-xs text-[#22D3EE]">{t("nav.subjectActive")}</span>
                  </button>
                  <div className="mt-1 flex min-h-11 items-center justify-between rounded-xl px-3 text-sm text-slate-400">
                    <span>🔬 {t("nav.subjectScience")}</span>
                    <span className="text-xs">{t("nav.subjectComingSoon")}</span>
                  </div>
                  <div className="flex min-h-11 items-center justify-between rounded-xl px-3 text-sm text-slate-400">
                    <span>📚 {t("nav.subjectEnglish")}</span>
                    <span className="text-xs">{t("nav.subjectComingSoon")}</span>
                  </div>
                </div>
              ) : null}
            </div>
          </div>

          {/* Primary Navigation Links (visible on tablet/desktop) */}
          <nav aria-label="Primary" className="hidden lg:flex items-center gap-1">
            {primaryLinks.map((item) => {
              const targetPath = `/${locale}/${item.slug}`;
              const active = pathname === targetPath;
              const cls = `flex min-h-11 items-center rounded-xl px-3.5 text-sm font-semibold transition-colors ${
                active
                  ? "bg-[#6366F1] text-white shadow shadow-indigo-500/30"
                  : "text-slate-200 hover:bg-white/10"
              }`;
              return item.slug === "dashboard" ? (
                <Link key={item.slug} to="/$locale/dashboard" params={{ locale }} className={cls}>
                  {item.label}
                </Link>
              ) : (
                <Link
                  key={item.slug}
                  to="/$locale/$"
                  params={{ locale, _splat: item.slug }}
                  className={cls}
                >
                  {item.label}
                </Link>
              );
            })}
          </nav>

          {/* Right Utility Cluster */}
          <div className="flex items-center gap-1 sm:gap-2">
            {/* Streak Counter */}
            <Link
              to="/$locale/$"
              params={{ locale, _splat: "streak" }}
              title={t("nav.streak")}
              className="flex min-h-11 items-center gap-1 rounded-xl border border-orange-400/30 bg-orange-500/15 px-2.5 text-sm font-bold text-orange-300 hover:bg-orange-500/25 planetlogi-tabular"
            >
              <Flame className="h-4 w-4 text-[#F97316]" aria-hidden="true" />
              <span>{user ? (user.currentStreak ?? 0) : 3}</span>
            </Link>

            {/* Desktop Utility Icons (>=1440px full expansion, compact on smaller) */}
            <button
              type="button"
              title={t("nav.bonusGift")}
              aria-label={t("nav.bonusGift")}
              className="hidden 2xl:flex min-h-11 min-w-11 items-center justify-center rounded-xl bg-purple-500/20 text-purple-300 hover:bg-purple-500/30 cursor-pointer"
            >
              <Gift className="h-5 w-5" aria-hidden="true" />
            </button>

            {/* Notifications */}
            <div className="relative hidden sm:block">
              <button
                type="button"
                aria-label={t("nav.notifications")}
                aria-expanded={openMenu === "notif"}
                onClick={() => setOpenMenu(openMenu === "notif" ? null : "notif")}
                className="flex min-h-11 min-w-11 items-center justify-center rounded-xl hover:bg-white/10 text-slate-200 cursor-pointer"
              >
                <Bell className="h-5 w-5" aria-hidden="true" />
              </button>
              {openMenu === "notif" ? (
                <div className="absolute right-0 mt-2 w-72 rounded-2xl border border-indigo-400/25 bg-[#131B3A] p-4 shadow-2xl">
                  <p className="text-sm font-bold text-white">{t("nav.notifications")}</p>
                  <p className="mt-2 text-sm text-slate-400">{t("nav.noNotifications")}</p>
                </div>
              ) : null}
            </div>

            {/* 9-Dot App Launcher */}
            <div className="relative">
              <button
                type="button"
                aria-label={t("nav.appLauncher")}
                aria-expanded={openMenu === "launcher"}
                onClick={() => setOpenMenu(openMenu === "launcher" ? null : "launcher")}
                className="flex min-h-11 min-w-11 items-center justify-center rounded-xl hover:bg-white/10 text-slate-200 cursor-pointer"
              >
                <Grid3x3 className="h-5 w-5" aria-hidden="true" />
              </button>
              {openMenu === "launcher" ? (
                <div className="absolute right-0 mt-2 w-72 sm:w-80 rounded-2xl border border-indigo-400/25 bg-[#131B3A] p-3 shadow-2xl">
                  <p className="px-2 pb-2 text-xs font-bold uppercase tracking-wider text-slate-400">
                    {t("nav.appLauncher")}
                  </p>
                  <div className="grid grid-cols-3 gap-2">
                    {LAUNCHER_APPS.map((app) => (
                      <Link
                        key={app.key}
                        to="/$locale/$"
                        params={{ locale, _splat: app.path }}
                        onClick={() => setOpenMenu(null)}
                        className="flex min-h-16 flex-col items-center justify-center gap-1 rounded-xl bg-[#1E2A55]/80 p-2 text-center text-xs font-semibold text-slate-100 hover:bg-[#6366F1]/40"
                      >
                        <span className="text-xl" aria-hidden="true">
                          {app.icon}
                        </span>
                        <span className="leading-tight">{t(`apps.${app.key}`)}</span>
                      </Link>
                    ))}
                  </div>
                </div>
              ) : null}
            </div>

            {/* Language Quick Switcher (EN / ID) */}
            <button
              type="button"
              onClick={() => handleLocaleSwitch(locale === "en" ? "id" : "en")}
              aria-label={t("language.label")}
              className="hidden md:flex min-h-11 items-center gap-1 rounded-xl border border-indigo-400/25 px-2.5 text-xs font-bold uppercase text-cyan-200 hover:bg-white/10 cursor-pointer"
            >
              <Globe className="h-4 w-4" aria-hidden="true" />
              <span>{locale.toUpperCase()}</span>
            </button>

            {/* Account Dropdown */}
            <div className="relative">
              <button
                type="button"
                aria-expanded={openMenu === "account"}
                onClick={() => setOpenMenu(openMenu === "account" ? null : "account")}
                className="flex min-h-11 items-center gap-1.5 rounded-xl border border-indigo-400/25 bg-[#1E2A55] px-2.5 text-sm font-semibold text-white hover:bg-[#2A3A73] cursor-pointer"
              >
                <span className="flex h-7 w-7 items-center justify-center rounded-full bg-[#6366F1] text-xs font-bold">
                  {(user?.displayName ?? "P").slice(0, 1).toUpperCase()}
                </span>
                <ChevronDown className="h-4 w-4" aria-hidden="true" />
              </button>
              {openMenu === "account" ? (
                <div className="absolute right-0 mt-2 w-56 rounded-2xl border border-indigo-400/25 bg-[#131B3A] p-2 shadow-2xl">
                  {[
                    { label: t("nav.editAvatar"), slug: "profile-settings" },
                    { label: t("nav.editProfile"), slug: "profile-settings" },
                    { label: t("nav.settings"), slug: "profile-settings" },
                    { label: t("nav.mySubscription"), slug: "profile-settings" },
                    { label: t("nav.help"), slug: "help" },
                  ].map((item) => (
                    <Link
                      key={item.label}
                      to="/$locale/$"
                      params={{ locale, _splat: item.slug }}
                      onClick={() => setOpenMenu(null)}
                      className="flex min-h-11 items-center rounded-xl px-3 text-sm text-slate-200 hover:bg-white/10"
                    >
                      {item.label}
                    </Link>
                  ))}
                  <hr className="my-1 border-indigo-400/20" />
                  <button
                    type="button"
                    onClick={handleLogout}
                    className="flex w-full min-h-11 items-center rounded-xl px-3 text-left text-sm font-semibold text-red-300 hover:bg-red-500/15 cursor-pointer"
                  >
                    {t("nav.logout")}
                  </button>
                </div>
              ) : null}
            </div>

            {/* 3-Dot Overflow Menu on <1440px viewports */}
            <div className="relative 2xl:hidden">
              <button
                type="button"
                aria-label={t("nav.moreMenu")}
                aria-expanded={openMenu === "overflow"}
                onClick={() => setOpenMenu(openMenu === "overflow" ? null : "overflow")}
                className="flex min-h-11 min-w-11 items-center justify-center rounded-xl hover:bg-white/10 text-slate-200 cursor-pointer"
              >
                <MoreVertical className="h-5 w-5" aria-hidden="true" />
              </button>
              {openMenu === "overflow" ? (
                <div className="absolute right-0 mt-2 w-60 rounded-2xl border border-indigo-400/25 bg-[#131B3A] p-2 shadow-2xl">
                  <div className="lg:hidden">
                    {primaryLinks.map((item) =>
                      item.slug === "dashboard" ? (
                        <Link
                          key={item.slug}
                          to="/$locale/dashboard"
                          params={{ locale }}
                          onClick={() => setOpenMenu(null)}
                          className="flex min-h-11 items-center rounded-xl px-3 text-sm font-semibold text-slate-100 hover:bg-white/10"
                        >
                          {item.label}
                        </Link>
                      ) : (
                        <Link
                          key={item.slug}
                          to="/$locale/$"
                          params={{ locale, _splat: item.slug }}
                          onClick={() => setOpenMenu(null)}
                          className="flex min-h-11 items-center rounded-xl px-3 text-sm font-semibold text-slate-100 hover:bg-white/10"
                        >
                          {item.label}
                        </Link>
                      )
                    )}
                    <hr className="my-1 border-indigo-400/20" />
                  </div>
                  <button
                    type="button"
                    onClick={() => handleLocaleSwitch(locale === "en" ? "id" : "en")}
                    className="flex w-full min-h-11 items-center justify-between rounded-xl px-3 text-sm text-cyan-200 hover:bg-white/10 cursor-pointer"
                  >
                    <span>{t("language.label")}</span>
                    <span className="font-bold">
                      {locale === "en" ? t("language.id") : t("language.en")}
                    </span>
                  </button>
                </div>
              ) : null}
            </div>
          </div>
        </div>
      </header>

      {/* PWA Offline Resilience Banner */}
      {isOffline ? (
        <div
          role="status"
          className="border-b border-amber-400/30 bg-amber-500/15 px-4 py-2.5 text-center text-sm text-amber-200"
        >
          <span className="inline-flex items-center gap-2 font-semibold">
            <WifiOff className="h-4 w-4" aria-hidden="true" />
            {t("pwa.offlineTitle")} — {t("pwa.offlineBody")}
          </span>
        </div>
      ) : null}

      <main className="mx-auto w-full max-w-[1440px] flex-1 px-4 py-6 sm:px-6">{children}</main>
    </div>
  );
}
