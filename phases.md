# StitchMap — Implementation Phases

> **Status**: The initial implementation (Phases 1–7) is complete as of 2026-02-20. These phase descriptions are preserved for historical reference. Active work is tracked in the **Planned Improvements** section of [CLAUDE.md](./CLAUDE.md).

Each phase was independently implementable, testable, and deployable. Each phase had to pass all existing tests before proceeding to the next.

---

## Phase 1: Project Scaffolding & Infrastructure

**Goal**: Bootable Go application with database connectivity, migrations, health endpoint, and CI pipeline.

**Deliverables**:
- `go.mod` initialized with module path
- `main.go` with HTTP server startup using `net/http`
- `domain/db.go` — `Database` interface (`Migrate`, `Close`) so the entire DB backend is swappable
- SQLite `DB` struct implementing `domain.Database` via `database/sql` + `modernc.org/sqlite`
- Custom migration runner owned by the SQLite implementation: reads `.sql` files from `embed.FS`, tracks applied migrations in a `schema_migrations` table, applies unapplied migrations in filename order via `Database.Migrate()`
- Migration 001 (users table) applied on startup
- Health check endpoint: `GET /healthz` returning 200
- Structured logging via `log/slog`
- GitHub Actions CI workflow running build + test + vet
- Base Templ layout with Bulma CSS + Datastar CDN includes
- Configuration via environment variables (PORT, DATABASE_PATH)

**Tests**:
- Database connection and migration execution
- Health endpoint returns 200
- Server starts and shuts down gracefully

**Regression Gate**: CI passes build, test, vet.

---

## Phase 2: User Authentication (JWT)

**Goal**: Users can register, log in, log out, and access protected routes via JWT tokens.

**Deliverables**:
- Migration 001 for users table (applied in Phase 1)
- `domain/user.go` — User entity and UserRepository interface
- `repository/sqlite/user.go` — SQLite UserRepository implementation
- `service/auth.go` — Registration (with validation, duplicate check), login (bcrypt verify, JWT generation), logout (cookie clearing)
- `handler/auth.go` — Register, login, logout HTTP handlers with Datastar SSE responses
- `handler/middleware.go` — Auth middleware that reads JWT from `auth_token` cookie, validates signature and expiration, loads user from DB, injects into context
- `view/auth.templ` — Login and register forms using Bulma styling
- `view/layout.templ` — Navbar with conditional auth state (login/register vs. user menu)

**Tests**:
- Unit: UserRepository CRUD operations
- Unit: Auth service — register (success, duplicate email, weak password), login (success, wrong password, unknown email), JWT generation and validation
- Unit: Auth middleware — valid JWT, expired JWT, missing cookie, tampered token
- Integration: Full registration -> login -> access protected route -> logout flow

**Regression Gate**: All Phase 1 + Phase 2 tests pass. CI green.

---

## Phase 3: Stitch Library

**Goal**: Predefined stitch abbreviations are seeded and browseable. Users can create custom stitches.

**Deliverables**:
- Migration 002 for stitches table
- `domain/stitch.go` — Stitch entity and StitchRepository interface
- `repository/sqlite/stitch.go` — SQLite StitchRepository implementation
- `service/stitch.go` — List predefined, list user custom, create/update/delete custom, seed predefined stitches
- Seed function that inserts all predefined stitches (idempotent — skip if already exist)
- `handler/stitch.go` — List, create, update, delete handlers with Datastar SSE
- `view/stitch_library.templ` — Stitch library page with category filtering, search, custom stitch CRUD

**Tests**:
- Unit: StitchRepository CRUD
- Unit: Seed function idempotency
- Unit: Custom stitch creation (success, duplicate abbreviation for same user, reject reserved abbreviation)
- Unit: Stitch service — list combined predefined + custom for a user
- Integration: Browse library, create custom stitch, see it appear, edit it, delete it

**Regression Gate**: All Phase 1-3 tests pass. CI green.

---

## Phase 4: Pattern Builder (Core)

**Goal**: Users can create patterns with instruction groups and stitch entries.

**Deliverables**:
- Migration 003 for patterns, instruction_groups, stitch_entries tables
- `domain/pattern.go` — Pattern, InstructionGroup, StitchEntry entities; PatternRepository interface
- `repository/sqlite/pattern.go` — SQLite PatternRepository implementation (with nested creates/updates within transactions)
- `service/pattern.go` — Pattern CRUD, validation (non-empty groups, valid stitch references), stitch count computation
- `handler/pattern.go` — List, create, read, update, delete handlers
- `view/pattern_list.templ` — Pattern card grid with actions
- `view/pattern_editor.templ` — Full pattern builder:
  - Pattern metadata fields (name, description, type, hook size, yarn weight, notes)
  - Add/remove/reorder instruction groups
  - Within each group: add/remove/reorder stitch entries
  - Stitch picker dropdown populated from predefined + user custom stitches
  - Count, into-stitch, repeat count, notes fields per stitch entry
  - Expected stitch count per group
  - All add/remove/reorder operations via Datastar SSE (no full page reloads)

