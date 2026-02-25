## Planned Improvements

These are post-v1 improvements to be implemented incrementally. Each item should be fully implemented, tested, and regression-checked before moving to the next.

---

### IMP-12: Pattern Sharing (Read-Only, Authenticated)

**Problem**: Users have no way to share patterns with others. Sharing patterns is a core use case for crochet communities — designers want to distribute patterns, and crocheters want to share finds with friends. The sharing mechanism must be read-only (recipients cannot edit the original) and work without requiring the recipient to know the pattern owner.

**Goal**: Allow a pattern owner to share any pattern via two modes, both requiring the viewer to be authenticated:

1. **Global link** — A shareable URL that any authenticated user can open. Good for posting in communities or sharing broadly.
2. **Email-bound link** — A unique share link tied to a specific user's email address. Only the authenticated user whose email matches can view the pattern. Good for sharing privately with a specific person.

The owner can revoke any share link at any time. Shared patterns are always read-only; authenticated viewers can "save" (duplicate) a shared pattern into their own collection.

---

#### Existing Foundation

The codebase already has several pieces that support sharing:

1. **`Pattern.Locked` field** (`internal/domain/pattern.go:24`) — A boolean on the `Pattern` struct. Currently used to prevent editing/deleting a pattern. This was added in migration 007. Locked patterns redirect from the edit page to the view page (`internal/handler/pattern.go:164`). The pattern list UI shows a lock icon for locked patterns.

2. **Self-contained `PatternStitch` model** (migration 007, `internal/domain/pattern.go:31-39`) — Patterns already snapshot their stitches into `pattern_stitches` rows, decoupled from the global stitch library. This means a shared pattern displays correctly even if the original owner's custom stitches are later modified or deleted. This is exactly the data model needed for sharing.

3. **`Duplicate` repository method** (`internal/repository/sqlite/pattern.go:205-229`) — Creates a full copy of a pattern (including all PatternStitches, InstructionGroups, and StitchEntries) for a different user. The copy is always unlocked. This is the mechanism for "save a shared pattern to my collection."

4. **Read-only pattern view** (`internal/handler/pattern.go:93-131`, `internal/view/pattern_view.templ`) — The `HandleView` handler and `PatternViewPage` templ already render a full read-only view of a pattern with all groups, stitch entries, pattern text preview, and images. Currently gated on `pattern.UserID == user.ID`, but the rendering logic itself is ownership-agnostic.

5. **Image serving** (`internal/handler/image.go`, `GET /images/{id}`) — Images are served by ID. Currently requires authentication and the actual serving logic doesn't depend on ownership (ownership is checked separately). Since all share routes now require auth, the existing authenticated image route works for shared patterns without modification.

---

#### What's Missing

##### 1. Share Domain Model & Storage

**New `PatternShare` entity** (`internal/domain/pattern.go`):

```go
type ShareType string

const (
    ShareTypeGlobal ShareType = "global" // Any authenticated user with the link
    ShareTypeEmail  ShareType = "email"  // Only the user matching the bound email
)

type PatternShare struct {
    ID            int64
    PatternID     int64
    Token         string    // Unique unguessable token (64 hex chars)
    ShareType     ShareType // "global" or "email"
    RecipientEmail string   // Non-empty only when ShareType == "email"
    CreatedAt     time.Time
}
```

A pattern can have **multiple shares** simultaneously — e.g., one global link and several email-bound links for different recipients. Each share has its own token and can be independently revoked.

**New `PatternShareRepository` interface** (`internal/domain/pattern.go`):

```go
type PatternShareRepository interface {
    Create(ctx context.Context, share *PatternShare) error
    GetByToken(ctx context.Context, token string) (*PatternShare, error)
    ListByPattern(ctx context.Context, patternID int64) ([]PatternShare, error)
    Delete(ctx context.Context, id int64) error
    DeleteAllByPattern(ctx context.Context, patternID int64) error
}
```

**New migration** (`internal/repository/sqlite/migrations/008_create_pattern_shares.sql`):

