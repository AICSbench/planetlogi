export type Role =
  | "STUDENT"
  | "PARENT"
  | "TEACHER"
  | "SCHOOL_ADMIN"
  | "ADMIN"
  | "CONTENT_EDITOR";

export interface Principal {
  id: string;
  role: Role;
  displayName: string;
  grade?: number;
  preferredLocale: "en" | "id";
  timezone: string;
  otpEnabled?: boolean;
  avatarItemId?: string;
  totalSp?: number;
  astroCoins?: number;
  pilotLevel?: number;
  currentStreak?: number;
}

export interface LiveFeedItem {
  id: string;
  maskedName: string;
  grade: number;
  avatarId: string;
  points: number;
  sourceLabel: string;
  submittedAt: string;
}

export interface InputPart {
  key: string;
  label?: string;
  prefix?: string;
  suffix?: string;
}

export interface MCQOption {
  key: string;
  label: string;
}

export interface DiagramSpec {
  type: string;
  params: Record<string, unknown>;
}

export interface SolutionStep {
  text: string;
  diagram?: DiagramSpec;
}

export interface RenderedQuestion {
  id: string;
  templateCode: string;
  seed: number;
  grade: number;
  skillCode: string;
  skillName: string;
  curriculumCode: string;
  difficulty: number;
  heuristicCode?: string;
  heuristicName?: string;
  inputType: "single_numeric" | "multi_numeric" | "short_text" | "mcq" | "fraction";
  locale: string;
  stem: string;
  ttsText: string;
  hints: string[];
  parts?: InputPart[];
  options?: MCQOption[];
  diagram?: DiagramSpec;
  videoTutorialId?: string;
}

export interface WorkedSolution {
  correctAnswers: Record<string, string>;
  steps: SolutionStep[];
}

export interface GradeResult {
  isCorrect: boolean;
  correctAnswers: Record<string, string>;
  workedSolution: WorkedSolution;
}

export type DailyOrbitTier = "ELEMENTARY" | "INTERMEDIATE" | "ADVANCED" | "MASTER";

export interface DailyOrbitSlot {
  index: number;
  stageName: string;
  nodeShape: "circle" | "hexagon" | "star";
  rewardSp: number;
  status: "PENDING" | "CORRECT" | "WRONG" | "SKIPPED";
  earnedSp: number;
  question: RenderedQuestion;
  submittedAnswer?: Record<string, string>;
  workedSolution?: WorkedSolution;
}

export interface DailyOrbitSession {
  localDate: string;
  isOpen: boolean;
  windowNotice?: string;
  tier: DailyOrbitTier;
  unlockedTiers: DailyOrbitTier[];
  activeCurricula: string[];
  changeQuestionUsed: number;
  changeQuestionLimit: number;
  correctCount: number;
  completedCount: number;
  totalSpEarned: number;
  completionBonusAwarded: boolean;
  todaySpBreakdown: {
    dailyOrbit: number;
    starDuels: number;
    sundayMeteor: number;
    totalToday: number;
  };
  slots: DailyOrbitSlot[];
}

export interface StreakStatus {
  currentStreak: number;
  longestStreak: number;
  cryoShields: number;
  maxCryoShields: number;
  novaPetStage: number;
  todayCorrectCount: number;
  dailyMilestoneTarget: number;
  lastCompletedDate?: string;
  reminderEnabled: boolean;
  reminderTimeLocal?: string;
}

export interface StreakCalendarDay {
  date: string;
  status: "COMPLETED" | "FROZEN" | "MISSED";
}

export interface CurriculumSummary {
  code: string;
  name: string;
  description: string;
  skillCount: number;
}

export interface SkillView {
  code: string;
  name: string;
  grade: number;
  curricula: string[];
  category: string;
  difficulty: number;
  heuristicCode?: string;
  ageBand?: string;
  templateCode: string;
  nationalAvgPct: number;
  attemptsCount: number;
  correctCount: number;
  proficiencyPct: number;
  proficiencyTier: "INCOMPLETE" | "BEGINNING" | "COMPETENT" | "MASTERY";
  bestSessionScore: number;
  bestStars: number;
}

export interface HOTSQuadrant {
  code: string;
  name: string;
  heuristics: Array<{
    code: string;
    name: string;
    skillCount: number;
    skills: SkillView[];
  }>;
}

