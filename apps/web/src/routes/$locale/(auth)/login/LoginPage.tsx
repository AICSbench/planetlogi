import { useState, type FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Eye, EyeOff, Globe, Rocket, Sparkles } from "lucide-react";
import type { Locale } from "@planetlogi/i18n";
import { GalaxyButton, GalaxyCard, CurrencyPill } from "@planetlogi/ui";
import { ApiError, type LiveFeedItem } from "@planetlogi/api-client";
import { setAppLocale } from "../../../../i18n/init";
import { apiClient, useAuthStore } from "../../../../store/authStore";

const FALLBACK_FEED: LiveFeedItem[] = [
  { id: "f1", maskedName: "Raka S.", grade: 1, avatarId: "astro_fox", points: 3, sourceLabel: "Daily Orbit", submittedAt: "" },
  { id: "f2", maskedName: "Maya K.", grade: 3, avatarId: "cosmic_dino", points: 2, sourceLabel: "Daily Orbit", submittedAt: "" },
  { id: "f3", maskedName: "Ethan L.", grade: 5, avatarId: "mini_bot", points: 3, sourceLabel: "Star Duels", submittedAt: "" },
  { id: "f4", maskedName: "Siti R.", grade: 2, avatarId: "star_owl", points: 2, sourceLabel: "Daily Orbit", submittedAt: "" },
  { id: "f5", maskedName: "Lucas W.", grade: 6, avatarId: "mecha_pilot", points: 3, sourceLabel: "Daily Orbit", submittedAt: "" },
];