```sql
CREATE TABLE IF NOT EXISTS pattern_shares (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pattern_id INTEGER NOT NULL REFERENCES patterns(id) ON DELETE CASCADE,
    token TEXT NOT NULL UNIQUE,
    share_type TEXT NOT NULL DEFAULT 'global',
    recipient_email TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_pattern_shares_token ON pattern_shares(token);
CREATE INDEX IF NOT EXISTS idx_pattern_shares_pattern ON pattern_shares(pattern_id);
CREATE INDEX IF NOT EXISTS idx_pattern_shares_email ON pattern_shares(recipient_email) WHERE recipient_email != '';
```

Using a separate `pattern_shares` table (instead of a column on `patterns`) because a pattern can have multiple share links. `ON DELETE CASCADE` ensures shares are cleaned up when a pattern is deleted.

**Repository implementation** (`internal/repository/sqlite/share.go`):

Implements `PatternShareRepository` with standard CRUD. `GetByToken` is the hot path for viewing shared patterns. `ListByPattern` supports the management UI. `Delete` revokes a single share. `DeleteAllByPattern` revokes all shares for a pattern at once.

##### 2. Share Service Methods

**New `ShareService`** (`internal/service/share.go`):

```go
type ShareService struct {
    shares   domain.PatternShareRepository
    patterns domain.PatternRepository
    users    domain.UserRepository
}
```

Methods:

- `CreateGlobalShare(ctx context.Context, userID, patternID int64) (*domain.PatternShare, error)` — Verifies ownership. If a global share already exists for this pattern, returns it (idempotent). Otherwise generates a token, creates the share, returns it.
- `CreateEmailShare(ctx context.Context, userID, patternID int64, recipientEmail string) (*domain.PatternShare, error)` — Verifies ownership. Validates the email is non-empty and well-formed. If an email share already exists for this pattern+email pair, returns it (idempotent). Otherwise generates a token, creates the share, returns it. Does NOT require the recipient to already have an account — the share is bound to the email, so it works once they register.
- `RevokeShare(ctx context.Context, userID, shareID int64) error` — Loads the share, verifies the pattern is owned by the user, deletes the share.
- `RevokeAllShares(ctx context.Context, userID, patternID int64) error` — Verifies ownership, deletes all shares for the pattern.
- `GetPatternByShareToken(ctx context.Context, viewerUserID int64, token string) (*domain.Pattern, error)` — Looks up the share by token. If `ShareType == "global"`, returns the pattern. If `ShareType == "email"`, loads the viewer user by ID and checks that their email matches `RecipientEmail` — returns `ErrUnauthorized` if it doesn't match. Returns the full pattern with groups and entries.
- `ListSharesForPattern(ctx context.Context, userID, patternID int64) ([]domain.PatternShare, error)` — Verifies ownership, returns all active shares for the pattern.

Token generation (same helper, reused):

```go
func generateShareToken() (string, error) {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return "", fmt.Errorf("generate share token: %w", err)
    }
    return hex.EncodeToString(b), nil
}
```

##### 3. Share Routes & Handler

**All share routes require authentication.** There are no public/unauthenticated share routes.

**Handler** (`internal/handler/share.go`):

- `GET /s/{token}` — **Authenticated**. Calls `GetPatternByShareToken(viewerUserID, token)`. On success, renders the shared pattern view. On `ErrNotFound` → 404. On `ErrUnauthorized` (email mismatch) → 403 with "This pattern was shared with a different account" message.
- `POST /s/{token}/save` — **Authenticated**. Duplicates the shared pattern into the viewer's collection. Calls `GetPatternByShareToken` first to verify access, then calls `PatternRepository.Duplicate`. Redirects to the viewer's pattern list.

**Owner management endpoints** (on the pattern resource):

- `POST /patterns/{id}/share` — Creates a global share. Redirects back to pattern view.
- `POST /patterns/{id}/share/email` — Creates an email-bound share. Reads `recipientEmail` from form body. Redirects back to pattern view.
- `POST /patterns/{id}/share/{shareID}/revoke` — Revokes a single share. Redirects back to pattern view.
- `POST /patterns/{id}/share/revoke-all` — Revokes all shares. Redirects back to pattern view.

**Route registration** (`internal/handler/routes.go`):