export interface PracticeSession {
  sessionId: string;
  skillCode: string;
  skillName: string;
  questions: RenderedQuestion[];
  answered: number;
  correct: number;
  starsEarned: number;
}

export type PracticeSessionProgress = Pick<
  PracticeSession,
  "sessionId" | "skillCode" | "answered" | "correct" | "starsEarned"
>;

export interface ProficiencyReport {
  masteryCount: number;
  competentCount: number;
  beginningCount: number;
  incompleteCount: number;
  skills: SkillView[];
}

export interface HighScoreReport {
  tier9To10: SkillView[];
  tier6To8: SkillView[];
  tier4To5: SkillView[];
  tier1To3: SkillView[];
  unplayed: SkillView[];
}

export interface AcademyLessonSummary {
  code: string;
  title: string;
  ageBand: string;
  completionPct: number;
  scrollEarned: boolean;
  skillCode: string;
}

export interface AcademyTopicSummary {
  code: string;
  grade: number;
  title: string;
  ageBand: string;
  scrollsEarned: number;
  totalScrolls: number;
  lessons: AcademyLessonSummary[];
}

export interface AcademyLessonDetail {
  code: string;
  topicCode: string;
  title: string;
  ageBand: string;
  skillCode: string;
  storyVideoTitle: string;
  conceptVideoTitle: string;
  storyVideoUrl?: string;
  conceptVideoUrl?: string;
  storyCaptionsUrl?: string;
  conceptCaptionsUrl?: string;
  /** YouTube video IDs, played through the privacy-enhanced youtube-nocookie.com embed. */
  storyYoutubeId?: string;
  conceptYoutubeId?: string;
  completionPct: number;
  scrollEarned: boolean;
  doAndLearn: Array<{
    index: number;
    question: RenderedQuestion;
    draftAnswer?: Record<string, string>;
    solved: boolean;
    lastWrong: boolean;
    workedSolution?: WorkedSolution;
  }>;
  selfCheck: Array<{ key: string; label: string; checked: boolean }>;
}

export interface AstroQuizPaperSummary {
  code: string;
  grade: number;
  category: "ESSENTIAL" | "TOP_SCHOOL" | "HOTS";
  title: string;
  questionCount: number;
  status: "NOT_STARTED" | "IN_PROGRESS" | "SUBMITTED";
  bestScore: number;
  weeklyDeadline: string;
}

export interface AstroQuizAttemptView {
  paperCode: string;
  title: string;
  grade: number;
  category: "ESSENTIAL" | "TOP_SCHOOL" | "HOTS";
  status: "IN_PROGRESS" | "SUBMITTED";
  elapsedSec: number;
  score: number;
  totalQuestions: number;
  supplyPodAwarded: boolean;
  supplyPodClaimedThisWeek: boolean;
  questions: Array<{
    index: number;
    question: RenderedQuestion;
    draftAnswer?: Record<string, string>;
    isCorrect?: boolean;
    workedSolution?: WorkedSolution;
  }>;
}

export interface PrintablePaper {
  paperCode: string;
  title: string;
  grade: number;
  category: string;
  questions: RenderedQuestion[];
  /** Whether requesting includeKey=true will succeed (adults, or after this student submitted). */
  answerKeyAvailable: boolean;
  /** Present only when requested with includeKey=true and allowed. */
  answerKeys?: Array<Record<string, string>>;
}

export interface AssignmentSummary {
  id: string;
  title: string;
  assignedByRole: "SYSTEM" | "PARENT" | "TEACHER";
  assignedByName: string;
  academicYear: number;
  dueDate: string;
  status: "INCOMPLETE" | "COMPLETE";
  questionCount: number;
  completedCount: number;
  score: number;
  rewardAc: number;
}

export interface AssignmentDetail {
  summary: AssignmentSummary;
  questions: Array<{
    index: number;
    question: RenderedQuestion;
    submittedAnswer?: Record<string, string>;
    isCorrect?: boolean;
    workedSolution?: WorkedSolution;
  }>;
}

