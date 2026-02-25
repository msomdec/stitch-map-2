## Planned Improvements

These are post-v1 improvements to be implemented incrementally. Each item should be fully implemented, tested, and regression-checked before moving to the next.

---

### IMP-12: Pattern Sharing (Snapshot-Based, Authenticated)

**Problem**: Users have no way to share patterns with others. Sharing patterns is a core use case for crochet communities — designers want to distribute patterns, and crocheters want to share finds with friends. The sharing mechanism must produce an independent snapshot for the recipient — if the original is later modified or deleted, the recipient's copy is unaffected.

**Goal**: Allow a pattern owner to share any pattern via two modes, both requiring the viewer to be authenticated:

1. **Global link** — A shareable URL that any authenticated user can open. Good for posting in communities or sharing broadly.
2. **Email-bound link** — A unique share link tied to a specific user's email address. Only the authenticated user whose email matches can view the pattern. Good for sharing privately with a specific person.

When a recipient opens a share link, they see a preview of the shared pattern and can **save it to their library**. Saving duplicates the entire pattern (including all instruction groups, stitch entries, pattern stitches, and images) as a snapshot into the recipient's collection. The snapshot is fully independent — edits or deletion of the original have no effect on saved copies. Saved copies are marked with their origin (who shared it) and appear in a dedicated "Shared with Me" section on the pattern list page.

The owner can revoke any share link at any time. Revoking a link prevents future saves but does NOT affect copies already saved by recipients.

---

#### Existing Foundation

The codebase already has several pieces that support sharing:

1. **`Pattern.Locked` field** (`internal/domain/pattern.go:24`) — A boolean on the `Pattern` struct. Currently used to prevent editing/deleting a pattern. This was added in migration 007. Locked patterns redirect from the edit page to the view page (`internal/handler/pattern.go:164`). The pattern list UI shows a lock icon for locked patterns.

2. **Self-contained `PatternStitch` model** (migration 007, `internal/domain/pattern.go:31-39`) — Patterns already snapshot their stitches into `pattern_stitches` rows, decoupled from the global stitch library. This means a shared pattern displays correctly even if the original owner's custom stitches are later modified or deleted. This is exactly the data model needed for sharing.

3. **`Duplicate` repository method** (`internal/repository/sqlite/pattern.go:205-229`) — Creates a full copy of a pattern (including all PatternStitches, InstructionGroups, and StitchEntries) for a different user. The copy is always unlocked. This is the core mechanism for snapshotting a shared pattern — it needs to be extended to also set the `shared_from_user_id` origin metadata on the copy.

4. **Read-only pattern view** (`internal/handler/pattern.go:93-131`, `internal/view/pattern_view.templ`) — The `HandleView` handler and `PatternViewPage` templ already render a full read-only view of a pattern with all groups, stitch entries, pattern text preview, and images. Currently gated on `pattern.UserID == user.ID`, but the rendering logic itself is ownership-agnostic.

5. **Image serving** (`internal/handler/image.go`, `GET /images/{id}`) — Images are served by ID. Currently requires authentication and the actual serving logic doesn't depend on ownership (ownership is checked separately). Since all share routes now require auth, the existing authenticated image route works for shared patterns without modification.

6. **Pattern list page** (`internal/view/pattern_list.templ`) — Currently renders a single flat grid under "My Patterns". Has no concept of sections or grouping. This needs to be split into two sections.

---

#### What's Missing

##### 1. Share Domain Model & Storage

**New `PatternShare` entity** (`internal/domain/share.go`):

```go
type ShareType string

const (
    ShareTypeGlobal ShareType = "global" // Any authenticated user with the link
    ShareTypeEmail  ShareType = "email"  // Only the user matching the bound email
)

type PatternShare struct {
    ID             int64
    PatternID      int64
    Token          string    // Unique unguessable token (64 hex chars)
    ShareType      ShareType // "global" or "email"
    RecipientEmail string    // Non-empty only when ShareType == "email"
    CreatedAt      time.Time
}
```

A pattern can have **multiple shares** simultaneously — e.g., one global link and several email-bound links for different recipients. Each share has its own token and can be independently revoked.