```go
// Shared pattern viewing (authenticated).
mux.Handle("GET /s/{token}", RequireAuth(auth, http.HandlerFunc(shareHandler.HandleViewShared)))
mux.Handle("POST /s/{token}/save", RequireAuth(auth, http.HandlerFunc(shareHandler.HandleSaveShared)))

// Share management (owner, authenticated).
mux.Handle("POST /patterns/{id}/share", RequireAuth(auth, http.HandlerFunc(shareHandler.HandleCreateGlobalShare)))
mux.Handle("POST /patterns/{id}/share/email", RequireAuth(auth, http.HandlerFunc(shareHandler.HandleCreateEmailShare)))
mux.Handle("POST /patterns/{id}/share/{shareID}/revoke", RequireAuth(auth, http.HandlerFunc(shareHandler.HandleRevokeShare)))
mux.Handle("POST /patterns/{id}/share/revoke-all", RequireAuth(auth, http.HandlerFunc(shareHandler.HandleRevokeAllShares)))
```

##### 4. Share Management UI (Owner)

**Pattern View page** (`internal/view/pattern_view.templ`):

Add a "Sharing" section below the action buttons:

- **"Share via Link" button** — POSTs to `/patterns/{id}/share` to generate a global link. If one already exists, shows the existing URL.
- **"Share with User" form** — An email input field + submit button that POSTs to `/patterns/{id}/share/email`. Creates an email-bound share link.
- **Active shares list** — Shows all active shares for the pattern:
  - Global shares: show the share URL, a "Copy Link" button, and a "Revoke" button.
  - Email shares: show the recipient email, the share URL, a "Copy Link" button, and a "Revoke" button.
- **"Revoke All" button** — Visible when there are multiple active shares. POSTs to `/patterns/{id}/share/revoke-all`.

**Pattern List page** (`internal/view/pattern_list.templ`):

Add a small share indicator icon on pattern cards that have any active shares (e.g., a link icon next to the lock icon). The handler should pass a set of pattern IDs that have shares, determined via a count query or by loading share data alongside patterns.

##### 5. Shared Pattern View Page

**New templ** (`internal/view/shared_pattern.templ`):

A dedicated shared view with different chrome from the owner view:
- No edit/delete buttons
- Shows the pattern owner's display name (e.g., "Shared by Alice")
- "Save to My Patterns" button that POSTs to `/s/{token}/save`
- Full pattern content: metadata, pattern text preview, instruction groups with stitch entries, images
- Standard authenticated navbar (Dashboard, Patterns, Stitch Library) since the viewer is always logged in

This requires the handler to load the pattern owner's display name:

```go
owner, err := h.users.GetByID(ctx, pattern.UserID)
```

The view signature:

```go
templ SharedPatternPage(displayName string, pattern *domain.Pattern, ownerName string, groupImages map[int64][]domain.PatternImage)
```

No `isAuthenticated` parameter needed — the viewer is always authenticated.

##### 6. Image Access for Shared Patterns

Since all share routes require authentication, and the existing `GET /images/{id}` route already requires authentication, **no changes are needed** for image serving. Authenticated users can already access images via the existing route. The image ownership check in the existing handler may need to be relaxed to allow viewing images for patterns the user has share access to — or images can be served without ownership checks since the route is already behind auth and image IDs are not sensitive.

##### 7. Authorization Considerations

- Only the pattern owner can create/revoke share links.
- All share viewing requires authentication — unauthenticated users hitting `/s/{token}` are redirected to the login page (standard `RequireAuth` behavior).
- Share tokens are 64 hex chars (256 bits of entropy) — unguessable.
- Email-bound shares enforce that the authenticated viewer's email matches the `RecipientEmail`. This prevents link forwarding — if Alice shares with bob@example.com, only the account registered with bob@example.com can view it.
- Revoking a share immediately prevents future access — no caching concerns since all views are server-rendered.
- Duplicating a shared pattern does NOT copy any shares — the recipient's copy starts with zero shares.
- A shared pattern with active work sessions remains shareable — sessions belong to the owner, not the viewer.
- Deleting a pattern cascades to delete all its shares (via `ON DELETE CASCADE`).

---

#### Affected files

