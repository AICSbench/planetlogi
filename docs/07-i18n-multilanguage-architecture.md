# 07 — Multi-Language & Localization Architecture (`en` & `id` First)

> **Owner Tracks**: `SHARED`, `FE`, `BE`, `CONTENT`  
> **Scope**: End-to-end bilingual architecture launching with **English (`en`)** and **Indonesian (`id`)**, architected cleanly so future locales (e.g., `zh-CN`, `ms`, `th`, `vi`) plug in without code rewrites.

---

## 1. Localization Pillars

PlanetLogi treats localization across **5 distinct layers** — UI translation alone is not enough for an elementary math platform:

1. **UI Chrome & Copy**: Navigation, buttons, modals, tooltips, error messages, notifications, emails, and gamification labels.
2. **Curriculum & Math Content**: Topic titles, skill names, question stems, hints, step-by-step worked solutions, HOTS heuristic names, and Self-Check objectives.
3. **Math Notation, Units & Currency Conventions**: Number formatting, decimal input tolerance, and culturally appropriate word-problem units/currency.
4. **Spoken Audio (TTS)**: Natural child-friendly read-aloud voices (`en` & `id`) with math-symbol verbalization.
5. **Rich Media**: Planet Stories 20-page narration/text overlays, Space Academy video tracks/subtitles, and Daily Star Badge trivia cards.

---

## 2. Routing, Locale Persistence & Resolution

- **URL Prefix Routing**: All web routes are locale-prefixed via TanStack Router (`apps/web/src/routes/$locale/...`), supporting `/en/...` and `/id/...` with **`i18next` (`react-i18next`)** bundled cleanly for static Cloudflare Pages deployment (`/* /index.html 200` SPA fallback).
- **Locale Resolution Priority**:
  1. Explicit URL prefix (`/en/` or `/id/`)
  2. Logged-in user's saved `preferredLocale` (from Parent/Student Settings)
  3. Cookie (`NEXT_LOCALE`)
  4. Browser `Accept-Language` header (`id*` → `id`, default fallback → `en`)
- **Seamless Switching**: Changing language in Settings or Login Screen updates the cookie, persists `PATCH /api/v1/profile/settings`, and preserves the user's current route without losing in-progress form state.

---

## 3. Conflict-Free Namespaced Translation Files

To enable multiple parallel frontend agents to add UI copy without merge conflicts, translation bundles in `packages/i18n/locales/` are split by domain namespace:

```text
packages/i18n/
├── config.ts                 # Supported locales: ['en', 'id'], defaultLocale: 'en'
├── scripts/check-i18n.ts     # CI validator: fails build on missing/extra keys or broken ICU vars
└── locales/
    ├── en/
    │   ├── common.json       # Nav, buttons, currencies, errors, dates
    │   ├── auth.json         # Login, OTP, password reset
    │   ├── dashboard.json    # 4-pillar hub, XP ranks, notifications
    │   ├── daily-orbit.json  # Daily Orbit, tiers, streaks, Cryo Shields
    │   ├── training.json     # Curricula, HOTS wheel, mastery tiers
    │   ├── player.json       # Question player, 4 math tools, feedback
    │   ├── academy.json      # Space Academy, Do & Learn, Self-Check
    │   ├── stories.json      # Planet Stories, 6 world names, reader controls
    │   ├── astro-quiz.json   # Exam categories, Supply Pods, print headers
    │   ├── social.json       # Star Duels, Star Crew, Leaderboards, Events
    │   ├── brain-galaxy.json # 5 zones, 25-min healthy cap notices
    │   ├── portals.json      # Parent, Teacher, Admin CMS copy
    │   └── billing.json      # Pricing, checkout, subscription management
    └── id/
        └── (exact mirror of all JSON namespaces above)
```

- **ICU MessageFormat** is mandatory for plurals, counts, and interpolation (e.g., `{count, plural, =1 {1 Star Point} other {# Star Points}}` in `en`, `{count} Poin Bintang` in `id`).
- **UI Layout Resilience**: Indonesian copy is typically **15–25% longer** than English. All buttons, nav pills, and badges MUST allow wrapping or responsive padding without clipping at `375px` mobile widths.

---

## 4. Bilingual Math Content & Notation Rules