**New `PatternShareRepository` interface** (`internal/domain/share.go`):

```go
type PatternShareRepository interface {
    Create(ctx context.Context, share *PatternShare) error
    GetByToken(ctx context.Context, token string) (*PatternShare, error)
    ListByPattern(ctx context.Context, patternID int64) ([]PatternShare, error)
    Delete(ctx context.Context, id int64) error
    DeleteAllByPattern(ctx context.Context, patternID int64) error
    HasSharesByPatternIDs(ctx context.Context, patternIDs []int64) (map[int64]bool, error)
}
```

The `HasSharesByPatternIDs` method supports the share indicator on the pattern list — given a list of pattern IDs, returns a map of which ones have at least one active share. This avoids N+1 queries when rendering the pattern list.

**Origin tracking on `Pattern`** (`internal/domain/pattern.go`):

```go
type Pattern struct {
    // ... existing fields ...
    SharedFromUserID *int64  // Non-nil = this pattern was saved from a share; points to the original sharer's user ID
    SharedFromName   string  // Denormalized display name of the sharer at time of save (so it survives account deletion)
}
```

`SharedFromUserID` distinguishes user-authored patterns (`nil`) from patterns received via sharing (non-nil). `SharedFromName` is denormalized so the "Shared by Alice" label works even if Alice's account is later deleted or her display name changes. Both fields are set during the duplicate-from-share operation and are immutable after creation.

**New migration** (`internal/repository/sqlite/migrations/008_pattern_sharing.sql`):

```sql
-- Share links table
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

-- Origin tracking on patterns (for "Shared with Me" section)
ALTER TABLE patterns ADD COLUMN shared_from_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE patterns ADD COLUMN shared_from_name TEXT NOT NULL DEFAULT '';
```

`ON DELETE SET NULL` for `shared_from_user_id` means if the sharer deletes their account, the recipient's copy retains `shared_from_name` for display but the FK becomes NULL. `ON DELETE CASCADE` on `pattern_shares` means deleting a pattern removes all its share links.

**Repository implementation** (`internal/repository/sqlite/share.go`):

Implements `PatternShareRepository` with standard CRUD. `GetByToken` is the hot path for viewing shared patterns. `ListByPattern` supports the management UI. `Delete` revokes a single share. `DeleteAllByPattern` revokes all shares for a pattern at once. `HasSharesByPatternIDs` runs a single query with `IN (...)` clause + `GROUP BY` to return the set of pattern IDs that have shares.

**Pattern repository changes** (`internal/repository/sqlite/pattern.go`):

- Add `shared_from_user_id` and `shared_from_name` to all `SELECT`, `INSERT`, and `UPDATE` queries.
- Extend `Duplicate` to accept an optional `SharedFrom` parameter (user ID + display name) that gets set on the copy. When duplicating from a share, these are populated. When duplicating your own pattern (existing feature), they remain nil/empty.
- Add `ListSharedWithUser(ctx context.Context, userID int64) ([]Pattern, error)` — returns all patterns where `user_id = ? AND shared_from_user_id IS NOT NULL`, sorted by `created_at DESC`. This powers the "Shared with Me" section.
- Update `ListByUser` to only return user-authored patterns: `user_id = ? AND shared_from_user_id IS NULL`. This keeps the "My Patterns" section clean.

**Domain interface update** (`internal/domain/pattern.go`):

```go
type PatternRepository interface {
    // ... existing methods ...
    ListSharedWithUser(ctx context.Context, userID int64) ([]Pattern, error)
    GetByShareToken(ctx context.Context, token string) (*Pattern, error)
}
```

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