export interface ProfileBundle {
  parent: { name: string; maskedEmail: string; maskedPhone: string };
  child: {
    studentId: string;
    firstName: string;
    grade: number;
    schoolName: string;
    nextPromotionDate: string;
    birthday: string;
    gender: string;
    avatarItemId: string;
  };
  settings: {
    otpEnabled: boolean;
    language: "en" | "id";
    dailyOrbitCurricula: string[];
    starDuelCurricula: string[];
    brainGalaxyDailyCapMin: number;
    brainGalaxyWindowStart: string;
    brainGalaxyWindowEnd: string;
    subscriptionPlanName: string;
    subscriptionValidUntil: string;
    billingEnabled: boolean;
  };
}

export interface ParentChildOverview {
  child: ProfileBundle["child"];
  streakDays: number;
  cryoShields: number;
  masteryCount: number;
  competentCount: number;
  beginningCount: number;
  incompleteCount: number;
  focusSkills: SkillView[];
  strengthSkills: SkillView[];
}

export class ApiError extends Error {
  status: number;
  detail?: string;

  constructor(status: number, message: string, detail?: string) {
    super(message);
    this.status = status;
    this.detail = detail;
  }
}

export function createApiClient(baseUrl: string) {
  const cleanBase = baseUrl.replace(/\/$/, "");

  async function request<T>(path: string, init?: RequestInit): Promise<T> {
    const res = await fetch(`${cleanBase}${path}`, {
      ...init,
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
        ...(init?.headers ?? {}),
      },
    });
    if (!res.ok) {
      let detail: string | undefined;
      try {
        const body = (await res.json()) as { detail?: string; title?: string };
        detail = body.detail ?? body.title;
      } catch {
        // ignore non-json errors
      }
      throw new ApiError(res.status, detail ?? `HTTP ${res.status}`, detail);
    }
    return (await res.json()) as T;
  }

  return {
    // Auth
    login(identifier: string, password: string) {
      return request<{ user: Principal }>("/api/v1/auth/login", {
        method: "POST",
        body: JSON.stringify({ identifier, password }),
      });
    },
    me() {
      return request<{ user: Principal }>("/api/v1/auth/me", { method: "GET" });
    },
    refresh() {
      return request<{ user: Principal }>("/api/v1/auth/refresh", { method: "POST" });
    },
    logout() {
      return request<{ ok: boolean }>("/api/v1/auth/logout", { method: "POST" });
    },
    async getLiveFeed() {
      const res = await request<{ items?: LiveFeedItem[] | null }>("/api/v1/auth/live-feed", {
        method: "GET",
      });
      return { items: res.items ?? [] };
    },
    requestParentOtp() {
      return request<{ sent: boolean; devCode?: string }>("/api/v1/auth/otp/request", {
        method: "POST",
      });
    },
    verifyParentOtp(code: string) {
      return request<{ verified: boolean }>("/api/v1/auth/otp/verify", {
        method: "POST",
        body: JSON.stringify({ code }),
      });
    },

    // Daily Orbit
    getDailyOrbitToday(locale: string, grade = 1) {
      return request<DailyOrbitSession>(`/api/v1/daily-orbit/today?locale=${locale}&grade=${grade}`);
    },
    submitDailyOrbit(
      slotIndex: number,
      answers: Record<string, string>,
      locale: string,
      grade = 1,
      questionMeta?: { templateCode?: string; seed?: number }
    ) {
      return request<{ slot: DailyOrbitSlot; session: DailyOrbitSession }>(
        `/api/v1/daily-orbit/submit?locale=${locale}&grade=${grade}`,
        {
          method: "POST",
          body: JSON.stringify({
            slotIndex,
            answers,
            templateCode: questionMeta?.templateCode,
            seed: questionMeta?.seed,
          }),
        }
      );
    },
    skipDailyOrbit(slotIndex: number, locale: string, grade = 1) {
      return request<{ slot: DailyOrbitSlot; session: DailyOrbitSession }>(
        `/api/v1/daily-orbit/skip?locale=${locale}&grade=${grade}`,
        { method: "POST", body: JSON.stringify({ slotIndex }) }
      );
    },
    changeDailyOrbitQuestion(slotIndex: number, mode: "easier" | "harder" | "topic", locale: string, grade = 1) {
      return request<{ slot: DailyOrbitSlot; session: DailyOrbitSession }>(
        `/api/v1/daily-orbit/change-question?locale=${locale}&grade=${grade}`,
        { method: "POST", body: JSON.stringify({ slotIndex, mode }) }
      );
    },
    setDailyOrbitTier(tier: DailyOrbitTier, locale: string, grade = 1) {
      return request<DailyOrbitSession>(
        `/api/v1/daily-orbit/tier?locale=${locale}&grade=${grade}`,
        { method: "PATCH", body: JSON.stringify({ tier }) }
      );
    },
    setDailyOrbitCurricula(curricula: string[], locale: string, grade = 1) {
      return request<DailyOrbitSession>(
        `/api/v1/daily-orbit/curricula?locale=${locale}&grade=${grade}`,
        { method: "PATCH", body: JSON.stringify({ curricula }) }
      );
    },

    // Streak
    getStreakStatus() {
      return request<StreakStatus>("/api/v1/streak/status");
    },
    getStreakCalendar(month?: string) {
      const q = month ? `?month=${encodeURIComponent(month)}` : "";
      return request<{ days: StreakCalendarDay[] }>(`/api/v1/streak/calendar${q}`);
    },
    setStreakReminder(enabled: boolean, timeLocal?: string) {
      return request<StreakStatus>("/api/v1/streak/reminder", {
        method: "POST",
        body: JSON.stringify({ enabled, timeLocal }),
      });
    },

    // Galaxy Training
    getCurricula(locale: string) {
      return request<{ curricula: CurriculumSummary[] }>(`/api/v1/training/curricula?locale=${locale}`);
    },
    getSkills(curriculum: string, grade: number, locale: string) {
      return request<{ skills: SkillView[] }>(
        `/api/v1/training/skills?curriculum=${encodeURIComponent(curriculum)}&grade=${grade}&locale=${locale}`
      );
    },
    getHotsWheel(ageBand: string, locale: string) {
      return request<{ quadrants: HOTSQuadrant[] }>(
        `/api/v1/training/hots?ageBand=${encodeURIComponent(ageBand)}&locale=${locale}`
      );
    },
    startPracticeSession(skillCode: string, locale: string) {
      return request<PracticeSession>(`/api/v1/training/sessions?locale=${locale}`, {
        method: "POST",
        body: JSON.stringify({ skillCode }),
      });
    },
    submitPracticeAnswer(
      sessionId: string,
      questionIndex: number,
      answers: Record<string, string>,
      locale: string
    ) {
      return request<{ result: GradeResult; session: PracticeSessionProgress }>(
        `/api/v1/training/sessions/submit?locale=${locale}`,
        { method: "POST", body: JSON.stringify({ sessionId, questionIndex, answers }) }
      );
    },

    // Reports
    getProficiencyReport(curriculum: string, grade: number, locale: string) {
      return request<ProficiencyReport>(
        `/api/v1/reports/proficiency?curriculum=${encodeURIComponent(curriculum)}&grade=${grade}&locale=${locale}`
      );
    },
    getHighScoreReport(curriculum: string, grade: number, locale: string) {
      return request<HighScoreReport>(
        `/api/v1/reports/high-score?curriculum=${encodeURIComponent(curriculum)}&grade=${grade}&locale=${locale}`
      );
    },

    // Space Academy
    getAcademyTopics(grade: number, locale: string) {
      return request<{ topics: AcademyTopicSummary[] }>(
        `/api/v1/academy/topics?grade=${grade}&locale=${locale}`
      );
    },
    getAcademyLesson(code: string, locale: string) {
      return request<AcademyLessonDetail>(
        `/api/v1/academy/lessons/${encodeURIComponent(code)}?locale=${locale}`
      );
    },
    saveAcademyDraft(lessonCode: string, questionIndex: number, answers: Record<string, string>, locale: string) {
      return request<AcademyLessonDetail>(`/api/v1/academy/do-learn/save?locale=${locale}`, {
        method: "POST",
        body: JSON.stringify({ lessonCode, questionIndex, answers }),
      });
    },
    checkAcademyAnswer(lessonCode: string, questionIndex: number, answers: Record<string, string>, locale: string) {
      return request<{ result: GradeResult; lesson: AcademyLessonDetail }>(
        `/api/v1/academy/do-learn/check?locale=${locale}`,
        { method: "POST", body: JSON.stringify({ lessonCode, questionIndex, answers }) }
      );
    },
    toggleAcademySelfCheck(lessonCode: string, itemKey: string, checked: boolean, locale: string) {
      return request<AcademyLessonDetail>(`/api/v1/academy/self-check?locale=${locale}`, {
        method: "PATCH",
        body: JSON.stringify({ lessonCode, itemKey, checked }),
      });
    },
    retryAcademyScroll(lessonCode: string, locale: string) {
      return request<AcademyLessonDetail>(`/api/v1/academy/scroll/retry?locale=${locale}`, {
        method: "POST",
        body: JSON.stringify({ lessonCode }),
      });
    },

    // AstroQuiz
    getAstroQuizPapers(grade: number, category: string, locale: string) {
      return request<{ papers: AstroQuizPaperSummary[]; supplyPodClaimedThisWeek: boolean }>(
        `/api/v1/astro-quiz/papers?grade=${grade}&category=${encodeURIComponent(category)}&locale=${locale}`
      );
    },
    openAstroQuizPaper(code: string, locale: string) {
      return request<AstroQuizAttemptView>(
        `/api/v1/astro-quiz/papers/${encodeURIComponent(code)}?locale=${locale}`
      );
    },
    saveAstroQuizPaper(code: string, elapsedSec: number, drafts: Record<number, Record<string, string>>, locale: string) {
      return request<AstroQuizAttemptView>(
        `/api/v1/astro-quiz/papers/${encodeURIComponent(code)}/save?locale=${locale}`,
        { method: "POST", body: JSON.stringify({ elapsedSec, drafts }) }
      );
    },
    submitAstroQuizPaper(code: string, elapsedSec: number, drafts: Record<number, Record<string, string>>, locale: string) {
      return request<AstroQuizAttemptView>(
        `/api/v1/astro-quiz/papers/${encodeURIComponent(code)}/submit?locale=${locale}`,
        { method: "POST", body: JSON.stringify({ elapsedSec, drafts }) }
      );
    },
    getPrintablePaper(code: string, locale: string, includeKey = false) {
      return request<PrintablePaper>(
        `/api/v1/astro-quiz/papers/${encodeURIComponent(code)}/print?locale=${locale}&includeKey=${includeKey}`
      );
    },

    // Assignments
    getAssignments(status = "ALL", assignedBy = "ALL", year = 0) {
      return request<{ assignments: AssignmentSummary[] }>(
        `/api/v1/assignments?status=${encodeURIComponent(status)}&assignedBy=${encodeURIComponent(assignedBy)}&year=${year}`
      );
    },
    createCustomAssignment(payload: {
      studentId?: string;
      title: string;
      dueDate?: string;
      templateCodes: string[];
      requestId?: string;
    }) {
      return request<AssignmentSummary>("/api/v1/assignments/custom", {
        method: "POST",
        body: JSON.stringify(payload),
      });
    },
    getAssignmentDetail(id: string, locale: string) {
      return request<AssignmentDetail>(
        `/api/v1/assignments/${encodeURIComponent(id)}?locale=${locale}`
      );
    },
    submitAssignmentQuestion(id: string, questionIndex: number, answers: Record<string, string>, locale: string) {
      return request<{ result: GradeResult; assignment: AssignmentDetail }>(
        `/api/v1/assignments/${encodeURIComponent(id)}/submit?locale=${locale}`,
        { method: "POST", body: JSON.stringify({ questionIndex, answers }) }
      );
    },

    // Profile & Parent Portal
    getProfileBundle() {
      return request<ProfileBundle>("/api/v1/profile/bundle");
    },
    getProfileChildren() {
      return request<{ children: ProfileBundle["child"][] }>("/api/v1/profile/children");
    },
    requestParentOTP() {
      return request<{ sent: boolean; devCode?: string }>("/api/v1/auth/otp/request", {
        method: "POST",
      });
    },
    updateProfileSettings(payload: {
      otpEnabled?: boolean;
      otpCode?: string;
      language?: "en" | "id";
      dailyOrbitCurricula?: string[];
      starDuelCurricula?: string[];
      brainGalaxyDailyCapMin?: number;
      brainGalaxyWindowStart?: string;
      brainGalaxyWindowEnd?: string;
    }) {
      return request<ProfileBundle>("/api/v1/profile/settings", {
        method: "PATCH",
        body: JSON.stringify(payload),
      });
    },
    getParentOverview(locale: string) {
      return request<ParentChildOverview>(`/api/v1/parent/overview?locale=${locale}`);
    },
  };
}
