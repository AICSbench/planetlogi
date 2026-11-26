import enCommon from "../locales/en/common.json";
import enAuth from "../locales/en/auth.json";
import enDashboard from "../locales/en/dashboard.json";
import enDailyOrbit from "../locales/en/daily-orbit.json";
import enTraining from "../locales/en/training.json";
import enPlayer from "../locales/en/player.json";
import idCommon from "../locales/id/common.json";
import idAuth from "../locales/id/auth.json";
import idDashboard from "../locales/id/dashboard.json";
import idDailyOrbit from "../locales/id/daily-orbit.json";
import idTraining from "../locales/id/training.json";
import idPlayer from "../locales/id/player.json";

export * from "./config";
export * from "./format";

export const resources = {
  en: {
    common: enCommon,
    auth: enAuth,
    dashboard: enDashboard,
    "daily-orbit": enDailyOrbit,
    training: enTraining,
    player: enPlayer,
  },
  id: {
    common: idCommon,
    auth: idAuth,
    dashboard: idDashboard,
    "daily-orbit": idDailyOrbit,
    training: idTraining,
    player: idPlayer,
  },
} as const;