- `CreateGlobalShare(ctx context.Context, userID, patternID int64) (*domain.PatternShare, error)` — Verifies ownership (`pattern.SharedFromUserID` must be nil — cannot reshare a received pattern). If a global share already exists for this pattern, returns it (idempotent). Otherwise generates a token, creates the share, returns it.
- `CreateEmailShare(ctx context.Context, userID, patternID int64, recipientEmail string) (*domain.PatternShare, error)` — Verifies ownership. Validates the email is non-empty and well-formed. Rejects if `recipientEmail` matches the owner's own email. If an email share already exists for this pattern+email pair, returns it (idempotent). Otherwise generates a token, creates the share, returns it. Does NOT require the recipient to already have an account — the share is bound to the email, so it works once they register.
- `RevokeShare(ctx context.Context, userID, shareID int64) error` — Loads the share, verifies the pattern is owned by the user, deletes the share. Does NOT affect copies already saved by recipients.
- `RevokeAllShares(ctx context.Context, userID, patternID int64) error` — Verifies ownership, deletes all shares for the pattern.
- `GetPatternByShareToken(ctx context.Context, viewerUserID int64, token string) (*domain.Pattern, error)` — Looks up the share by token. If `ShareType == "global"`, returns the pattern. If `ShareType == "email"`, loads the viewer user by ID and checks that their email matches `RecipientEmail` — returns `ErrUnauthorized` if it doesn't match. Returns the full pattern with groups and entries.
- `SaveSharedPattern(ctx context.Context, viewerUserID int64, token string) (*domain.Pattern, error)` — The core "accept share" operation. Verifies access (same logic as `GetPatternByShareToken`). Loads the original pattern owner to get their display name. Calls `Duplicate` with `SharedFrom{UserID, Name}` set. Returns the new pattern. If the viewer has already saved this pattern (checked via a query for existing patterns with matching `shared_from_user_id` + source pattern metadata), returns an error to prevent duplicate saves.
- `ListSharesForPattern(ctx context.Context, userID, patternID int64) ([]domain.PatternShare, error)` — Verifies ownership, returns all active shares for the pattern.
- `HasSharesByPatternIDs(ctx context.Context, patternIDs []int64) (map[int64]bool, error)` — Passthrough to repository. Used by the pattern list handler to determine which patterns to show the share indicator on.

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

