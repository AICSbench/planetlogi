# 06 — Parent Portal, Teacher/School Portal & Admin Content CMS

> **Owner Tracks**: `BE`, `FE`, `ADMIN_CMS`  
> **Scope**: Complete functional specifications for the three supporting portals that power the AlgiPlanet ecosystem alongside the Student Portal.

---

## 1. Parent Portal (`/parent/*`)

Designed for busy parents on both **mobile phones** and desktops to monitor progress, customize homework, and manage healthy digital boundaries in under 3 minutes.

### 1.1 Multi-Child Family Dashboard (`/parent/dashboard`)
- **Child Switcher**: Seamlessly toggle between multiple linked children (Grades 1–6) under one Parent account.
- **Weekly Snapshot Cards**:
  - Daily Orbit Completion (`X / 7 days this week`) & Current Orbit Streak + Cryo Shields left.
  - Questions Solved, Overall Accuracy %, and Total Study Time.
  - **Top 3 Strengths** (*Mastery 80–100%*) and **Top 3 Focus Areas** (*Beginning 0–59%*).
- **One-Click Action**: *"Assign Practice on Focus Areas"* pre-fills the Personalised Assignment Builder with questions targeting the child's weakest skills.

### 1.2 Detailed Skill & High-Score Reports (`/parent/reports`)
- Mirrors and expands the student's **Proficiency Report** and **High Score Report** across all 6 curricula and Grades 1–6.
- Includes **Mistake Review Drawer**: Parents can inspect recent incorrectly answered questions alongside the child's submitted answer and the visual worked solution so they can coach their child effectively at home.
- Weekly automated bilingual **Email / Push Progress Digest** (opt-in).

### 1.3 Personalised Assignment Builder (`/parent/assignments/new` & `/assignments/parent-assign`)
- Step 1: Select Child, Curriculum Track, and Grade Level.
- Step 2: Browse Topics & Skills displaying the child's live **Proficiency % badge** (`Mastery` / `Competent` / `Beginning` / `Incomplete`) right next to each skill.
- Step 3: Expand a skill to **preview and hand-pick specific questions** (or click *"Auto-Pick 5 Questions"*), see difficulty flames & bilingual preview.
- Step 4: Set Assignment Title, Due Date, and optional encouragement note → Click **Assign**.
- Child receives an immediate notification bell alert and sees the task under **My Assignments → Assigned by: Parents**.

### 1.4 Family Controls, Security & Billing (`/parent/settings`, `/parent/billing`)
- **OTP Security Toggle**: Protect sensitive account & setting changes with 6-digit OTP verification.
- **Language & Curricula**: Set child's default language (`en` / `id`), active Daily Orbit curricula, and Star Duel curricula.
- **Screen-Time & Healthy Limits**: Adjust Brain Galaxy daily cap (`0–25 min`), operating hours window, bedtime lock, and study reminder time.
- **Child Profile & Grade Override**: Edit name, birthday, school, reset child password, or override grade level.
- **Subscription & Invoices**: Manage Free/Premium plans, view payment receipts, and handle account data export / deletion requests.

---

## 2. Teacher & School Portal (`/teacher/*` — Phase 4 B2B)

Enables elementary schools (SD / Primary Schools) to deploy AlgiPlanet across classes Grades 1–6.

### 2.1 School & Class Management (`/teacher/classes`)
- **Role Hierarchy**: `SCHOOL_ADMIN` (manages all grades/teachers/licenses in school) and `TEACHER` (manages assigned classes).
- **Roster Onboarding**:
  - Add students individually OR **CSV Bulk Import Wizard** (template columns: `FirstName`, `ClassName`, `Grade`, `PreferredLocale`, `ParentEmailOptional`) with row-level validation preview.
  - Auto-generates kid-friendly unique Login IDs & temporary passwords + **Printable Student Login Slips (A4 PDF)** with QR code login shortcuts for easy classroom handout.
