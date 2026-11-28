import enCommon from "../locales/en/common.json";
import enAuth from "../locales/en/auth.json";
import enDashboard from "../locales/en/dashboard.json";
import enDailyOrbit from "../locales/en/daily-orbit.json";
import enTraining from "../locales/en/training.json";
import enPlayer from "../locales/en/player.json";
import enAcademy from "../locales/en/academy.json";
import enAstroQuiz from "../locales/en/astro-quiz.json";
import enPortals from "../locales/en/portals.json";
import idCommon from "../locales/id/common.json";
import idAuth from "../locales/id/auth.json";
import idDashboard from "../locales/id/dashboard.json";
import idDailyOrbit from "../locales/id/daily-orbit.json";
import idTraining from "../locales/id/training.json";
import idPlayer from "../locales/id/player.json";
import idAcademy from "../locales/id/academy.json";
import idAstroQuiz from "../locales/id/astro-quiz.json";
import idPortals from "../locales/id/portals.json";

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
    academy: enAcademy,
    "astro-quiz": enAstroQuiz,
    portals: enPortals,
  },
  id: {
    common: idCommon,
    auth: idAuth,
    dashboard: idDashboard,
    "daily-orbit": idDailyOrbit,
    training: idTraining,
    player: idPlayer,
    academy: idAcademy,
    "astro-quiz": idAstroQuiz,
    portals: idPortals,
  },
} as const;