- **New**: `internal/domain/share.go` (or extend `pattern.go` — `PatternShare` entity, `PatternShareRepository` interface), `internal/repository/sqlite/share.go` (repository implementation), `internal/repository/sqlite/migrations/008_create_pattern_shares.sql`, `internal/service/share.go`, `internal/handler/share.go`, `internal/view/shared_pattern.templ`
- **Modified**: `internal/handler/routes.go` (register share routes), `internal/view/pattern_view.templ` (share management UI), `internal/view/pattern_list.templ` (share indicator), `main.go` (wire share service and handler)

#### Regression gate

All existing tests pass. New tests cover: create global share (success, non-owner → error, idempotent), create email share (success, non-owner → error, idempotent, invalid email → error), view global shared pattern (valid token, invalid token → 404, revoked token → 404), view email shared pattern (matching email → success, non-matching email → 403), save shared pattern (duplicate created, redirects to pattern list), revoke single share, revoke all shares, cascade delete on pattern delete. Pattern CRUD, work sessions, and stitch library unaffected.

---

### IMP-13: Existing Feature Improvements

These are smaller improvements to existing features identified during a codebase review. Each can be implemented independently.

---

#### 13a. Duplicate Pattern — Ownership Check

**File**: `internal/service/pattern.go:91-93`, `internal/handler/pattern.go:263-288`

**Problem**: The `Duplicate` method has no ownership check — any authenticated user who knows a pattern ID could duplicate it. While pattern IDs are auto-increment and not easily guessable, this is an authorization gap. Currently the `HandleDuplicate` handler doesn't verify `pattern.UserID == user.ID` before calling `Duplicate`.

**Fix**: Add an ownership check in `PatternService.Duplicate`:

```go
func (s *PatternService) Duplicate(ctx context.Context, userID int64, id int64, newUserID int64) (*Pattern, error) {
    existing, err := s.patterns.GetByID(ctx, id)
    if err != nil {
        return nil, err
    }
    if existing.UserID != userID {
        return nil, domain.ErrUnauthorized
    }
    return s.patterns.Duplicate(ctx, id, newUserID)
}
```

**Note**: Once IMP-12 (sharing) is implemented, the ownership check for duplicate should be relaxed to allow duplicating shared patterns (where the viewer has the share token). This should be handled by a separate `DuplicateShared` method or a flag.

---

#### 13b. Auth Cookie — Missing `Secure` Flag

**File**: `internal/handler/auth.go:90-97`

**Problem**: The `auth_token` cookie is set with `HttpOnly` and `SameSite=Lax` but is missing the `Secure` flag. In production HTTPS environments, this means the cookie could be transmitted over plain HTTP connections, exposing the JWT token.

**Fix**: Add `Secure: true` to the cookie in both `HandleLogin` and `HandleLogout`. To maintain local development usability, make the secure flag configurable via an environment variable (e.g., `COOKIE_SECURE=true` defaulting to `false` for development):

```go
http.SetCookie(w, &http.Cookie{
    Name:     "auth_token",
    Value:    token,
    Path:     "/",
    HttpOnly: true,
    Secure:   cfg.CookieSecure,
    SameSite: http.SameSiteLaxMode,
    MaxAge:   86400,
})
```

---

#### 13c. Image Service — Ownership Verification Relies on Pattern Load

**File**: `internal/service/image.go`

**Problem**: Image upload ownership is verified by loading the pattern, but `GetFile` and `Delete` also need ownership checks. Verify that the current implementation checks that the image's instruction group belongs to a pattern owned by the requesting user. If it doesn't, add the check.

---

#### 13d. Dashboard — N+1 Query for Pattern Names

**File**: `internal/handler/dashboard.go:44-58`

**Problem**: `buildPatternNames` calls `PatternService.GetByID` once per unique pattern ID. With many sessions this becomes N+1 queries. While acceptable for small datasets, this could be optimized.

**Fix**: Add a `GetNamesByIDs(ctx, ids []int64) (map[int64]string, error)` method to the pattern repository that fetches names in a single query using `WHERE id IN (...)`.

---

#### 13e. ListByUser — Eager Loading Performance

**File**: `internal/repository/sqlite/pattern.go:87-124`

**Problem**: `ListByUser` loads full pattern data (including all groups, entries, and pattern stitches) for every pattern. The list page only needs metadata, group count, and stitch count. For users with many complex patterns, this could be slow.

**Fix**: Add a `ListSummaryByUser(ctx, userID) ([]PatternSummary, error)` method that returns only the fields needed for the list page, computing group count and stitch count in SQL:

```sql
SELECT p.id, p.name, p.description, p.pattern_type, p.hook_size, p.yarn_weight, p.difficulty, p.locked, p.share_token,
       p.created_at, p.updated_at,
       COUNT(DISTINCT ig.id) as group_count,
       COALESCE(SUM(se.count * se.repeat_count * ig.repeat_count), 0) as stitch_count
FROM patterns p
LEFT JOIN instruction_groups ig ON ig.pattern_id = p.id
LEFT JOIN stitch_entries se ON se.instruction_group_id = ig.id
WHERE p.user_id = ?
GROUP BY p.id
ORDER BY p.updated_at DESC
```

---

### IMP-14: Pattern Import/Export (Text Format)

**Problem**: Users can't export patterns for backup or import patterns from text. Many crocheters share patterns in plain text format on forums, social media, and Ravelry.

**Goal**: Allow users to export a pattern as formatted text (matching the existing `RenderPatternText` output) and import a pattern from a structured text format.

---

#### Export

Already partially implemented — `service.RenderPatternText(pattern)` produces a readable text output. Add:

- A "Download as Text" button on the pattern view page
- `GET /patterns/{id}/export` handler that returns `Content-Type: text/plain` with `Content-Disposition: attachment; filename="{name}.txt"`
- Include pattern metadata (name, type, hook size, yarn weight, difficulty) as a header in the export

#### Import

More complex — requires a text parser. Start with a structured format:

```
Name: My Pattern
Type: round
Hook: 5.0mm
Yarn: Worsted
Difficulty: Beginner

Round 1: MR, sc 6 (6)
Round 2: inc 6 (12)
Round 3: [sc, inc] x6 (18)
```

The parser would:
1. Extract metadata from `Key: Value` headers
2. Parse each line as an instruction group with label, stitch entries, and optional expected count
3. Resolve stitch abbreviations against the user's available stitches (predefined + custom)
4. Create the pattern via the existing `PatternService.Create`

**Handler**: `POST /patterns/import` with a textarea form or file upload.

---

### IMP-15: Pattern Search & Filtering

**Problem**: The pattern list page shows all patterns in a single unfiltered list. As users accumulate patterns, finding a specific one becomes difficult.

**Goal**: Add search and filter capabilities to the pattern list page.

**Features**:
- **Text search**: Filter by pattern name and description (SQL `LIKE` or `INSTR`)
- **Filter by type**: Round / Row / All
- **Filter by difficulty**: Beginner / Intermediate / Advanced / Expert / All
- **Sort options**: Last modified (default), name, created date, stitch count

**Implementation**:
- Use query parameters (`?q=hat&type=round&difficulty=Beginner&sort=name`)
- Add filter controls above the pattern grid (using standard form submission, not Datastar — consistent with the stitch library filter approach)
- Update `PatternRepository.ListByUser` to accept filter/sort options, or add a new `SearchByUser` method

---

### IMP-16: Multi-Session per Pattern Prevention

**Problem**: A user can start multiple work sessions for the same pattern. This leads to confusion — which session represents their current progress? There's no warning or prevention mechanism.

**Goal**: When a user starts a session for a pattern that already has an active/paused session, either:
- **Option A**: Warn and redirect to the existing session (don't allow a second session)
- **Option B**: Warn but allow creating a new one ("You already have an active session for this pattern. Resume it or start fresh?")

**Implementation**: In `WorkSessionService.Start`, check `GetActiveByUser` for an existing session with the same `PatternID`. Show a confirmation prompt if one exists.

---

### IMP-17: Pattern Versioning / History

**Problem**: Editing a pattern is destructive — the previous version is lost. If a user accidentally changes or removes instruction groups, there's no way to recover.

**Goal**: Keep a version history of patterns. Each save creates a new version. Users can view previous versions and optionally restore them.

**Approach**: This is a significant feature. A lightweight approach:
- Add a `pattern_versions` table that stores a JSON snapshot of the pattern at each save
- Each `Update` call creates a version record before applying changes
- A "Version History" page shows timestamps and allows viewing past versions
- "Restore" duplicates a past version as the current state

This is an idea-level item — needs further design before implementation.