- `GET /s/{token}` — **Authenticated**. Calls `GetPatternByShareToken(viewerUserID, token)`. On success, renders the shared pattern preview page. On `ErrNotFound` → 404. On `ErrUnauthorized` (email mismatch) → 403 with "This pattern was shared with a different account" message. If the viewer is the pattern owner, redirects to the normal pattern view page (owners don't need the share preview).
- `POST /s/{token}/save` — **Authenticated**. Calls `SaveSharedPattern(viewerUserID, token)`. On success, redirects to the pattern list with a flash message ("Pattern saved to your library!"). On duplicate save attempt, shows a flash message ("You've already saved this pattern.") and redirects.

**Owner management endpoints** (on the pattern resource):

- `POST /patterns/{id}/share` — Creates a global share. Redirects back to pattern view.
- `POST /patterns/{id}/share/email` — Creates an email-bound share. Reads `recipientEmail` from form body. Redirects back to pattern view.
- `POST /patterns/{id}/share/{shareID}/revoke` — Revokes a single share. Redirects back to pattern view.
- `POST /patterns/{id}/share/revoke-all` — Revokes all shares. Redirects back to pattern view.

**Route registration** (`internal/handler/routes.go`):

```go
// Shared pattern viewing and saving (authenticated).
mux.Handle("GET /s/{token}", RequireAuth(auth, http.HandlerFunc(shareHandler.HandleViewShared)))
mux.Handle("POST /s/{token}/save", RequireAuth(auth, http.HandlerFunc(shareHandler.HandleSaveShared)))

// Share management (owner, authenticated).
mux.Handle("POST /patterns/{id}/share", RequireAuth(auth, http.HandlerFunc(shareHandler.HandleCreateGlobalShare)))
mux.Handle("POST /patterns/{id}/share/email", RequireAuth(auth, http.HandlerFunc(shareHandler.HandleCreateEmailShare)))
mux.Handle("POST /patterns/{id}/share/{shareID}/revoke", RequireAuth(auth, http.HandlerFunc(shareHandler.HandleRevokeShare)))
mux.Handle("POST /patterns/{id}/share/revoke-all", RequireAuth(auth, http.HandlerFunc(shareHandler.HandleRevokeAllShares)))
```

##### 4. Pattern List Page — Two Sections

**Handler changes** (`internal/handler/pattern.go` — `HandleList`):

The existing `HandleList` handler currently calls `ListByUser` and renders a single list. It now needs to:

1. Call `ListByUser(userID)` — returns only user-authored patterns (`shared_from_user_id IS NULL`).
2. Call `ListSharedWithUser(userID)` — returns only patterns received via sharing (`shared_from_user_id IS NOT NULL`).
3. Call `HasSharesByPatternIDs(patternIDs)` with the IDs from step 1 — returns a `map[int64]bool` indicating which user-authored patterns have active share links.
4. Pass all three datasets to the templ.

**Template changes** (`internal/view/pattern_list.templ`):

The page renders two distinct sections:

**Section 1: "My Patterns"** — User-authored patterns (same cards as today). Each card additionally shows a **share indicator** (e.g., a small link/share icon next to the lock icon) if `sharedPatternIDs[pattern.ID]` is true. The indicator is purely informational — clicking it doesn't navigate anywhere; the share management UI lives on the pattern view page. Clicking the card goes to the normal pattern view/edit.

**Section 2: "Shared with Me"** — Patterns received via sharing. Each card shows:
- Pattern name
- "Shared by {SharedFromName}" subtitle
- Pattern metadata (type, hook size, yarn weight) — same as "My Patterns" cards
- Footer actions: **View** only (no Edit, no Delete, no Duplicate, no Start). These are read-only snapshots.
  - **Exception**: The owner may choose to unlock a received pattern for editing. If unlocked, Edit and Start become available. If locked (default for received patterns — see below), only View is available.

Received patterns are **locked by default** when duplicated via sharing. This preserves the snapshot semantics — the recipient sees exactly what was shared. If they want to modify it, they can explicitly unlock it (same unlock mechanism as existing locked patterns), at which point it behaves like any other pattern they own.

**Empty states**: If one section is empty, show a contextual empty state message:
- "My Patterns" empty: "You haven't created any patterns yet. Click 'New Pattern' to get started!"
- "Shared with Me" empty: (don't show the section at all — no need to call attention to it)

##### 5. Share Management UI (Owner)

**Pattern View page** (`internal/view/pattern_view.templ`):

Add a "Sharing" section below the action buttons (only shown for user-authored patterns, i.e., `pattern.SharedFromUserID == nil`):

- **"Share via Link" button** — POSTs to `/patterns/{id}/share` to generate a global link. If one already exists, shows the existing URL.
- **"Share with User" form** — An email input field + submit button that POSTs to `/patterns/{id}/share/email`. Creates an email-bound share link.
- **Active shares list** — Shows all active shares for the pattern:
  - Global shares: show the share URL, a "Copy Link" button, and a "Revoke" button.
  - Email shares: show the recipient email, the share URL, a "Copy Link" button, and a "Revoke" button.
- **"Revoke All" button** — Visible when there are multiple active shares. POSTs to `/patterns/{id}/share/revoke-all`.

For **received patterns** (where `pattern.SharedFromUserID != nil`), the view page shows a "Shared by {SharedFromName}" notice instead of the sharing section. No share management is available — you cannot reshare a pattern that was shared to you.

##### 6. Shared Pattern Preview Page (Share Link Landing)

**New templ** (`internal/view/shared_pattern.templ`):

This is the page shown when a user opens `/s/{token}` — it's a **preview** before saving. It has different chrome from the owner view:

- No edit/delete/duplicate buttons
- Shows the pattern owner's display name (e.g., "Shared by Alice")
- Prominent **"Save to My Library"** button that POSTs to `/s/{token}/save`
- Full pattern content: metadata, pattern text preview, instruction groups with stitch entries, images
- Standard authenticated navbar (Dashboard, Patterns, Stitch Library) since the viewer is always logged in
- If the viewer has already saved this pattern, the "Save" button is replaced with a "Already in Your Library" indicator (with a link to their copy)

This requires the handler to load the pattern owner's display name:

```go
owner, err := h.users.GetByID(ctx, pattern.UserID)
```

The view signature:

```go
templ SharedPatternPreviewPage(displayName string, pattern *domain.Pattern, ownerName string, groupImages map[int64][]domain.PatternImage, alreadySaved bool, savedPatternID int64)
```

##### 7. Image Access for Shared Patterns

Since all share routes require authentication, and the existing `GET /images/{id}` route already requires authentication, **no changes are needed** for image serving. The image ownership check in the existing handler needs to be relaxed to allow viewing images that belong to any pattern the user has access to (owns or is previewing via share token). Alternatively, since image IDs are opaque integers behind auth, the ownership check can simply be removed — the auth gate is sufficient.

When a pattern is duplicated via sharing, images should also be duplicated (the `Duplicate` method should copy image records and files so the snapshot is fully self-contained). This ensures images survive if the original pattern is deleted.

##### 8. Authorization & Business Rules

- Only the **pattern owner** can create/revoke share links.
- **Cannot reshare**: Patterns received via sharing (`SharedFromUserID != nil`) cannot have share links created for them. The sharing UI is hidden, and the service rejects attempts.
- All share viewing requires authentication — unauthenticated users hitting `/s/{token}` are redirected to the login page (standard `RequireAuth` behavior).
- Share tokens are 64 hex chars (256 bits of entropy) — unguessable.
- Email-bound shares enforce that the authenticated viewer's email matches the `RecipientEmail`. This prevents link forwarding — if Alice shares with bob@example.com, only the account registered with bob@example.com can view it.
- **Revoking** a share immediately prevents future previews and saves but does NOT affect copies already saved by recipients (those are independent snapshots owned by the recipient).
- Saved copies are **locked by default** to preserve the snapshot. Recipients can unlock to modify.
- Saved copies do NOT inherit share links — they start with zero shares.
- A shared pattern with active work sessions remains shareable — sessions belong to the owner, not the viewer.
- Deleting a pattern cascades to delete all its share links (via `ON DELETE CASCADE`). Existing saved copies in other users' libraries are unaffected (they are separate rows with their own `user_id`).
- **Duplicate save prevention**: A user cannot save the same shared pattern twice. The service checks for existing patterns with matching origin before duplicating. (Checked by querying for a pattern with the same `shared_from_user_id` and source pattern metadata, or by recording the source pattern ID in the share metadata.)

---

#### Affected files

- **New**: `internal/domain/share.go` (`PatternShare` entity, `ShareType` constants, `PatternShareRepository` interface), `internal/repository/sqlite/share.go` (repository implementation), `internal/repository/sqlite/migrations/008_pattern_sharing.sql`, `internal/service/share.go`, `internal/handler/share.go`, `internal/view/shared_pattern.templ`
- **Modified**: `internal/domain/pattern.go` (add `SharedFromUserID`, `SharedFromName` fields to `Pattern`; add `ListSharedWithUser`, `GetByShareToken` to `PatternRepository`), `internal/repository/sqlite/pattern.go` (add new columns to queries, implement new methods, extend `Duplicate` to set origin metadata and copy images), `internal/service/pattern.go` (no-reshare guard), `internal/handler/pattern.go` (`HandleList` fetches both sections + share indicators), `internal/handler/routes.go` (register share routes), `internal/view/pattern_view.templ` (share management UI for owners, "Shared by" notice for received patterns), `internal/view/pattern_list.templ` (two sections, share indicator icons), `main.go` (wire share service and handler)

#### Regression gate

All existing tests pass. New tests cover: create global share (success, non-owner → error, idempotent, reshare-received-pattern → error), create email share (success, non-owner → error, idempotent, invalid email → error, self-share → error), view shared pattern preview (valid token, invalid token → 404, revoked token → 404, owner-views-own → redirect), view email-bound share (matching email → success, non-matching email → 403), save shared pattern (creates locked duplicate with origin metadata, images copied, flash message), duplicate save prevention (second save → error), pattern list two sections (user-authored in "My Patterns", received in "Shared with Me", share indicators on authored patterns with active shares), revoke single share, revoke all shares, cascade delete on pattern delete (copies unaffected), received pattern locked by default. Pattern CRUD, work sessions, and stitch library unaffected.

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

**Note**: IMP-12 (sharing) introduces `SaveSharedPattern` in the share service, which calls `Duplicate` with origin metadata (`SharedFromUserID`, `SharedFromName`). This bypasses the ownership check since the share token serves as authorization. The existing `DuplicatePattern` method here (owner-only) remains unchanged.

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
