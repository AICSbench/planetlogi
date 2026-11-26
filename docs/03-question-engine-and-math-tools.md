# 03 — Question Engine, Player Modes & Interactive Math Tools

> **Owner Tracks**: `ENGINE`, `FE`, `CONTENT`, `BE`  
> **Scope**: Unified Question Player across 4 behavioral modes, bilingual Question JSON schema, deterministic Procedural Question Generator, 4 on-screen interactive Math Working Tools, TTS/Video/Worked Solutions, and A4 Printable Worksheet Renderer.

---

## 1. Unified Question Player — 4 Mode Behaviors

AlgiPlanet uses **one modular Question Player architecture** (`apps/web/src/components/question-player/`) whose rules and UI chrome adapt cleanly via a `mode` prop so there is zero duplicated rendering logic:

| Capability | `DAILY_ORBIT` | `GALAXY_TRAINING` (incl. HOTS) | `SPACE_ACADEMY` (*Do & Learn*) | `ASTRO_QUIZ` |
|---|---|---|---|---|
| **Session Length** | 10 questions (3 reward tiers) | 10 questions (reward bar) | 4–6 questions (awards Star Scroll) | 15–20 questions (exam paper) |
| **Attempt Policy** | Single attempt (locks after submit) | Single attempt (no retry on same Q) | **Multi-attempt (`Check Answer` → `Try Again`)** | Editable until full paper `Submit` |
| **Save Draft** | Auto-saved per question | Auto-saved per session | Explicit **Save** button (`Saved` pill) | Explicit **Save** button |
| **Change Question** | Yes (max **3/day** — easier/harder/topic) | Yes (unlimited fresh variant) | No (curated lesson sequence) | No (fixed exam paper) |
| **Skip / Show Answer** | Yes (Skip confirmation → **0 SP**) | Yes (reveals solution) | Shows hint / allows retry | Navigate freely across Q1–Q20 |
| **Solutions Reveal** | Immediately after submit/skip | Immediately after submit | After correct check or reveal | **Only after full paper `Submit`** |
| **Timer** | None (calm focus) | None | None | **Live elapsed timer (`00:00:00`)** |
| **4 Working Tools** | Available (collapsible drawer) | **Prominent right-side toolbar** | Available | Available |
| **Video Tutorial** | Optional icon if linked | **Prominent Video Tutorial button** | Embedded in prior lesson step | Shown in post-submit review |
| **Print Worksheet** | No | No | No | **Yes (`/astro-quiz/print/[id]`)** |
| **HOTS Heuristic Tag** | Shown if HOTS question | Shown (e.g., *Draw a Diagram*) | N/A | Shown on HOTS papers |

### Shared Chrome Elements on the Question Canvas
- **Header Strip**: Skill Name + `?` Skill Info tooltip, Question ID (e.g., `AP-G3-FRAC-0142`), **5-Level Difficulty Meter**, **TTS Speaker** button, and **Report / Feedback** button (opens quick modal: *Typo*, *Wrong Answer*, *Unclear Diagram*, *Translation Issue*).
- **Collapsible Left Side Nav**: Lists question nodes (`Q1`–`Q10` or `Q1`–`Q20`), status icons (unanswered, current, correct `✓`, wrong `✕`, skipped), and stage names in Daily Orbit.
- **Feedback Micro-Interactions**:
  - **Correct**: Friendly cosmic celebration (*"STAR SHOT! THAT'S RIGHT!"* / *"HEBAT! JAWABANMU BENAR!"*), lightweight confetti burst, and auto-expanded visual worked solution.
  - **Incorrect**: Supportive non-punitive feedback showing *Your Answer* vs *Correct Answer*, **Show Solution**, **Video Tutorial** (if available), and **Next Question** (or **Try Again** in Space Academy mode where wrong input box highlights red `✕` and resets cleanly on retry).

---

## 2. Supported Question & Input Types (Grades 1–6)

All question types MUST work seamlessly with mouse, keyboard, and tablet touchscreens:

