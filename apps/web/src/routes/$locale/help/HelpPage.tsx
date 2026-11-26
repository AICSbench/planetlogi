import { useTranslation } from "react-i18next";
import { GalaxyCard } from "@planetlogi/ui";

export function HelpPage() {
  const { t } = useTranslation("portals");
  const faqs = [
    { q: t("help.q1"), a: t("help.a1") },
    { q: t("help.q2"), a: t("help.a2") },
    { q: t("help.q3"), a: t("help.a3") },
  ];

  return (
    <div className="mx-auto max-w-3xl space-y-5">
      <GalaxyCard>
        <h1 className="text-2xl font-extrabold text-white">❓ {t("help.title")}</h1>
        <p className="mt-1 text-sm text-slate-300">{t("help.subtitle")}</p>
      </GalaxyCard>

      <div className="space-y-3">
        {faqs.map((item, i) => (
          <GalaxyCard key={i} className="space-y-1.5">
            <h2 className="text-base font-bold text-cyan-200">{item.q}</h2>
            <p className="text-sm text-slate-300">{item.a}</p>
          </GalaxyCard>
        ))}
      </div>
    </div>
  );
}