**Tests**:
- Unit: PatternRepository CRUD (create with nested groups/entries, update, delete cascades)
- Unit: Pattern service validation — reject empty pattern, reject invalid stitch ID, require at least one group
- Unit: Stitch count calculation — simple counts, repeats, group repeats
- Integration: Create pattern -> add groups -> add stitches -> save -> reload -> verify persistence

**Regression Gate**: All Phase 1-4 tests pass. CI green.

---

## Phase 5: Pattern Preview & Text Rendering

**Goal**: Patterns can be viewed as formatted text using standard crochet notation.

**Deliverables**:
- Pattern-to-text renderer that outputs standard crochet notation:
  - `Round 1: MR, 6 sc (6)`
  - `Round 2: inc in each st around (12)`
  - `Rounds 3-5: sc in each st around (12)`
  - Uses `*..., repeat from * N times` notation for stitch repeats
  - Includes stitch counts per group
- Live preview panel in the pattern editor (updates via Datastar as pattern changes)
- Read-only pattern detail view for reviewing a completed pattern
- Pattern duplication: copy an existing pattern as a starting point

**Tests**:
- Unit: Text renderer — simple group, group with repeats, stitch with repeats, mixed, edge cases (single stitch, empty group)
- Unit: Pattern duplication creates independent copy
- Integration: Edit pattern -> see live preview update -> duplicate -> verify independent copy

**Regression Gate**: All Phase 1-5 tests pass. CI green.

---

## Phase 6: Work Session (Live Pattern Tracker)

**Goal**: Users can start a work session on a pattern and track their progress stitch by stitch with keyboard navigation.

**Deliverables**:
- Migration 004 for work_sessions table
- `domain/session.go` — WorkSession entity and WorkSessionRepository interface
- `repository/sqlite/worksession.go` — SQLite WorkSessionRepository implementation
- `service/worksession.go` — Start session, advance/retreat navigation logic, pause/resume, complete, abandon
- Navigation logic:
  - **Forward**: Advance `current_stitch_count` within a stitch entry's count. When count exhausted, advance `current_stitch_repeat`. When repeats exhausted, advance `current_stitch_index`. When entries exhausted, advance `current_group_repeat`. When group repeats exhausted, advance `current_group_index`. When all groups exhausted, mark completed.
  - **Backward**: Reverse of forward logic.
- `handler/worksession.go` — Start, navigate (forward/backward), pause, resume, abandon handlers via SSE
- `view/worksession.templ` — Full-screen tracker:
  - Large display of current stitch abbreviation and name
  - Current group label and repeat progress (e.g., "Round 3 — Repeat 2 of 4")
  - Context: previous stitch (dimmed) and next stitch (dimmed)
  - Overall progress bar (percentage of total stitches completed)
  - Group progress bar
  - Keyboard shortcuts: Space/Right Arrow = forward, Backspace/Left Arrow = backward, P = pause, Esc = exit
  - Keyboard handling via Datastar `data-on-keydown` sending SSE requests
  - Active sessions list on dashboard

**Tests** (following Testing Strategy — only where they add value):
- Unit: Navigation logic — this is the core complexity of Phase 6 and warrants thorough testing: forward through simple pattern, forward through repeats, forward through group repeats, backward, boundary transitions, completion detection
- Integration: Start session -> navigate forward through entire small pattern -> verify completion
- Integration: Navigate backward from middle of pattern -> verify correct position
- Note: WorkSessionRepository CRUD and simple service wiring (start, pause/resume) are exercised by integration tests and do not need separate unit tests

**Regression Gate**: All Phase 1-6 tests pass. CI green.

---

## Phase 7: Polish, Accessibility & UX Refinements

**Goal**: Production-quality UX, mobile responsiveness, accessibility, and error handling.

**Deliverables**:
- Mobile-responsive layout (Bulma is mobile-first, but verify tracker view works on small screens)
- Touch-friendly tracker: swipe or tap zones for forward/backward on mobile
- Accessible markup: ARIA labels, keyboard focus management, screen reader support
- Flash messages for all user actions (success/error) via Datastar SSE
- Form validation feedback (inline errors on registration, login, pattern editor)
- Loading states during SSE requests (Bulma `is-loading` on buttons)
- Empty states (no patterns yet, no custom stitches, no active sessions)
- Confirm dialogs for destructive actions (delete pattern, abandon session)
- Pattern editor: undo last change (single-level, via server state)
- Rate limiting on auth endpoints (simple in-memory token bucket)
- Graceful error pages (404, 500) rendered via Templ

**Tests**:
- Unit: Rate limiter logic
- Integration: Full happy-path regression — register -> create custom stitch -> create pattern -> preview -> start session -> navigate to completion -> verify session marked complete
- Manual: Mobile responsiveness check, keyboard-only navigation, screen reader audit

**Regression Gate**: All Phase 1-7 tests pass. CI green. Full regression passes.