1. **Single Numeric Input**: Integer, decimal (locale-aware `.` and `,` tolerance), or currency value with optional prefix/suffix unit label (`cm`, `Rp`, `$`, `kg`).
2. **Multi-Part Numeric / Fill-in-the-Blank**: 2–6 inline boxes embedded in a sentence, table, number bond, or diagram (e.g., Q9 3-box bead/place-value problem).
3. **Letter / Short Text Input**: Single-letter pattern/label answers (`A`, `B`, `C`, `D`) or short word-form numbers (normalized case-insensitive + trimmed).
4. **Multiple Choice (MCQ — Single & Multi-Select)**: 2–4 rich visual option cards supporting text, KaTeX math, or inline SVG figures.
5. **Fraction & Mixed-Number Input**: Dedicated stacked numerator/denominator (and optional whole number) widget — eliminates awkward slash typing for young learners.
6. **Interactive Visual / Drag-and-Drop (Enhancement)**: Drag blocks onto a scale, shade fraction bars, plot points on a number line, or sort shapes/numbers into bins (serialized cleanly as deterministic JSON answers).

### Canonical Bilingual Question JSON Schema (`apps/api/internal/mathengine`)
```json
{
  "id": "AP-G1-ADD-0042",
  "version": 1,
  "grade": 1,
  "skillId": "sg-p1-add-within-20",
  "difficulty": 2,
  "heuristicTag": null,
  "inputType": "multi_numeric",
  "locales": {
    "en": {
      "stem": "Rina has {{a}} blue stars and {{b}} gold stars. How many stars does she have altogether?",
      "ttsText": "Rina has {{a}} blue stars and {{b}} gold stars. How many stars does she have altogether?",
      "hints": ["Count on from the larger number.", "Use the number line to add {{a}} and {{b}}."],
      "solutionSteps": [
        { "text": "Start at {{a}} on the number line and jump forward {{b}} steps." },
        { "text": "{{a}} + {{b}} = {{ans}}", "diagramRef": "number_line_add" }
      ],
      "parts": [{ "key": "total", "prefix": "", "suffix": "stars" }]
    },
    "id": {
      "stem": "Rina memiliki {{a}} bintang biru dan {{b}} bintang emas. Berapa jumlah bintang Rina semuanya?",
      "ttsText": "Rina memiliki {{a}} bintang biru dan {{b}} bintang emas. Berapa jumlah bintang Rina semuanya?",
      "hints": ["Hitung maju dari bilangan yang lebih besar.", "Gunakan garis bilangan untuk menjumlahkan {{a}} dan {{b}}."],
      "solutionSteps": [
        { "text": "Mulai dari angka {{a}} pada garis bilangan lalu lompat maju {{b}} langkah." },
        { "text": "{{a}} + {{b}} = {{ans}}", "diagramRef": "number_line_add" }
      ],
      "parts": [{ "key": "total", "prefix": "", "suffix": "bintang" }]
    }
  },
  "media": {
    "diagramType": "svg_template",
    "diagramId": "number_line_add",
    "videoTutorialId": "vid-g1-add-count-on"
  },
  "grading": {
    "strategy": "exact_numeric",
    "answers": { "total": "{{ans}}" },
    "tolerance": 0
  }
}
```

---

## 3. Hybrid Question Bank: Static + Deterministic Procedural Generator

To ensure elementary students never run out of fresh practice questions — and to power unlimited *"Change Question"* and *Star Duels* — the server-authoritative Go math engine (`apps/api/internal/mathengine/`) combines:
- **Zero Client/Server Duplication**: Go is the single source of truth for procedural question generation and grading. The API sends rendered bilingual question payloads (stems, input slots, SVG diagram parameters — **never answer keys**) to the React frontend; Admin CMS previews call `POST /api/v1/admin/questions/preview`.

1. **Curated Static Questions**: Hand-crafted, illustrated multi-step word problems, HOTS heuristics, Top School exam items, and Real-World Explorer problems stored in PostgreSQL and authored via Admin CMS.
2. **Deterministic Seeded Procedural Templates**:
   - Each template defines parameter ranges, constraints (e.g., `a + b <= 20`, `dividend % divisor === 0`, `a > b` for no negative answers in G1–G5), culturally neutral bilingual names, SVG diagram parameters, and computed step-by-step worked solutions.
   - Given `(templateId, seed)`, the generator produces the **exact same question, diagram, and worked solution** on both backend (for secure grading) and frontend (for rendering & review) — requiring only `(templateId, seed)` to be stored per attempt!
   - **Equivalence Grader**:
     - Normalizes whitespace and leading zeros.
     - Accepts both `.` and `,` decimal separators appropriately based on locale rules (while preventing ambiguity on thousands separators).
     - Supports simplified or equivalent fraction checks (configurable `requireSimplified: true/false` per skill).
     - Supports order-independent multi-box answers where applicable (e.g., *"List any two factors of 12"*).

