## Planned Improvements

These are post-v1 improvements to be implemented incrementally. Each item should be fully implemented, tested, and regression-checked before moving to the next.

---

### IMP-12: Pattern Sharing (Read-Only)

**Problem**: Users have no way to share patterns with others. Sharing patterns is a core use case for crochet communities — designers want to distribute patterns, and crocheters want to share finds with friends. The sharing mechanism must be read-only (recipients cannot edit the original) and work without requiring the recipient to know the pattern owner.

**Goal**: Allow a pattern owner to generate a unique, unguessable share link for any pattern. Anyone with the link (authenticated or not) can view the pattern in read-only mode. Authenticated users can additionally "save" (duplicate) a shared pattern into their own collection. The share link can be revoked by the owner at any time.

---

#### Existing Foundation

The codebase already has several pieces that support sharing:

1. **`Pattern.Locked` field** (`internal/domain/pattern.go:24`) — A boolean on the `Pattern` struct. Currently used to prevent editing/deleting a pattern. This was added in migration 007. Locked patterns redirect from the edit page to the view page (`internal/handler/pattern.go:164`). The pattern list UI shows a lock icon for locked patterns.

2. **Self-contained `PatternStitch` model** (migration 007, `internal/domain/pattern.go:31-39`) — Patterns already snapshot their stitches into `pattern_stitches` rows, decoupled from the global stitch library. This means a shared pattern displays correctly even if the original owner's custom stitches are later modified or deleted. This is exactly the data model needed for sharing.

3. **`Duplicate` repository method** (`internal/repository/sqlite/pattern.go:205-229`) — Creates a full copy of a pattern (including all PatternStitches, InstructionGroups, and StitchEntries) for a different user. The copy is always unlocked. This is the mechanism for "save a shared pattern to my collection."

4. **Read-only pattern view** (`internal/handler/pattern.go:93-131`, `internal/view/pattern_view.templ`) — The `HandleView` handler and `PatternViewPage` templ already render a full read-only view of a pattern with all groups, stitch entries, pattern text preview, and images. Currently gated on `pattern.UserID == user.ID`, but the rendering logic itself is ownership-agnostic.

5. **Image serving** (`internal/handler/image.go`, `GET /images/{id}`) — Images are served by ID. Currently requires authentication but the actual serving logic doesn't depend on ownership (ownership is checked separately).

---

#### What's Missing

##### 1. Share Token — Domain & Storage

**New field on `Pattern` struct** (`internal/domain/pattern.go`):

```go
type Pattern struct {
    // ... existing fields ...
    ShareToken  string    // Unique unguessable token for sharing; empty = not shared
}
```

The `ShareToken` is a cryptographically random string (e.g., 32-byte hex = 64 characters, generated via `crypto/rand`). When empty, the pattern is not shared. When populated, anyone with the token can view the pattern.

**New migration** (`internal/repository/sqlite/migrations/008_add_share_token.sql`):

```sql
ALTER TABLE patterns ADD COLUMN share_token TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_patterns_share_token ON patterns(share_token) WHERE share_token != '';
```

The partial unique index ensures tokens are unique when present but allows multiple rows with empty strings.

**Repository changes** (`internal/repository/sqlite/pattern.go`):

- Add `share_token` to all `SELECT`, `INSERT`, and `UPDATE` queries for patterns.
- Add new method `GetByShareToken(ctx context.Context, token string) (*Pattern, error)` — looks up a pattern by its share token. Returns `ErrNotFound` if no match.
- Update `Duplicate` to clear the share token on copies (copies should not inherit the share link).

**Domain interface** (`internal/domain/pattern.go`):

```go
type PatternRepository interface {
    // ... existing methods ...
    GetByShareToken(ctx context.Context, token string) (*Pattern, error)
}
```

##### 2. Share/Unshare Service Methods

**Service layer** (`internal/service/pattern.go`):

- `GenerateShareLink(ctx context.Context, userID, patternID int64) (string, error)` — Verifies ownership, generates a random token via `crypto/rand`, sets `pattern.ShareToken`, updates the pattern, returns the token. If the pattern already has a share token, returns the existing one (idempotent).
- `RevokeShareLink(ctx context.Context, userID, patternID int64) error` — Verifies ownership, clears `pattern.ShareToken`, updates the pattern.
- `GetByShareToken(ctx context.Context, token string) (*Pattern, error)` — Passthrough to repository, no ownership check (anyone with the token can view).

Token generation:

```go
func generateShareToken() (string, error) {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return "", fmt.Errorf("generate share token: %w", err)
    }
    return hex.EncodeToString(b), nil
}
```

##### 3. Public Share Route & Handler

**Handler** (`internal/handler/pattern.go` or new `internal/handler/share.go`):

