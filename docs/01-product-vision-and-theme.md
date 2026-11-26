# 01 — Product Vision, Personas & Galaxy Design System

> **Owner Tracks**: `UI`, `FE`, `PRODUCT`  
> **Scope**: Product identity, target elementary personas (Grades 1–6), AlgiPlanet Galaxy theme & visual design system, responsive/PWA rules, and accessibility standards.

---

## 1. Product Vision & Mission

**AlgiPlanet** empowers every elementary school student (**Grades 1–6 / Primary 1–6, ages 6–12**) to develop lasting mathematical fluency, visual problem-solving intuition, and higher-order thinking skills through **20 minutes of joyful daily practice** — guided by a rich **Planet & Galaxy expedition theme** and built natively for **English (`en`)** and **Indonesian (`id`)** learners.

### Core Product Principles
1. **Habit Over Cramming**: Short daily challenges (*Daily Orbit*) with gentle streak mechanics (*Nova Pet* & *Cryo Shields*) build intrinsic daily study habits.
2. **Visual & Conceptual Mastery**: Combine procedural drill fluency with bar-modeling, interactive working tools, video explanations, and 9 HOTS heuristics.
3. **Healthy Digital Boundaries**: Built-in study/play operating hours, a strict 25-minute/day cognitive mini-game cap, and earn-to-play economics (*Astro Coins* earned via math unlock stories and games).
4. **Clarity for Focus, Wonder for Reward**: Outer navigation and hubs immerse children in a vibrant cosmic galaxy; the active **Question Player** transitions to a calm, high-contrast, glare-free focus canvas so math stays effortless to read and solve.
5. **Bilingual Parity**: English and Indonesian are co-primary languages from day one — never second-class machine translations.

---

## 2. Target Personas (Grades 1–6 Ecosystem)

### 2.1 Lower Elementary Student ("Space Cadet" — Grades 1–2, Ages 6–8)
- **Characteristics**: Developing reading stamina, uses tablet/iPad frequently, motivated by colorful companions, sound effects, stars, and immediate praise.
- **Key UX Needs**: Large touch targets (`>=48px`), prominent **TTS Read-Aloud** button on every question, visual diagrams/number lines, clear step-by-step worked solutions, low-friction number inputs.

### 2.2 Upper Elementary Student ("Star Navigator" — Grades 3–6, Ages 9–12)
- **Characteristics**: Confident reader, solving multi-step word problems, fractions, ratios, geometry, and Olympiad/HOTS heuristics; motivated by levels, peer duels, leaderboards, and custom avatars/buddies.
- **Key UX Needs**: Fast keyboard + touch input, on-screen **Bar Model Builder**, **Scratchpad**, **Equation Pad**, **Column 4-Operations Grid**, printable **AstroQuiz** exam papers, and national/school leaderboards.

### 2.3 Parent ("Mission Control")
- **Characteristics**: Wants measurable improvement in school exams (Kurikulum Merdeka, Singapore Math, or Cambridge) without constant nagging or excessive screen time.
- **Key UX Needs**: At-a-glance **Proficiency %** heatmap, ability to assign tailored homework on weak skills in 2 minutes, language & curriculum toggles, OTP-locked settings, and screen-time guardrails.

### 2.4 Teacher / School Admin ("Fleet Commander" — Phase 4 B2B)
- **Characteristics**: Manages 25–40 students per class across Grades 1–6; needs bulk onboarding, quick homework distribution, and class-wide mastery diagnostics.
- **Key UX Needs**: CSV roster upload, printable student login slips, class mastery matrix, bulk assignments, and automatic annual grade rollover.

---

## 3. AlgiPlanet Galaxy World & Theme Architecture

Each elementary grade corresponds to a distinct **Planet** in the AlgiPlanet solar system, unifying Storybooks (*Planet Stories*), Dashboard banners, Badges, and Progression milestones:

| Grade | Planet Name (EN) | Planet Name (ID) | Visual Motif & Palette Accent |
|---|---|---|---|
| **Grade 1 (P1)** | **Terra Pebble** | **Planet Kerikil** | Meadow boulders, cozy launchpad, emerald green & warm amber |
| **Grade 2 (P2)** | **Planet Tropica** | **Planet Tropika** | Floating rainforest islands, bioluminescent vines, teal & lime |
| **Grade 3 (P3)** | **Planet Aquaris** | **Planet Akuaris** | Crystal oceans, coral domes, submarines, sapphire & aqua |
| **Grade 4 (P4)** | **Planet Crystalia** | **Planet Kristalia** | Amethyst caverns, prism bridges, violet & magenta |
| **Grade 5 (P5)** | **Planet Lumina** | **Planet Lumina** | Neon star-city, orbital railways, electric cyan & gold |
| **Grade 6 (P6)** | **Planet Solaris** | **Planet Solaris** | Citadel of Light, golden rings, solar gold & royal indigo |

### Thematic Metaphors Mapped to Mechanics
- **Orbit Path**: Daily Orbit's 10-question progression path connects glowing moons (Q1–Q5 circles), asteroids (Q6–Q8 hexagons), and supernovas (Q9–Q10 stars).
- **Nova Pet & Orbit Streak**: A cosmic companion creature that hatches and evolves visually as the student sustains their daily 5-correct-answer streak.
- **Cryo Shields**: Crystalline frost shields that automatically preserve the Orbit Streak if a day is missed (max 3 held).
- **Supply Pods**: Weekly treasure capsules awarded for completing AstroQuiz papers on time.
- **Star Scrolls**: Glowing celestial scrolls earned from completing Space Academy *Do & Learn* guided lessons.