---

## 4. The 4 Interactive On-Screen Math Working Tools

Docked as a sleek right-side vertical toolbar on tablets/desktops (and slide-up bottom sheet on smartphones), available inside the Question Player without leaving the question:

### Tool 1: Star Sketchpad (Freehand Scratchpad Overlay)
- Transparent or toggleable dotted-grid HTML5 Canvas overlay above/beside the question.
- Controls: **Pencil** (3 stroke widths, 4 high-contrast inks), **Highlighter** (translucent yellow/cyan), **Eraser**, **Undo / Redo**, **Clear All**.
- Supports mouse, finger touch, and stylus/Apple Pencil with palm-rejection friendly `pointerdown/move/up` events.

### Tool 2: Equation Pad (*Add Working*)
- Step-by-step structured math scratchpad where students can type working lines (`Line 1`, `Line 2`, ...) using a kid-friendly **Math Symbol Pad**: `+`, `−`, `×`, `÷`, `=`, `(`, `)`, `<`, `>`, fraction button, `°`, `cm`, `m`, `kg`, `%`.
- Helps students organize multi-step word problem calculations cleanly (and optionally save working steps for parent/teacher review).

### Tool 3: Bar Model Builder (Singapore Math Visual Tool)
- Interactive horizontal bar-model composer specifically designed for Grades 1–6 word problems (part-whole, comparison, before-after, fraction/ratio units):
  - Add 1–4 named rows (e.g., *Ali*, *Budi*).
  - Adjust discrete equal **units from `1` to `20`** per bar via `+ / −` or slider.
  - Split/shade selected units in color (for fractions/percentages).
  - Add top/bottom **brace labels** (`?`, `24`, `3 units`) to visualize unknown quantities.
  - Reset / Undo buttons.

### Tool 4: Column 4-Operations Grid (*Four Operations Tool*)
- Structured vertical column workspace supporting **`+`**, **`−`**, **`×`**, and **`÷` (long division)** up to **4 digits**.
- Provides aligned place-value cells (**Th, H, T, O**) with dedicated small boxes for **carry-over** and **borrowing/regrouping** scratch marks.
- **Pedagogical Rule**: This is a *manual guided workspace*, NOT an auto-answering calculator — students enter their own digits so it reinforces mental calculation and standard vertical algorithms!

---

## 5. Audio (TTS), Video Tutorials & Visual Worked Solutions

- **Text-to-Speech (TTS)**:
  - Clicking the Speaker button reads the question stem aloud in the active locale (`en` or `id`).
  - Uses pre-synthesized cached neural TTS audio (Phase 4 CDN pipeline) with instant Web Speech API fallback in Phase 1.
  - Math verbalizer converts symbols before synthesis (e.g., `3/4` → `"three quarters"` / `"tiga per empat"`, `×` → `"times"` / `"dikali"`).
- **Video Tutorial Modal**:
  - Opens a clean, distraction-free embedded player (self-hosted HLS/MP4 via CDN or curated privacy-enhanced YouTube embed with `rel=0`) matched to the skill ID and locale.
- **Visual Worked Solutions**:
  - Supports multi-step collapsible/stepped explanations combining plain bilingual text, aligned equations, and programmatic SVG visual aids (number lines, place-value charts, bar models, fraction circles, clock faces, geometry labels).

---

## 6. Printable Worksheet Renderer (`AstroQuiz` Print Mode)

- Route: `/[locale]/astro-quiz/print/[paperId]`
- Dedicated print stylesheet (`@media print`) and optional server-side PDF export:
  - Strips all dark galaxy backgrounds, nav bars, and interactive buttons.
  - Renders crisp black-on-white A4 header (AlgiPlanet light logo, Student Name, Class, Date, Paper Title, Score Box `___ / 20`).
  - Uses `break-inside: avoid` on every question card so diagrams never split across pages.
  - Reserves clean lined/blank working space beneath each question and includes an optional parent/teacher **Answer Key** toggle on the final page.
