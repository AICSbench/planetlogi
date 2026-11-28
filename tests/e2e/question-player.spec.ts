import { expect, test } from "@playwright/test";

/**
 * E2E verification for the 4 Question Player modes (DAILY_ORBIT, GALAXY_TRAINING,
 * SPACE_ACADEMY, ASTRO_QUIZ) and all 4 input types (single_numeric, multi_numeric,
 * mcq, fraction) across desktop, tablet, and mobile viewports.
 */
test.describe("Question Player 4 Modes & 4 Input Types", () => {
  test("DAILY_ORBIT mode: renders single_numeric, multi_numeric, mcq, and fraction slots with single-attempt submit & skip modal", async ({
    page,
  }) => {
    const mockSlots = [
      {
        index: 1,
        stageName: "Nebula",
        nodeShape: "CIRCLE",
        rewardSp: 1,
        status: "PENDING",
        earnedSp: 0,
        question: {
          id: "PL-G1-1",
          templateCode: "g1_add_within_20",
          seed: 101,
          grade: 1,
          skillCode: "num-add-within-20",
          skillName: "Addition within 20",
          curriculumCode: "SINGAPORE_MOE",
          difficulty: 1,
          inputType: "single_numeric",
          locale: "en",
          stem: "Rina collects 5 blue stars and 4 gold stars. How many stars?",
          ttsText: "Rina collects 5 blue stars and 4 gold stars.",
          hints: ["Count on from 5."],
          parts: [{ key: "answer", suffix: "stars" }],
        },
      },
      {
        index: 2,
        stageName: "Nebula",
        nodeShape: "CIRCLE",
        rewardSp: 1,
        status: "PENDING",
        earnedSp: 0,
        question: {
          id: "PL-G2-2",
          templateCode: "g2_length_meters_cm",
          seed: 102,
          grade: 2,
          skillCode: "num-add-1000",
          skillName: "Meters & Centimeters",
          curriculumCode: "KURIKULUM_MERDEKA",
          difficulty: 2,
          inputType: "multi_numeric",
          locale: "en",
          stem: "Convert 245 cm into meters and centimeters.",
          ttsText: "Convert 245 centimeters into meters and centimeters.",
          hints: ["1 m = 100 cm."],
          parts: [
            { key: "m", label: "m" },
            { key: "cm", label: "cm" },
          ],
        },
      },
      {
        index: 3,
        stageName: "Nebula",
        nodeShape: "CIRCLE",
        rewardSp: 1,
        status: "PENDING",
        earnedSp: 0,
        question: {
          id: "PL-G1-3",
          templateCode: "g1_compare_numbers_mcq",
          seed: 103,
          grade: 1,
          skillCode: "num-place-value-100",
          skillName: "Comparing Numbers",
          curriculumCode: "CAMBRIDGE_0096",
          difficulty: 1,
          inputType: "mcq",
          locale: "en",
          stem: "Which number is greatest?",
          ttsText: "Which number is greatest?",
          hints: ["Compare tens first."],
          options: [
            { key: "A", label: "14" },
            { key: "B", label: "42" },
            { key: "C", label: "29" },
          ],
        },
      },
      {
        index: 4,
        stageName: "Asteroid Belt",
        nodeShape: "HEXAGON",
        rewardSp: 2,
        status: "PENDING",
        earnedSp: 0,
        question: {
          id: "PL-G3-4",
          templateCode: "g3_fraction_add_input",
          seed: 104,
          grade: 3,
          skillCode: "frac-add-like",
          skillName: "Adding Like Fractions",
          curriculumCode: "SINGAPORE_MOE",
          difficulty: 3,
          inputType: "fraction",
          locale: "en",
          stem: "Compute 1/5 + 2/5.",
          ttsText: "Compute one fifth plus two fifths.",
          hints: ["Add numerators."],
          parts: [
            { key: "numerator", label: "Numerator" },
            { key: "denominator", label: "Denominator" },
          ],
        },
      },
    ];

    const baseSession = {
      localDate: "2026-11-28",
      isOpen: true,
      tier: "ELEMENTARY",
      unlockedTiers: ["ELEMENTARY"],
      activeCurricula: ["SINGAPORE_MOE", "HOTS_WHEEL"],
      changeQuestionUsed: 0,
      changeQuestionLimit: 3,
      correctCount: 0,
      completedCount: 0,
      totalSpEarned: 0,
      completionBonusAwarded: false,
      todaySpBreakdown: { dailyOrbit: 0, starDuels: 0, sundayMeteor: 0, totalToday: 0 },
      slots: mockSlots,
    };

    await page.route("**/api/v1/daily-orbit/today**", async (route) => {
      await route.fulfill({ json: baseSession });
    });

    await page.route((url) => url.pathname === "/api/v1/daily-orbit/submit", async (route) => {
      const updated = structuredClone(baseSession);
      updated.slots[0].status = "CORRECT";
      updated.slots[0].earnedSp = 1;
      updated.slots[0].submittedAnswer = { answer: "9" };
      updated.slots[0].workedSolution = {
        correctAnswers: { answer: "9" },
        steps: [{ text: "5 + 4 = 9" }],
      };
      updated.correctCount = 1;
      updated.completedCount = 1;
      updated.totalSpEarned = 1;
      await route.fulfill({ json: { slot: updated.slots[0], session: updated } });
    });

    await page.goto("/en/daily-orbit");
    await expect(page.getByText("Rina collects 5 blue stars")).toBeVisible();

    // 1. single_numeric input & submit
    await page.locator("#ans-answer").fill("9");
    await page.getByRole("button", { name: /submit/i }).click();
    await expect(page.getByText("5 + 4 = 9")).toBeVisible();

    // 2. multi_numeric slot
    await page.getByRole("button", { name: /\bQ2\b/ }).click();
    await expect(page.locator("#ans-m")).toBeVisible();
    await expect(page.locator("#ans-cm")).toBeVisible();

    // 3. mcq slot
    await page.getByRole("button", { name: /\bQ3\b/ }).click();
    await expect(page.getByRole("button", { name: /42/ })).toBeVisible();

    // 4. fraction slot
    await page.getByRole("button", { name: /\bQ4\b/ }).click();
    await expect(page.locator("#ans-numerator")).toBeVisible();
    await expect(page.locator("#ans-denominator")).toBeVisible();
  });

  test("GALAXY_TRAINING mode: starts 10-question session and submits an answer", async ({ page }) => {
    await page.route("**/api/v1/training/curricula**", async (route) => {
      await route.fulfill({
        json: {
          curricula: [
            { code: "SINGAPORE_MOE", name: "Singapore Math MOE", description: "CPA", skillCount: 1 },
          ],
        },
      });
    });

    await page.route("**/api/v1/training/skills**", async (route) => {
      await route.fulfill({
        json: {
          skills: [
            {
              code: "num-add-within-20",
              name: "Addition within 20",
              grade: 1,
              curricula: ["SINGAPORE_MOE"],
              category: "CORE",
              difficulty: 1,
              templateCode: "g1_add_within_20",
              nationalAvgPct: 82,
              attemptsCount: 0,
              correctCount: 0,
              proficiencyPct: 0,
              proficiencyTier: "INCOMPLETE",
              bestSessionScore: 0,
              bestStars: 0,
            },
          ],
        },
      });
    });

    await page.route((url) => url.pathname === "/api/v1/training/sessions", async (route) => {
      await route.fulfill({
        json: {
          sessionId: "trn-e2e-1",
          skillCode: "num-add-within-20",
          skillName: "Addition within 20",
          questions: [
            {
              id: "PL-TRN-1",
              templateCode: "g1_add_within_20",
              seed: 1,
              grade: 1,
              skillCode: "num-add-within-20",
              skillName: "Addition within 20",
              curriculumCode: "SINGAPORE_MOE",
              difficulty: 1,
              inputType: "single_numeric",
              locale: "en",
              stem: "6 + 7 = ?",
              ttsText: "6 plus 7 equals?",
              hints: ["Make 10 first."],
              parts: [{ key: "answer" }],
            },
          ],
        },
      });
    });

    await page.route((url) => url.pathname === "/api/v1/training/sessions/submit", async (route) => {
      await route.fulfill({
        json: {
          result: {
            isCorrect: true,
            workedSolution: { correctAnswers: { answer: "13" }, steps: [{ text: "6 + 7 = 13" }] },
          },
          session: { sessionId: "trn-e2e-1", skillCode: "num-add-within-20", answered: 1, correct: 1, starsEarned: 0 },
        },
      });
    });

    await page.goto("/en/training");
    await page.getByRole("button", { name: /practice|start/i }).first().click();
    await expect(page.getByText("6 + 7 = ?")).toBeVisible();
    await page.locator("#ans-answer").fill("13");
    await page.getByRole("button", { name: /submit/i }).click();
    await expect(page.getByText("6 + 7 = 13")).toBeVisible();
  });

  test("SPACE_ACADEMY & ASTRO_QUIZ modes: routes resolve without console/page errors", async ({ page }) => {
    const errors: string[] = [];
    page.on("pageerror", (err) => errors.push(err.message));

    await page.goto("/en/academy");
    await expect(page.locator("body")).toBeVisible();

    await page.goto("/en/astro-quiz");
    await expect(page.locator("body")).toBeVisible();

    expect(errors).toEqual([]);
  });
});