---

## 4. Visual Design System & Tokens

### 4.1 Color System (Dual-Surface Strategy)
AlgiPlanet uses a **Cosmic Chrome + Crisp Study Surface** dual strategy: rich deep-space surfaces for exploration/gamification, and high-contrast light/warm-paper surfaces inside the Question Player to minimize eye strain during math reading and scratchpad drawing.

| Token | Hex | Usage |
|---|---|---|
| `--space-950` | `#0B1026` | Deep space app background (hubs, leaderboards, stories) |
| `--space-900` | `#131B3A` | Top navigation bar, dark cards, modal backdrops |
| `--space-800` | `#1E2A55` | Elevated cosmic panels, interactive card surfaces |
| `--nebula-500` | `#6366F1` | Primary brand accent, active tabs, focus rings |
| `--aurora-400` | `#22D3EE` | Secondary cosmic highlight, XP bars, Cryo Shields |
| `--starlight-400` | `#FBBF24` | Star Points (SP), Astro Coins (AC), stars, gold badges |
| `--nova-500` | `#F97316` | Orbit Streak flame, difficulty meter active icons |
| `--canvas-paper` | `#FAFBFD` | Question Player main study surface (high readability) |
| `--canvas-ink` | `#111827` | Primary math text & equations on study surface (`>14:1` contrast) |
| `--status-mastery` | `#10B981` | Mastery (80–100%), Correct feedback, Star Scrolls |
| `--status-competent` | `#3B82F6` | Competent (60–79%), informational badges |
| `--status-beginning` | `#F59E0B` | Beginning (0–59%), hints, caution states |
| `--status-error` | `#EF4444` | Incorrect answer highlight, validation alerts |
| `--status-incomplete` | `#94A3B8` | Incomplete (<10 questions) neutral indicator |

### 4.2 Typography
- **Display / Headings**: `Fredoka` or `Plus Jakarta Sans` (friendly geometric rounded curves, clear numerals, full Indonesian diacritic support).
- **Body & UI**: `Plus Jakarta Sans` or `Nunito` (highly legible for ages 6–12).
- **Math & Numerals**: Tabular lining figures (`font-variant-numeric: tabular-nums`) with unmistakable glyphs (`1` vs `l` vs `I`, `0` vs `O`, clear fraction bars via KaTeX/MathML where needed).
- **Base Scale**: Minimum `16px` body text; `18px–22px` for Grade 1–3 math question stems.

### 4.3 Core Reusable Components (`packages/ui`)
- **Cosmic Shell & Top Bar** (full expansion `>=1440px`, 3-dot overflow menu `<1440px`).
- **Pillar Tab Switcher** (4 cosmic tabs with icon + badge counts).
- **Difficulty Meter** (5-flame/comet intensity icons, accessible `aria-label="Difficulty 3 of 5"`).
- **Star Rating Badge** (0–3 animated stars).
- **Proficiency Pill** (4 color-coded tiers + tooltip).
- **Currency Pill** (Star Points `SP`, Astro Coins `AC`, Diamond `S`, Star Scrolls with animated increment counters).
- **Celebration Layer** (lightweight canvas confetti + optional muteable chime on correct answer / level-up; respects `prefers-reduced-motion`).

---

## 5. Responsive Web + PWA Specifications

### Breakpoint Strategy
| Viewport | Target Device | Layout Behavior |
|---|---|---|
| `375px – 639px` | Smartphones | Single column; bottom drawer for 4 Math Tools; compact top bar with 3-dot overflow & 9-dot launcher |
| `640px – 1023px` | Tablets Portrait (iPad) | 2-column cards; collapsible left profile sidebar; touch-optimized Question Player |
| `1024px – 1439px` | Tablets Landscape & Laptops | Full dashboard 3-zone grid; right-docked collapsible Math Tools panel; 3-dot overflow for secondary nav items |
| `>= 1440px` | Wide Desktops | Full top navigation bar with zero overflow; simultaneous Question Side Nav + Question Canvas + Math Tools |

### PWA Capabilities
- **Installable Manifest**: Standalone display mode, galaxy app icon set (`192x192`, `512x512`, maskable), deep-space splash screen (`#0B1026`).
- **Caching Strategy**: Cache-first for static assets/SVG icons/UI bundles; Network-first with graceful offline notice for live APIs; local draft persistence (`IndexedDB`) for saved *Space Academy* and *AstroQuiz* progress.
- **Touch & Stylus Polish**: `touch-action: none` on Scratchpad canvas; prevent accidental pinch-zoom during math tool interaction; Apple Pencil & Chromebook touchscreen support.

---

## 6. Accessibility & Inclusive Kid UX

- **WCAG 2.1 AA Compliance**: All text meets `>=4.5:1` contrast (`>=3:1` for large text/icons).
- **Color-Blind Safe Indicators**: Never rely on color alone for Proficiency tiers, Correct/Incorrect states, or Difficulty meters — always pair color with distinct icons (`✓`, `✕`, stars, labels).
- **Reduced Motion**: Honor `prefers-reduced-motion` by replacing confetti/orbital star animations with subtle fade feedback.
- **Audio Controls**: Global mute toggle for SFX + dedicated per-question **TTS Read-Aloud** button so students can listen without background noise distractions.
- **Keyboard Navigation**: Full tab order, Enter-to-submit, Escape-to-close modals, and arrow navigation inside Planet Stories and Question Side Nav.
