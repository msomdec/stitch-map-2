# StitchMap - Crochet Pattern Builder & Tracker

## Project Overview

StitchMap is a web application that allows registered users to create, manage, and actively work through crochet patterns. Users define patterns using standard crochet stitch abbreviations (with support for custom stitches), organize stitches into rounds/rows with repeat notation, and then track their real-time progress stitch-by-stitch as they crochet.

---

## Tech Stack

| Layer | Technology | Notes |
|-------|-----------|-------|
| **Language** | Go (latest stable) | Standard library preferred; external deps only when they reduce complexity significantly |
| **HTTP Router** | `net/http` (stdlib) | Use Go 1.22+ enhanced routing with method+pattern support |
| **HTML Rendering** | [Templ](https://templ.guide/) (`github.com/a-h/templ`) | Type-safe HTML components compiled to Go |
| **Reactivity** | [Datastar](https://data-star.dev/) (`github.com/starfederation/datastar-go`) | Hypermedia-driven SSE reactivity; no JS build step |
| **CSS Framework** | [Bulma CSS](https://bulma.io/) | Loaded via CDN; no build tooling needed |
| **Database** | SQLite via `modernc.org/sqlite` (pure Go) | CGo-free; all access behind repository interfaces |
| **Migrations** | Manual SQL files per DB implementation | SQL-based, embedded via `embed.FS`, owned by each `domain.Database` implementation |
| **Authentication** | JWT-based with bcrypt | `golang.org/x/crypto/bcrypt` for password hashing; `github.com/golang-jwt/jwt/v5` for JWTs |
| **Testing** | stdlib `testing` + `net/http/httptest` | No test framework dependencies |
| **CI/CD** | GitHub Actions | Build validation + unit tests on PRs and `main` |

### Dependency Philosophy

- **Prefer stdlib**: `net/http`, `database/sql`, `encoding/json`, `crypto/rand`, `log/slog`, `context`, `embed`
- **Use external deps when**: the stdlib alternative would require significant boilerplate or introduce maintenance burden (e.g., Templ for HTML, Datastar for SSE reactivity, bcrypt for password hashing, JWT for auth tokens)
- **Avoid**: ORMs, heavy middleware frameworks, JavaScript build tools, npm

---

## Architecture

### Domain-Driven Design (Go-Idiomatic)

The project follows Go-idiomatic DDD principles: domain types and repository interfaces live in the `domain` package, implementations live in infrastructure packages, and dependency injection wires everything together via constructors.

```
stitch-map-2/
├── CLAUDE.md
├── README.md
├── go.mod
├── go.sum
├── main.go                          # Entrypoint: wiring, server startup
├── .github/
│   └── workflows/
│       └── ci.yml                   # Build + test on PR and main
├── internal/
│   ├── domain/                      # Core business types & interfaces (no external deps)
│   │   ├── db.go                    # Database interface (Migrate, Close)
│   │   ├── user.go                  # User entity, UserRepository interface
│   │   ├── pattern.go               # Pattern, Round, StitchEntry entities
│   │   ├── stitch.go                # Stitch (predefined + custom), StitchRepository interface
│   │   ├── session.go               # WorkSession entity, progress tracking
│   │   └── errors.go                # Domain-specific error types
│   ├── service/                     # Application services (business logic orchestration)
│   │   ├── auth.go                  # Registration, login, session management
│   │   ├── pattern.go               # Pattern CRUD, validation, duplication
│   │   ├── stitch.go                # Stitch library management
│   │   └── worksession.go           # Active pattern tracking, navigation
│   ├── repository/                  # Repository implementations
│   │   └── sqlite/
│   │       ├── sqlite.go            # DB struct, connection, implements domain.Database
│   │       ├── migrations/          # SQLite-specific migration files & runner
│   │       │   ├── embed.go         # embed.FS for migration SQL files
│   │       │   ├── runner.go        # Migration runner (schema_migrations tracking)
│   │       │   ├── 001_create_users.sql
│   │       │   ├── 002_create_stitches.sql
│   │       │   ├── 003_create_patterns.sql
│   │       │   └── 004_create_work_sessions.sql
│   │       ├── user.go              # UserRepository implementation
│   │       ├── pattern.go           # PatternRepository implementation
│   │       ├── stitch.go            # StitchRepository implementation
│   │       └── worksession.go       # WorkSessionRepository implementation
│   ├── handler/                     # HTTP handlers (Datastar SSE + page renders)
│   │   ├── middleware.go            # Auth middleware, request logging
│   │   ├── auth.go                  # Login, register, logout handlers
│   │   ├── pattern.go               # Pattern CRUD handlers
│   │   ├── stitch.go                # Stitch library handlers
│   │   ├── worksession.go           # Live tracking SSE handlers
│   │   └── routes.go                # Route registration
│   └── view/                        # Templ components (.templ files)
│       ├── layout.templ             # Base HTML layout (Bulma + Datastar CDN)
│       ├── auth.templ               # Login/register forms
│       ├── pattern_list.templ       # Pattern listing page
│       ├── pattern_editor.templ     # Pattern builder UI
│       ├── stitch_library.templ     # Stitch abbreviation browser/editor
│       ├── worksession.templ        # Live pattern tracker UI
│       └── components.templ         # Shared reusable components (navbar, flash, etc.)
├── static/                          # Static assets (minimal; Bulma via CDN)
│   └── favicon.ico
└── test/                            # Integration tests
    └── integration_test.go
```

### Key Architectural Rules

1. **`domain/` has zero external imports** - only stdlib types. All interfaces are defined here.
2. **Repository interfaces in `domain/`** - implementations in `repository/sqlite/`. SQLite can be swapped for Postgres, etc., by implementing the same interfaces.
3. **`Database` interface in `domain/`** — defines lifecycle operations (`Migrate`, `Close`) so the entire database backend (including migrations) is swappable. Each database implementation owns its own migration files and runner. Migration SQL files live alongside the implementation (e.g., `repository/sqlite/migrations/`) since DDL is database-specific.
4. **Services depend on interfaces, not implementations** - all repository dependencies are injected via constructors.
5. **Handlers depend on services** - handlers never touch repositories directly.
6. **Templ components are pure rendering** - no business logic in `.templ` files.
7. **Datastar SSE pattern**: handlers create `datastar.NewSSE(w, r)`, read signals with `datastar.ReadSignals(r, &store)`, and respond with `sse.PatchElements(...)` or `sse.MarshalAndPatchSignals(...)`.

---

## Domain Model

### Database

The `Database` interface defines lifecycle operations for the underlying database. Each implementation (SQLite, Postgres, etc.) owns its migration files and strategy, ensuring the entire database backend is swappable.

```go
type Database interface {
    Migrate(ctx context.Context) error
    Close() error
}
```

### User

```go
type User struct {
    ID           int64
    Email        string
    DisplayName  string
    PasswordHash string
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

type UserRepository interface {
    Create(ctx context.Context, user *User) error
    GetByID(ctx context.Context, id int64) (*User, error)
    GetByEmail(ctx context.Context, email string) (*User, error)
}
```

### Stitch

Stitches represent crochet stitch types. The system seeds predefined standard stitches and allows users to create custom ones.

```go
type Stitch struct {
    ID           int64
    Abbreviation string    // e.g., "sc", "dc", "hdc"
    Name         string    // e.g., "Single Crochet", "Double Crochet"
    Description  string    // How to perform the stitch
    Category     string    // "basic", "advanced", "decrease", "increase", "post", "specialty"
    IsCustom     bool      // false = predefined, true = user-created
    UserID       *int64    // nil for predefined stitches
    CreatedAt    time.Time
}

type StitchRepository interface {
    ListPredefined(ctx context.Context) ([]Stitch, error)
    ListByUser(ctx context.Context, userID int64) ([]Stitch, error)
    GetByID(ctx context.Context, id int64) (*Stitch, error)
    GetByAbbreviation(ctx context.Context, abbreviation string, userID *int64) (*Stitch, error)
    Create(ctx context.Context, stitch *Stitch) error
    Update(ctx context.Context, stitch *Stitch) error
    Delete(ctx context.Context, id int64) error
}
```

### Predefined Stitch Library

The following standard US crochet abbreviations are seeded into the database on first run (sourced from the [Craft Yarn Council](https://www.craftyarncouncil.com/standards/crochet-abbreviations)):

#### Basic Stitches
| Abbreviation | Name | Category |
|---|---|---|
| `ch` | Chain | basic |
| `sl st` | Slip Stitch | basic |
| `sc` | Single Crochet | basic |
| `hdc` | Half Double Crochet | basic |
| `dc` | Double Crochet | basic |
| `tr` | Treble Crochet | basic |
| `dtr` | Double Treble Crochet | basic |

#### Increase / Decrease
| Abbreviation | Name | Category |
|---|---|---|
| `inc` | Increase (2 stitches in one) | increase |
| `dec` | Decrease (2 stitches together) | decrease |
| `sc2tog` | Single Crochet 2 Together | decrease |
| `hdc2tog` | Half Double Crochet 2 Together | decrease |
| `dc2tog` | Double Crochet 2 Together | decrease |
| `dc3tog` | Double Crochet 3 Together | decrease |
| `tr2tog` | Treble Crochet 2 Together | decrease |

#### Post Stitches
| Abbreviation | Name | Category |
|---|---|---|
| `FPsc` | Front Post Single Crochet | post |
| `BPsc` | Back Post Single Crochet | post |
| `FPdc` | Front Post Double Crochet | post |
| `BPdc` | Back Post Double Crochet | post |
| `FPtr` | Front Post Treble Crochet | post |
| `BPtr` | Back Post Treble Crochet | post |

#### Loop Variations
| Abbreviation | Name | Category |
|---|---|---|
| `BLO` | Back Loop Only | advanced |
| `FLO` | Front Loop Only | advanced |

#### Specialty Stitches
| Abbreviation | Name | Category |
|---|---|---|
| `pc` | Popcorn Stitch | specialty |
| `puff` | Puff Stitch | specialty |
| `cl` | Cluster | specialty |
| `sh` | Shell | specialty |
| `bob` | Bobble | specialty |
| `crab st` | Crab Stitch (Reverse SC) | specialty |
| `lp st` | Loop Stitch | specialty |
| `v-st` | V-Stitch | specialty |
| `sk` | Skip | action |
| `yo` | Yarn Over | action |
| `tch` | Turning Chain | action |
| `MR` | Magic Ring | action |

### Pattern

A pattern is a collection of ordered instruction groups (rounds or rows), each containing stitch entries with optional repeat counts.

```go
type PatternType string

const (
    PatternTypeRound PatternType = "round"   // Worked in continuous rounds (e.g., amigurumi)
    PatternTypeRow   PatternType = "row"     // Worked in flat rows (e.g., scarf)
)

type Pattern struct {
    ID            int64
    UserID        int64
    Name          string
    Description   string
    PatternType   PatternType
    HookSize      string       // e.g., "5.0mm", "H/8"
    YarnWeight    string       // e.g., "Worsted", "DK", "Bulky"
    Notes         string       // General pattern notes
    InstructionGroups []InstructionGroup
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

type InstructionGroup struct {
    ID            int64
    PatternID     int64
    SortOrder     int          // Position in the pattern
    Label         string       // e.g., "Round 1", "Row 3", "Brim", "Body"
    RepeatCount   int          // How many times to repeat this entire group (default 1)
    StitchEntries []StitchEntry
    ExpectedCount *int         // Expected stitch count at end of group (for verification)
}

type StitchEntry struct {
    ID                 int64
    InstructionGroupID int64
    SortOrder          int
    StitchID           int64    // References a Stitch
    Count              int      // How many of this stitch (e.g., "sc 5" = sc with count 5)
    IntoStitch         string   // Optional: "into ch-sp", "into next st", etc.
    RepeatCount        int      // How many times to repeat this entry (default 1)
    Notes              string   // e.g., "in BLO", "working into ring"
}

type PatternRepository interface {
    Create(ctx context.Context, pattern *Pattern) error
    GetByID(ctx context.Context, id int64) (*Pattern, error)
    ListByUser(ctx context.Context, userID int64) ([]Pattern, error)
    Update(ctx context.Context, pattern *Pattern) error
    Delete(ctx context.Context, id int64) error
    Duplicate(ctx context.Context, id int64, newUserID int64) (*Pattern, error)
}
```

### Work Session (Live Tracking)

A work session tracks a user's real-time progress through a specific pattern.

```go
type WorkSession struct {
    ID                   int64
    PatternID            int64
    UserID               int64
    CurrentGroupIndex    int     // Which instruction group the user is on (0-based)
    CurrentGroupRepeat   int     // Which repeat of the group they're on (0-based)
    CurrentStitchIndex   int     // Which stitch entry within the group (0-based)
    CurrentStitchRepeat  int     // Which repeat of the stitch entry (0-based)
    CurrentStitchCount   int     // Which individual stitch within the count (0-based)
    Status               string  // "active", "paused", "completed"
    StartedAt            time.Time
    LastActivityAt       time.Time
    CompletedAt          *time.Time
}

type WorkSessionRepository interface {
    Create(ctx context.Context, session *WorkSession) error
    GetByID(ctx context.Context, id int64) (*WorkSession, error)
    GetActiveByUser(ctx context.Context, userID int64) ([]WorkSession, error)
    Update(ctx context.Context, session *WorkSession) error
    Delete(ctx context.Context, id int64) error
}
```

---

## Database Schema

All tables use SQLite. Migrations are plain `.sql` files owned by the SQLite implementation (`repository/sqlite/migrations/`), embedded via `embed.FS`. A custom migration runner tracks applied migrations in a `schema_migrations` table and applies any unapplied migrations in filename order when `Database.Migrate()` is called. Because migration files live with the database implementation, a different backend (e.g., Postgres) can provide its own DDL and migration strategy while implementing the same `domain.Database` interface.

### Migration 001: Users

```sql
-- 001_create_users.sql
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email);
```

### Migration 002: Stitches

```sql
-- 002_create_stitches.sql
CREATE TABLE IF NOT EXISTS stitches (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    abbreviation TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT 'basic',
    is_custom BOOLEAN NOT NULL DEFAULT FALSE,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(abbreviation, user_id)
);

CREATE INDEX IF NOT EXISTS idx_stitches_user ON stitches(user_id);
CREATE INDEX IF NOT EXISTS idx_stitches_category ON stitches(category);
```

### Migration 003: Patterns

```sql
-- 003_create_patterns.sql
CREATE TABLE IF NOT EXISTS patterns (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    pattern_type TEXT NOT NULL DEFAULT 'round',
    hook_size TEXT NOT NULL DEFAULT '',
    yarn_weight TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_patterns_user ON patterns(user_id);

CREATE TABLE IF NOT EXISTS instruction_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pattern_id INTEGER NOT NULL REFERENCES patterns(id) ON DELETE CASCADE,
    sort_order INTEGER NOT NULL,
    label TEXT NOT NULL,
    repeat_count INTEGER NOT NULL DEFAULT 1,
    expected_count INTEGER,
    UNIQUE(pattern_id, sort_order)
);

CREATE INDEX IF NOT EXISTS idx_instruction_groups_pattern ON instruction_groups(pattern_id);

CREATE TABLE IF NOT EXISTS stitch_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instruction_group_id INTEGER NOT NULL REFERENCES instruction_groups(id) ON DELETE CASCADE,
    sort_order INTEGER NOT NULL,
    stitch_id INTEGER NOT NULL REFERENCES stitches(id),
    count INTEGER NOT NULL DEFAULT 1,
    into_stitch TEXT NOT NULL DEFAULT '',
    repeat_count INTEGER NOT NULL DEFAULT 1,
    notes TEXT NOT NULL DEFAULT '',
    UNIQUE(instruction_group_id, sort_order)
);

CREATE INDEX IF NOT EXISTS idx_stitch_entries_group ON stitch_entries(instruction_group_id);
```

### Migration 004: Work Sessions

```sql
-- 004_create_work_sessions.sql
CREATE TABLE IF NOT EXISTS work_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pattern_id INTEGER NOT NULL REFERENCES patterns(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    current_group_index INTEGER NOT NULL DEFAULT 0,
    current_group_repeat INTEGER NOT NULL DEFAULT 0,
    current_stitch_index INTEGER NOT NULL DEFAULT 0,
    current_stitch_repeat INTEGER NOT NULL DEFAULT 0,
    current_stitch_count INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'active',
    started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_activity_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_work_sessions_user ON work_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_work_sessions_pattern ON work_sessions(pattern_id);
CREATE INDEX IF NOT EXISTS idx_work_sessions_status ON work_sessions(status);
```

---

## UI / UX Design

### Layout

- **Bulma CSS** loaded via CDN for responsive, mobile-first design
- **Datastar** loaded via CDN `<script>` tag — no JS build step
- **Navbar**: App logo, navigation links (Patterns, Stitch Library, Active Sessions), user menu (profile, logout)
- **Flash messages**: Success/error notifications using Bulma notification components, delivered via Datastar SSE

### Key Pages

1. **Login / Register** — Simple forms with email, password, display name. Datastar handles form submission via SSE.
2. **Pattern List** — Card grid showing user's patterns with name, type (rounds/rows), stitch count, last modified. Actions: Edit, Duplicate, Delete, Start Working.
3. **Pattern Editor** — The core builder interface:
   - Header: pattern name, description, hook size, yarn weight, pattern type selector
   - Instruction group list: sortable groups with label, repeat count, expected stitch count
   - Within each group: sortable stitch entries with stitch picker (dropdown of available stitches), count, into-stitch, repeat, notes
   - Add/remove/reorder groups and entries via Datastar reactivity
   - Live pattern preview: rendered text view of the pattern using standard crochet notation
4. **Stitch Library** — Browseable/searchable table of all predefined stitches grouped by category. Section for user's custom stitches with add/edit/delete.
5. **Work Session (Tracker)** — Full-screen focused view:
   - Current instruction group label and repeat info
   - Current stitch highlighted with large text
   - Previous/next stitch context visible
   - Progress bar (overall pattern and current group)
   - Navigation: Forward (Space/Right Arrow), Backward (Backspace/Left Arrow)
   - Keyboard shortcuts handled via Datastar `data-on-keydown`
   - Pause/Resume, abandon session

### Datastar Interaction Pattern

All interactivity follows the Datastar hypermedia pattern:
- Page loads render full HTML via Templ
- User actions trigger SSE requests via `data-on-click="@post('/api/...')"` or `data-on-keydown`
- Server reads signals, processes logic, responds with `sse.PatchElements(...)` to update DOM fragments
- No client-side state management — server is the source of truth

---

## Authentication (JWT-Based)

- **Registration**: Email + password + display name. Passwords hashed with bcrypt (cost 12).
- **Login**: Email + password. On success, issue a signed JWT containing `sub` (user ID), `email`, `exp` (expiration). Set the JWT as an `HttpOnly`, `Secure`, `SameSite=Lax` cookie named `auth_token`.
- **JWT structure**: Standard claims (`sub`, `exp`, `iat`) plus custom claims (`email`, `display_name`). Signed with HMAC-SHA256 using `JWT_SECRET` environment variable. Default expiration: 24 hours.
- **Auth middleware**: Reads `auth_token` cookie, validates and parses JWT, extracts user ID from `sub` claim, loads user from DB, injects user into request context. Returns 401 for missing/invalid/expired tokens.
- **Logout**: Clears the `auth_token` cookie (sets `MaxAge=-1`). Since JWTs are stateless, no server-side session cleanup is needed.
- **No server-side session table**: Auth state is entirely in the JWT. This simplifies the database schema and eliminates session cleanup concerns.

---

## GitHub Actions CI

### `.github/workflows/ci.yml`

Triggers on:
- Pull requests targeting `main`
- Pushes to `main`

Jobs:
1. **build**: `go build ./...`
2. **test**: `go test ./... -race -count=1`
3. **templ-generate**: Install templ CLI, run `templ generate`, verify no diff (ensures generated files are committed)
4. **vet**: `go vet ./...`

Matrix: Latest stable Go version on `ubuntu-latest`.

---

## Implementation Phases

Each phase is independently implementable, testable, and deployable. Each phase must pass all existing tests before proceeding to the next.

---

### Phase 1: Project Scaffolding & Infrastructure

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

### Phase 2: User Authentication (JWT)

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

### Phase 3: Stitch Library

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

### Phase 4: Pattern Builder (Core)

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

### Phase 5: Pattern Preview & Text Rendering

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

### Phase 6: Work Session (Live Pattern Tracker)

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

**Tests**:
- Unit: Navigation logic — forward through simple pattern, forward through repeats, forward through group repeats, backward, boundary transitions, completion detection
- Unit: WorkSessionRepository CRUD
- Unit: Service — start session, advance to completion, pause/resume state
- Integration: Start session -> navigate forward through entire small pattern -> verify completion
- Integration: Navigate backward from middle of pattern -> verify correct position

**Regression Gate**: All Phase 1-6 tests pass. CI green.

---

### Phase 7: Polish, Accessibility & UX Refinements

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

---

## Development Commands

```bash
# Install templ CLI
go install github.com/a-h/templ/cmd/templ@latest

# Generate templ files (run after any .templ file changes)
templ generate

# Run the application
go run main.go

# Run all tests
go test ./... -race -count=1

# Run tests with verbose output
go test ./... -race -count=1 -v

# Run specific package tests
go test ./internal/service/... -race -count=1

# Vet
go vet ./...

# Build
go build -o stitch-map ./main.go
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `DATABASE_PATH` | `stitch-map.db` | Path to SQLite database file |
| `JWT_SECRET` | (required) | HMAC-SHA256 signing key for JWT tokens |
| `BCRYPT_COST` | `12` | bcrypt cost factor |

---

## Coding Conventions

- **Error handling**: Always return errors; use `fmt.Errorf("operation: %w", err)` for wrapping with context.
- **Context**: Pass `context.Context` as first parameter to all repository and service methods.
- **Naming**: Follow Go conventions — exported types are PascalCase, unexported are camelCase, acronyms are all-caps (ID, HTTP, SSE).
- **Logging**: Use `log/slog` with structured fields. Log at handler level, not in domain/repository.
- **Testing**: Table-driven tests. Use `t.Helper()` in test helpers. Use `testutil` sub-packages for shared test fixtures.
- **Database transactions**: Use a `WithTx` helper for operations that span multiple tables.
- **Templ files**: One `.templ` file per page/feature. Shared components in `components.templ`.
- **No global state**: All dependencies injected through constructors. Server struct holds all handler dependencies.
- **Routing**: Use Go 1.22+ enhanced `ServeMux` patterns. Method prefixes (`GET /path`) restrict by HTTP method. The `{$}` suffix matches a path exactly (e.g., `GET /{$}` matches only `/`, not `/foo`), eliminating manual path checks in handlers. Use `{name}` for path parameters (e.g., `GET /patterns/{id}`) — extract with `r.PathValue("id")`. Prefer these built-in features over custom routing logic.