### 4.1 Database Localization Pattern
- Dynamic content tables (`Question`, `Skill`, `Topic`, `StoryChapter`, `BadgeTrivia`, `Event`) store localized fields in a structured JSONB column (`translations: Record<LocaleCode, LocalizedPayload>`) validated by Zod schemas on write, indexed with GIN if full-text search is needed.
- **Publish Gate**: A Question, Lesson, or Story CANNOT transition from `IN_REVIEW` → `PUBLISHED` in Admin CMS unless **both `en` and `id`** payloads pass schema validation.
- **Runtime Fallback**: If a newly added future locale (e.g., `ms`) lacks a translation for a specific record, the API falls back gracefully to `en` and attaches `fallbackLocaleUsed: "en"` metadata for CMS tracking.

### 4.2 Math Number, Decimal & Currency Localization
| Convention | English (`en`) | Indonesian (`id`) | Grader Behavior |
|---|---|---|---|
| **Decimal Display** | `3.5` | `3,5` | **Grader accepts BOTH `3.5` and `3,5`** for single-decimal answers so students using either mobile keyboard are never penalized! |
| **Thousands Separator** | `1,250` or `1 250` | `1.250` or `1 250` | Display uses locale-appropriate separator; strip spaces; disambiguate via `inputType` (`integer` vs `decimal`) |
| **Currency in Word Problems** | `$12` (or `12 dollars`) | `Rp12.000` (localized realistic denominations via template multiplier!) | Procedural templates define locale-specific currency multipliers (e.g., `$1` ↔ `Rp10.000` or `Rp5.000`) so Indonesian money problems feel natural |
| **Units** | Metric (`cm`, `m`, `kg`, `l`, `ml`) across both `en` & `id` | Metric (`cm`, `m`, `kg`, `l`, `ml`) | Consistent metric standard across Kurikulum Merdeka, Singapore MOE, and Cambridge |
| **Character Names** | Culturally inclusive names (*Rina, Budi, Maya, Lucas, Siti, Ethan*) | Same inclusive names | Shared cleanly across both locales |

---

## 5. Bilingual Text-to-Speech (TTS) & Video Strategy

### 5.1 TTS Read-Aloud Pipeline
- **Phase 1 (Immediate)**: Browser `window.speechSynthesis` selecting `en-US`/`en-GB` or `id-ID` voices with gentle speed (`rate: 0.92` for Grades 1–2, `1.0` for Grades 3–6).
- **Phase 4 (Production Neural TTS)**: Server endpoint `POST /api/v1/tts/synthesize` hashes `(locale, normalizedTtsText, voiceId)` → serves cached MP3/Opus audio from S3/R2 CDN.
- **Math Verbalizer** (`apps/api/internal/mathengine/tts_verbalizer.go`, mirrored helper for browser fallback): Converts symbols prior to speech synthesis:
  - `+` → `"plus"` (`en`) / `"ditambah"` (`id`)
  - `−` → `"minus"` (`en`) / `"dikurangi"` (`id`)
  - `×` → `"times"` (`en`) / `"dikali"` (`id`)
  - `÷` → `"divided by"` (`en`) / `"dibagi"` (`id`)
  - `=` → `"equals"` (`en`) / `"sama dengan"` (`id`)
  - `3/4` → `"three quarters"` (`en`) / `"tiga per empat"` (`id`)

### 5.2 Space Academy Videos & Planet Stories
- **Videos**: Support dual audio tracks or separate `videoUrl.en` / `videoUrl.id` with mandatory WebVTT closed captions (`en.vtt` & `id.vtt`) for noisy classrooms or hearing accessibility.
- **Planet Stories**: Illustrations stay completely **text-free** inside the artwork images; dialogue bubbles and narration panels render dynamically as crisp HTML/SVG overlays per locale so one artwork set powers unlimited languages!

---

## 6. Translation QA Workflow

1. **Automated CI Gate (`pnpm i18n:check`)**: Runs on every PR to verify 100% key parity between `locales/en/*.json` and `locales/id/*.json` and valid ICU syntax.
2. **Pedagogical Terminology Glossary**: Maintained in `packages/i18n/GLOSSARY.md` so math terms stay consistent across all agents and content editors (e.g., *Fraction* = *Pecahan*, *Remainder* = *Sisa*, *Perimeter* = *Keliling*, *Area* = *Luas*, *Bar Model* = *Model Batang*, *Place Value* = *Nilai Tempat*, *LCM/GCF* = *KPK/FPB*).