- **Annual Automatic Grade Rollover**:
  - Configurable academic rollover date per school (default July 1) with pre-rollover preview email for School Admins and one-click class reassignment.

### 2.2 Class Analytics & Mastery Heatmap (`/teacher/analytics`)
- **Class Proficiency Matrix**: Rows = Students, Columns = Curriculum Topics/Skills, Cells color-coded by Mastery (`80–100%`), Competent (`60–79%`), Beginning (`0–59%`), Incomplete.
- Instantly highlights *"Skills where >40% of the class is struggling"* to inform next day's classroom teaching.
- Exportable **CSV / PDF Class Progress Report** for parent-teacher conferences.

### 2.3 Bulk Assignments & Class Leaderboards (`/teacher/assignments`, `/teacher/leaderboard`)
- Create assignments targeting an entire **Class**, a **Custom Student Group** (e.g., Remediation Group vs Enrichment Group), or individual students.
- Track real-time completion status (`24 / 30 submitted`), average score, and per-question error rates.
- View Class & School leaderboards (Champion Class, Hero of the Day, Orbit Streak) and optionally pause/resume class leaderboard visibility during exam weeks.

---

## 3. Admin & Bilingual Content CMS (`/admin/*`)

Internal operations portal (`ADMIN` & `CONTENT_EDITOR` roles) allowing non-engineers to scale AlgiPlanet's bilingual library and live operations safely.

### 3.1 Bilingual Question & Procedural Template Studio (`/admin/questions`)
- **Side-by-Side `en` / `id` Editor**: Edit stem, hints, TTS pronunciation overrides, input boxes, and step-by-step worked solutions simultaneously.
- **Live Question Player Preview**: Renders the exact student view (including mobile/tablet toggle and interactive grading test) before publishing.
- **Procedural Template Sandbox**: Test generator constraints across 50 random seeds to verify zero invalid numbers, zero divide-by-zero errors, and crisp diagram rendering.
- **Workflow States**: `DRAFT` → `IN_REVIEW` (Pedagogy + Bilingual QA) → `PUBLISHED` → `ARCHIVED` (versioned so historical student attempts never break).

### 3.2 Curriculum, Academy, Quiz & Story Managers (`/admin/content/*`)
- **Curriculum Tree Manager**: Map canonical skills into the 6 curriculum tracks, reorder topics, set difficulty (`1–5`), and attach video tutorial links.
- **Space Academy Manager**: Compose Topics and 4-step Lessons (*Story Launch*, *Watch & Learn*, *Do & Learn* question sets, bilingual *Self-Check* checklists).
- **AstroQuiz Paper Builder**: Assemble 15–20 question revision/exam/HOTS papers, set weekly Supply Pod deadlines, and test A4 Print preview.
- **Planet Stories Manager**: Upload ordered ~20-page illustrations per chapter, author bilingual page captions/narration, and configure unlock cost (`0 AC` for Opening Chapter, `20 AC` for subsequent chapters).
- **Super Sight & Badge Trivia Manager**: Upload spot-the-difference image pairs with visual bounding-box editor (5 target zones) and manage the 365-day bilingual Daily Star Badges trivia calendar.

### 3.3 Live Ops, Events & Moderation (`/admin/ops/*`)
- **Event Scheduler**: Configure seasonal leaderboard events (*Zodiac Warp*, *Comet vs Meteor*, *Star Academy Cup*, *Galactic City Race*), Sunday Meteor Challenge question pools, and Quarterly Creator Challenges.
- **Student Feedback Triage Queue**: Lists every *"Report / Feedback"* ticket submitted from the Question Player (linked directly to Question ID, seed, locale, and student comment) with status workflow (`OPEN` → `FIXED` → `DISMISSED`).
- **Support & Audit Logs**: Impersonation-free read-only support diagnostics, subscription adjustment logs, and immutable audit trail of all Admin/CMS actions.