- `GET /s/{token}` — **Public route** (no auth required). Looks up the pattern by share token. Renders a read-only view using an adapted version of `PatternViewPage` (or a new `SharedPatternViewPage`). Shows the pattern content, pattern text preview, images. Does NOT show edit/delete buttons. Shows a "Save to My Patterns" button if the viewer is authenticated.
- `POST /s/{token}/save` — **Authenticated route**. Duplicates the shared pattern into the authenticated user's collection. Redirects to the user's pattern list.

**Route registration** (`internal/handler/routes.go`):

```go
// Public share routes.
mux.Handle("GET /s/{token}", OptionalAuth(auth, http.HandlerFunc(shareHandler.HandleViewShared)))

// Save shared pattern (authenticated).
mux.Handle("POST /s/{token}/save", RequireAuth(auth, http.HandlerFunc(shareHandler.HandleSaveShared)))
```

The `GET /s/{token}` route uses `OptionalAuth` so that:
- Unauthenticated users see the pattern + a prompt to register/login to save it
- Authenticated users see the pattern + a "Save to My Patterns" button

##### 4. Share Management UI (Owner)

**Pattern View page** (`internal/view/pattern_view.templ`):

Add a "Share" section below the action buttons:

- If `pattern.ShareToken == ""`: Show a "Share" button that POSTs to `/patterns/{id}/share` to generate a link.
- If `pattern.ShareToken != ""`: Show the share URL (e.g., `{baseURL}/s/{token}`), a "Copy Link" button (small JS or Datastar to copy to clipboard), and a "Revoke" button that POSTs to `/patterns/{id}/unshare`.

**Pattern List page** (`internal/view/pattern_list.templ`):

Add a small share indicator icon on pattern cards that have an active share token (e.g., a link icon next to the lock icon).

**Handler endpoints**:

- `POST /patterns/{id}/share` — Calls `GenerateShareLink`, redirects back to the pattern view page.
- `POST /patterns/{id}/unshare` — Calls `RevokeShareLink`, redirects back to the pattern view page.

##### 5. Shared Pattern View Page

**New templ** (`internal/view/shared_pattern.templ`) or adapt existing `PatternViewPage`:

A dedicated shared view is cleaner because it has different chrome:
- No edit/delete/duplicate buttons
- Shows the pattern owner's display name (e.g., "Shared by Alice")
- "Save to My Patterns" button (if authenticated) or "Login to save this pattern" prompt (if not)
- Full pattern content: metadata, pattern text preview, instruction groups with stitch entries, images
- No navbar items for patterns/stitches/sessions if the viewer is unauthenticated

This requires the handler to load the pattern owner's display name. Add to the handler:

```go
owner, err := h.auth.GetUserByID(ctx, pattern.UserID)
```

The view signature:

```go
templ SharedPatternPage(displayName string, pattern *domain.Pattern, ownerName string, groupImages map[int64][]domain.PatternImage, isAuthenticated bool)
```

##### 6. Image Access for Shared Patterns

Currently `GET /images/{id}` requires authentication. For shared patterns, images need to be viewable without auth.

Two options:
- **Option A**: Make `GET /images/{id}` public (simplest, but exposes all images by ID — low risk since IDs are auto-increment integers, but not ideal).
- **Option B (Recommended)**: Add a share-token-scoped image route: `GET /s/{token}/images/{id}` — verifies the image belongs to the shared pattern before serving. This keeps `GET /images/{id}` authenticated-only.

##### 7. Authorization Considerations

- Only the pattern owner can generate/revoke share links.
- Share tokens should be long enough to be unguessable (64 hex chars = 256 bits of entropy).
- Revoking a share link immediately prevents future access — there's no caching to worry about since all views are server-rendered.
- Duplicating a shared pattern does NOT copy the share token — the recipient's copy starts unshared.
- A shared pattern with active work sessions remains shareable — sessions belong to the owner, not the viewer.

---

#### Affected files

- **New**: `internal/view/shared_pattern.templ`, `internal/handler/share.go` (or extend `pattern.go`), `internal/repository/sqlite/migrations/008_add_share_token.sql`
- **Modified**: `internal/domain/pattern.go` (add `ShareToken` field, add `GetByShareToken` to interface), `internal/repository/sqlite/pattern.go` (add `share_token` to queries, implement `GetByShareToken`, clear token on duplicate), `internal/service/pattern.go` (add share/unshare methods), `internal/handler/routes.go` (register share routes), `internal/view/pattern_view.templ` (share management UI), `internal/view/pattern_list.templ` (share indicator), `internal/handler/image.go` (share-scoped image route), `main.go` (wire share handler if separate)

#### Regression gate

All existing tests pass. New tests cover: generate share token (success, non-owner → error), revoke share token, view shared pattern (valid token, invalid token → 404, revoked token → 404), save shared pattern (authenticated → duplicate created, unauthenticated → redirect to login), image access through share route. Pattern CRUD, work sessions, and stitch library unaffected.

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