export function LoginPage({ locale }: { locale: Locale }) {
  const { t } = useTranslation(["auth", "common"]);
  const navigate = useNavigate();
  const setUser = useAuthStore((s) => s.setUser);

  const [identifier, setIdentifier] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const feedQuery = useQuery({
    queryKey: ["auth-live-feed"],
    queryFn: () => apiClient.getLiveFeed().then((r) => r.items),
    refetchInterval: 15_000,
    retry: false,
  });

  const feedItems = feedQuery.data ?? FALLBACK_FEED;

  async function handleLocaleToggle() {
    const nextLocale: Locale = locale === "en" ? "id" : "en";
    await setAppLocale(nextLocale);
    void navigate({ to: "/$locale/login", params: { locale: nextLocale } });
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setErrorMsg(null);
    setSubmitting(true);
    try {
      const res = await apiClient.login(identifier.trim(), password);
      setUser(res.user);
      const targetLocale = res.user.preferredLocale ?? locale;
      await setAppLocale(targetLocale);
      void navigate({ to: "/$locale/dashboard", params: { locale: targetLocale } });
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setErrorMsg(t("auth:errorInvalidCredentials"));
      } else {
        setErrorMsg(t("auth:errorGeneric"));
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="min-h-screen planetlogi-galaxy-bg text-slate-100 flex flex-col">
      <div className="mx-auto flex w-full max-w-6xl items-center justify-between px-4 py-4 sm:px-6">
        <div className="flex items-center gap-2.5">
          <span className="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-[#22D3EE] to-[#6366F1] shadow">
            <Rocket className="h-5 w-5 text-white" aria-hidden="true" />
          </span>
          <div>
            <p className="text-xl font-extrabold tracking-tight text-white">{t("common:brand")}</p>
            <p className="text-xs text-slate-300">{t("common:tagline")}</p>
          </div>
        </div>

        <button
          type="button"
          onClick={handleLocaleToggle}
          className="flex min-h-11 items-center gap-2 rounded-xl border border-indigo-400/30 bg-[#131B3A] px-3.5 text-sm font-semibold text-cyan-200 hover:bg-[#1E2A55] cursor-pointer"
        >
          <Globe className="h-4 w-4" aria-hidden="true" />
          <span>{locale === "en" ? t("common:language.id") : t("common:language.en")}</span>
        </button>
      </div>

      <div className="mx-auto grid w-full max-w-6xl flex-1 grid-cols-1 items-center gap-8 px-4 py-6 sm:px-6 lg:grid-cols-12">
        {/* Left Panel: Live Anonymized Star Points Feed */}
        <section aria-label={t("auth:liveFeedTitle")} className="order-2 lg:order-1 lg:col-span-6">
          <GalaxyCard className="space-y-4">
            <div className="flex items-center gap-2.5">
              <Sparkles className="h-5 w-5 text-[#FBBF24]" aria-hidden="true" />
              <div>
                <h2 className="text-lg font-bold text-white">{t("auth:liveFeedTitle")}</h2>
                <p className="text-xs text-slate-300">{t("auth:liveFeedSubtitle")}</p>
              </div>
            </div>
            <ul className="space-y-2.5">
              {feedItems.map((item) => (
                <li
                  key={item.id}
                  className="flex items-center justify-between gap-3 rounded-xl border border-indigo-400/15 bg-[#1E2A55]/70 px-3.5 py-2.5"
                >
                  <div className="flex items-center gap-3">
                    <span className="flex h-9 w-9 items-center justify-center rounded-full bg-[#6366F1]/30 text-sm font-bold text-indigo-100">
                      {item.maskedName.slice(0, 1)}
                    </span>
                    <div>
                      <p className="text-sm font-bold text-white">{item.maskedName}</p>
                      <p className="text-xs text-slate-300">
                        {t("common:common.grade", { grade: item.grade })} •{" "}
                        {t("auth:liveFeedEarned", {
                          points: item.points,
                          source: item.sourceLabel,
                        })}
                      </p>
                    </div>
                  </div>
                  <CurrencyPill kind="SP" value={`+${item.points}`} label={t("common:currencies.sp")} />
                </li>
              ))}
            </ul>
          </GalaxyCard>
        </section>

        {/* Right Panel: Sign-In Form */}
        <section className="order-1 lg:order-2 lg:col-span-6">
          <GalaxyCard className="mx-auto max-w-md space-y-5 p-6 sm:p-8">
            <div>
              <h1 className="text-2xl font-extrabold text-white">{t("auth:title")}</h1>
              <p className="mt-1 text-sm text-slate-300">{t("auth:subtitle")}</p>
            </div>

            <form onSubmit={handleSubmit} className="space-y-4">
              <div>
                <label htmlFor="loginId" className="block text-sm font-semibold text-slate-200">
                  {t("auth:loginIdLabel")}
                </label>
                <input
                  id="loginId"
                  type="text"
                  autoComplete="username"
                  required
                  value={identifier}
                  onChange={(e) => setIdentifier(e.target.value)}
                  placeholder={t("auth:loginIdPlaceholder")}
                  className="mt-1.5 min-h-12 w-full rounded-xl border border-indigo-400/30 bg-[#0B1026] px-4 text-base text-white placeholder:text-slate-500 focus:border-[#22D3EE] focus:outline-none"
                />
              </div>

              <div>
                <label htmlFor="password" className="block text-sm font-semibold text-slate-200">
                  {t("auth:passwordLabel")}
                </label>
                <div className="relative mt-1.5">
                  <input
                    id="password"
                    type={showPassword ? "text" : "password"}
                    autoComplete="current-password"
                    required
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    placeholder={t("auth:passwordPlaceholder")}
                    className="min-h-12 w-full rounded-xl border border-indigo-400/30 bg-[#0B1026] pl-4 pr-24 text-base text-white placeholder:text-slate-500 focus:border-[#22D3EE] focus:outline-none"
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword((prev) => !prev)}
                    className="absolute right-2 top-1/2 -translate-y-1/2 inline-flex min-h-9 items-center gap-1 rounded-lg px-2.5 text-xs font-semibold text-cyan-300 hover:bg-white/10 cursor-pointer"
                  >
                    {showPassword ? (
                      <>
                        <EyeOff className="h-4 w-4" aria-hidden="true" />
                        <span>{t("auth:hidePassword")}</span>
                      </>
                    ) : (
                      <>
                        <Eye className="h-4 w-4" aria-hidden="true" />
                        <span>{t("auth:showPassword")}</span>
                      </>
                    )}
                  </button>
                </div>
              </div>

              {errorMsg ? (
                <div
                  role="alert"
                  className="rounded-xl border border-red-400/40 bg-red-500/15 px-3.5 py-2.5 text-sm text-red-200"
                >
                  {errorMsg}
                </div>
              ) : null}

              <GalaxyButton
                type="submit"
                size="lg"
                disabled={submitting}
                className="w-full"
              >
                {submitting ? t("auth:signingIn") : t("auth:submit")}
              </GalaxyButton>
            </form>

            <div className="space-y-2 border-t border-indigo-400/20 pt-4 text-center text-xs text-slate-300">
              <p>{t("auth:forgotPassword")}</p>
              <p className="rounded-lg bg-[#1E2A55]/70 px-3 py-1.5 font-mono text-cyan-200">
                {t("auth:demoHint")}
              </p>
            </div>
          </GalaxyCard>
        </section>
      </div>
    </div>
  );
}
