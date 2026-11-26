import enCommon from "../locales/en/common.json";
import enAuth from "../locales/en/auth.json";
import enDashboard from "../locales/en/dashboard.json";
import idCommon from "../locales/id/common.json";
import idAuth from "../locales/id/auth.json";
import idDashboard from "../locales/id/dashboard.json";

export * from "./config";
export * from "./format";

export const resources = {
  en: {
    common: enCommon,
    auth: enAuth,
    dashboard: enDashboard,
  },
  id: {
    common: idCommon,
    auth: idAuth,
    dashboard: idDashboard,
  },
} as const;
