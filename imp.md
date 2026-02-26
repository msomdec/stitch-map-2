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

3. **`Duplicate` repository method** (`internal/repository/sqlite/pattern.go:205-229`) — Creates a full copy of a pattern (including all PatternStitches, InstructionGroups, and StitchEntries) for a different user. Currently the copy is always unlocked. This is the core mechanism for snapshotting a shared pattern — it needs to be extended to set `Locked = true`, set the `shared_from_user_id`/`shared_from_name` origin metadata, and copy images when duplicating from a share.

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
- Footer actions: **View** and **Start** only. No Edit, no Delete, no Duplicate. These are permanently read-only snapshots — the recipient can view the pattern and work through it via a work session, but cannot modify, copy, or remove it.

Received patterns are **permanently locked and read-only**. The `Locked` flag is set to `true` on save and the UI never offers an unlock option for received patterns. The handlers and service layer enforce this: edit, delete, duplicate, and unlock requests for a received pattern (`SharedFromUserID != nil`) are rejected.

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

For **received patterns** (where `pattern.SharedFromUserID != nil`), the view page shows a "Shared by {SharedFromName}" notice instead of the sharing section. The view page hides all mutation actions: no Edit, Delete, Duplicate, Unlock, or Share buttons. The only actions available are View (already on this page) and Start (to begin a work session). This enforces that received patterns are permanently read-only snapshots.

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

- Only the **pattern owner** (user-authored patterns) can create/revoke share links.
- **Received patterns are immutable**: Patterns received via sharing (`SharedFromUserID != nil`) are permanently locked and read-only. They cannot be edited, unlocked, deleted, duplicated, or reshared. The service layer rejects all mutation operations on received patterns. The only permitted actions are viewing and starting a work session.
- All share viewing requires authentication — unauthenticated users hitting `/s/{token}` are redirected to the login page (standard `RequireAuth` behavior).
- Share tokens are 64 hex chars (256 bits of entropy) — unguessable.
- Email-bound shares enforce that the authenticated viewer's email matches the `RecipientEmail`. This prevents link forwarding — if Alice shares with bob@example.com, only the account registered with bob@example.com can view it.
- **Revoking** a share immediately prevents future previews and saves but does NOT affect copies already saved by recipients (those are independent snapshots owned by the recipient).
- Saved copies are **permanently locked** — the `Locked` flag is set to `true` and cannot be changed.
- Saved copies do NOT inherit share links — they start with zero shares.
- A shared pattern with active work sessions remains shareable — sessions belong to the owner, not the viewer. Recipients can also start their own work sessions on received patterns.
- Deleting a pattern cascades to delete all its share links (via `ON DELETE CASCADE`). Existing saved copies in other users' libraries are unaffected (they are separate rows with their own `user_id`).
- **Duplicate save prevention**: A user cannot save the same shared pattern twice. The service checks for existing patterns with matching origin before duplicating. (Checked by querying for a pattern with the same `shared_from_user_id` and source pattern metadata, or by recording the source pattern ID in the share metadata.)

---

#### Design Rationale: Snapshot Duplication vs. Version-on-Save

An alternative approach was evaluated: instead of duplicating on share-save, every pattern edit would create a new version (copy-on-write), and share links would point to specific versions. All recipients of the same share would reference the same version row. This was **rejected** for the following reasons:

1. **Net storage increase** — Version-on-save trades O(N) copies for N recipients (small — most patterns shared with 1–5 people) for O(V) full copies for V saves (large — patterns are typically saved 20–50+ times during active editing). Most patterns are never shared, yet all would pay the versioning cost on every save.

2. **Disproportionate complexity** — The pattern data graph spans 4–6 tables (patterns, pattern_stitches, instruction_groups, stitch_entries, pattern_images, file_blobs). Versioning requires either a parallel `pattern_versions` table set or version-aware FKs throughout — a foundational refactor touching every existing query, not an additive change.

3. **Conflation of concerns** — Versioning (undo/history for the author) and sharing (read-only access for others) solve different problems with different triggers. Coupling them means shipping sharing requires also shipping versioning. IMP-17 already plans versioning as a standalone feature with a simpler approach (JSON snapshots).

4. **Snapshot duplication is already built** — The `Duplicate` repository method already creates a full deep copy. Extending it with origin metadata and image copying is a small, well-scoped change.

5. **Read-only immutability makes duplication ideal** — Since received patterns are permanently read-only (no editing, no unlocking, no duplicating), there is no scenario where a recipient needs to track updates from the original. A snapshot is exactly the right semantics.

Soft-delete (author hides a pattern while recipients keep access) is already solved by duplication — recipients have independent rows, so the author can hard-delete freely. If "archive without delete" is desired for the author's own UX, a simple `archived_at` column achieves this without versioning.

---

#### Affected files

- **New**: `internal/domain/share.go` (`PatternShare` entity, `ShareType` constants, `PatternShareRepository` interface), `internal/repository/sqlite/share.go` (repository implementation), `internal/repository/sqlite/migrations/008_pattern_sharing.sql`, `internal/service/share.go`, `internal/handler/share.go`, `internal/view/shared_pattern.templ`
- **Modified**: `internal/domain/pattern.go` (add `SharedFromUserID`, `SharedFromName` fields to `Pattern`; add `ListSharedWithUser`, `GetByShareToken` to `PatternRepository`), `internal/repository/sqlite/pattern.go` (add new columns to queries, implement new methods, extend `Duplicate` to set origin metadata and copy images), `internal/service/pattern.go` (no-reshare guard), `internal/handler/pattern.go` (`HandleList` fetches both sections + share indicators), `internal/handler/routes.go` (register share routes), `internal/view/pattern_view.templ` (share management UI for owners, "Shared by" notice for received patterns), `internal/view/pattern_list.templ` (two sections, share indicator icons), `main.go` (wire share service and handler)

#### Regression gate

All existing tests pass. New tests cover: create global share (success, non-owner → error, idempotent, reshare-received-pattern → error), create email share (success, non-owner → error, idempotent, invalid email → error, self-share → error), view shared pattern preview (valid token, invalid token → 404, revoked token → 404, owner-views-own → redirect), view email-bound share (matching email → success, non-matching email → 403), save shared pattern (creates permanently locked duplicate with origin metadata, images copied, flash message), duplicate save prevention (second save → error), received pattern immutability (edit → error, delete → error, duplicate → error, unlock → error, create share → error), pattern list two sections (user-authored in "My Patterns", received in "Shared with Me", share indicators on authored patterns with active shares), work session on received pattern (allowed — read-only does not prevent tracking progress), revoke single share, revoke all shares, cascade delete on pattern delete (copies unaffected). Pattern CRUD, work sessions, and stitch library unaffected.

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

### IMP-14: Pattern Import/Export (Text Format) — Rejected

**Summary**: Allow users to export patterns as text and import patterns from a structured text format.

**Status**: Rejected — this feature is not desired and will not be implemented.

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

**Problem**: Editing a pattern is destructive — the previous version is lost. The repository's `Update` method performs a full delete-and-reinsert of all instruction groups, stitch entries, and pattern stitches within a transaction (`internal/repository/sqlite/pattern.go:166-227`). If a user accidentally removes instruction groups, reorders stitches incorrectly, or saves an unintended change, the previous state is irrecoverable. There is no undo, no history, and no safety net.

**Goal**: Automatically capture a version snapshot every time a pattern is saved, creating a browsable version history. Users can view any previous version in a read-only preview and restore it as the current state. Versioning is transparent — no extra "save version" step is needed; every save is automatically versioned.

---

#### Existing Foundation

1. **Full delete-and-reinsert on save** (`internal/repository/sqlite/pattern.go:166-227`) — The `Update` method begins a transaction, deletes all groups/entries/pattern stitches, then re-inserts them. This means the "before" state is naturally available: the service already loads the existing pattern via `GetByID` before calling `Update` (`internal/service/pattern.go:53-56`). The loaded pattern is the perfect snapshot source.

2. **Self-contained `PatternStitch` model** (migration 007, `internal/domain/pattern.go:33-41`) — Patterns carry their own stitch definitions in `pattern_stitches`, decoupled from the global library. A JSON snapshot of the pattern is therefore fully self-contained — no dangling references to library stitches that might later change or be deleted.

3. **Read-only pattern view** (`internal/handler/pattern.go:93-131`, `internal/view/pattern_view.templ`) — The existing `PatternViewPage` renders a full read-only view with all groups, entries, pattern text preview, and images. The version preview page can reuse this rendering logic with minor adaptations (different header, no action buttons, restore button instead).

4. **`RenderPatternText` function** (`internal/service/pattern.go`) — Already produces a human-readable text representation of a pattern. This can be applied to deserialized version snapshots to show a text preview on the version history page, giving users a quick way to identify versions without opening each one.

5. **Domain struct fields are all exported** (`internal/domain/pattern.go`) — `Pattern`, `InstructionGroup`, `StitchEntry`, and `PatternStitch` all have exported fields with simple types (`int64`, `string`, `bool`, `time.Time`, slices). They serialize cleanly to JSON. No JSON struct tags exist currently, so dedicated snapshot types with explicit JSON tags are needed.

---

#### What's Missing

##### 1. Snapshot Domain Model & Storage

**Snapshot types** (`internal/domain/version.go`):

Define dedicated JSON-serializable types for version snapshots. These are separate from the main domain types to avoid coupling JSON serialization concerns to the core model and to give explicit control over what's included:

```go
// PatternVersion represents a single saved version of a pattern.
type PatternVersion struct {
    ID            int64
    PatternID     int64
    VersionNumber int       // Auto-incrementing per pattern, starting at 1
    Snapshot      string    // JSON-encoded PatternSnapshot
    CreatedAt     time.Time
}

// PatternSnapshot is the JSON-serializable representation of a pattern's
// full state at a point in time. Images are NOT included — they are managed
// separately and are not versioned.
type PatternSnapshot struct {
    Name              string                     `json:"name"`
    Description       string                     `json:"description"`
    PatternType       PatternType                `json:"pattern_type"`
    HookSize          string                     `json:"hook_size"`
    YarnWeight        string                     `json:"yarn_weight"`
    Difficulty        string                     `json:"difficulty"`
    PatternStitches   []PatternStitchSnapshot    `json:"pattern_stitches"`
    InstructionGroups []InstructionGroupSnapshot  `json:"instruction_groups"`
}

type PatternStitchSnapshot struct {
    Abbreviation    string `json:"abbreviation"`
    Name            string `json:"name"`
    Description     string `json:"description"`
    Category        string `json:"category"`
    LibraryStitchID *int64 `json:"library_stitch_id,omitempty"`
}

type InstructionGroupSnapshot struct {
    SortOrder     int                    `json:"sort_order"`
    Label         string                 `json:"label"`
    RepeatCount   int                    `json:"repeat_count"`
    ExpectedCount *int                   `json:"expected_count,omitempty"`
    Notes         string                 `json:"notes,omitempty"`
    StitchEntries []StitchEntrySnapshot  `json:"stitch_entries"`
}

type StitchEntrySnapshot struct {
    SortOrder       int    `json:"sort_order"`
    PatternStitchIdx int   `json:"pattern_stitch_idx"` // Index into PatternStitches slice
    Count           int    `json:"count"`
    IntoStitch      string `json:"into_stitch,omitempty"`
    RepeatCount     int    `json:"repeat_count"`
}
```

`StitchEntrySnapshot.PatternStitchIdx` stores the **index** into the `PatternStitches` slice (not a database ID). This makes the snapshot fully self-referential — no external IDs needed to reconstruct the pattern.

**Helper functions** (on `PatternSnapshot`):

```go
// SnapshotFromPattern creates a PatternSnapshot from a loaded domain.Pattern.
func SnapshotFromPattern(p *Pattern) PatternSnapshot { ... }

// ToPattern reconstructs a domain.Pattern from a snapshot. The returned pattern
// has ID=0, no UserID, no timestamps — it's a template for restoration.
func (s PatternSnapshot) ToPattern() *Pattern { ... }

// Summary returns a human-readable summary: group count, total stitch count.
func (s PatternSnapshot) Summary() string { ... }
```

**New `PatternVersionRepository` interface** (`internal/domain/version.go`):

```go
type PatternVersionRepository interface {
    // Create stores a new version. VersionNumber is computed as MAX(version_number)+1
    // for the pattern. If the pattern already has MaxVersions versions, the oldest
    // is deleted before inserting.
    Create(ctx context.Context, version *PatternVersion) error

    // ListByPattern returns all versions for a pattern, ordered by version_number DESC
    // (newest first). Returns version metadata only — Snapshot field is empty.
    ListByPattern(ctx context.Context, patternID int64) ([]PatternVersion, error)

    // GetByPatternAndVersion returns a specific version with its full Snapshot.
    GetByPatternAndVersion(ctx context.Context, patternID int64, versionNumber int) (*PatternVersion, error)

    // DeleteByPattern removes all versions for a pattern. Called when a pattern
    // is deleted (also handled by ON DELETE CASCADE, but explicit for clarity).
    DeleteByPattern(ctx context.Context, patternID int64) error

    // CountByPattern returns the number of versions for a pattern.
    CountByPattern(ctx context.Context, patternID int64) (int, error)

    // DeleteOldest deletes the oldest N versions for a pattern (by version_number ASC).
    DeleteOldest(ctx context.Context, patternID int64, count int) error
}
```

**New migration** (`internal/repository/sqlite/migrations/009_pattern_versions.sql`):

```sql
CREATE TABLE IF NOT EXISTS pattern_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pattern_id INTEGER NOT NULL REFERENCES patterns(id) ON DELETE CASCADE,
    version_number INTEGER NOT NULL,
    snapshot TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(pattern_id, version_number)
);

CREATE INDEX IF NOT EXISTS idx_pattern_versions_pattern ON pattern_versions(pattern_id);
CREATE INDEX IF NOT EXISTS idx_pattern_versions_lookup ON pattern_versions(pattern_id, version_number);
```

`ON DELETE CASCADE` ensures that deleting a pattern automatically removes all its version history. The `UNIQUE(pattern_id, version_number)` constraint guarantees version numbers are unique per pattern.

**Repository implementation** (`internal/repository/sqlite/version.go`):

Standard CRUD against the `pattern_versions` table. `Create` computes the next version number with `SELECT COALESCE(MAX(version_number), 0) + 1 FROM pattern_versions WHERE pattern_id = ?`. `ListByPattern` returns rows ordered by `version_number DESC` with `snapshot` omitted from the SELECT (for performance — snapshots can be large). `GetByPatternAndVersion` fetches the full row including `snapshot`. `DeleteOldest` uses `DELETE FROM pattern_versions WHERE pattern_id = ? AND version_number IN (SELECT version_number FROM pattern_versions WHERE pattern_id = ? ORDER BY version_number ASC LIMIT ?)`.

##### 2. Version Service

**New `VersionService`** (`internal/service/version.go`):

```go
type VersionService struct {
    versions    domain.PatternVersionRepository
    patterns    domain.PatternRepository
    maxVersions int // configurable cap, default 50
}

func NewVersionService(
    versions domain.PatternVersionRepository,
    patterns domain.PatternRepository,
    maxVersions int,
) *VersionService { ... }
```

Methods:

- `CreateVersion(ctx context.Context, pattern *domain.Pattern) error` — Takes a loaded `*Pattern`, serializes it to a `PatternSnapshot` via `SnapshotFromPattern`, JSON-encodes it, and calls `versions.Create`. If the pattern already has `maxVersions` versions, the oldest is pruned. This method is called by `PatternService.Update` after loading the existing pattern and before writing changes. It is **not** called for new patterns (Create) since there's no prior state to snapshot.

- `ListVersions(ctx context.Context, userID, patternID int64) ([]domain.PatternVersion, error)` — Verifies ownership (loads the pattern, checks `UserID == userID`), then returns `versions.ListByPattern`. Returns version metadata only (no snapshots) for the history page.

- `GetVersion(ctx context.Context, userID, patternID int64, versionNumber int) (*domain.PatternVersion, *domain.PatternSnapshot, error)` — Verifies ownership, loads the version, deserializes the JSON snapshot into a `PatternSnapshot`, and returns both. The snapshot is used to render the version preview page.

- `RestoreVersion(ctx context.Context, userID, patternID int64, versionNumber int) error` — The core restore operation:
  1. Loads the current pattern via `patterns.GetByID` and verifies ownership.
  2. Rejects if the pattern is locked (`Locked == true`) or is a received shared pattern (`SharedFromUserID != nil`) — locked/shared patterns cannot be modified.
  3. Snapshots the current state by calling `CreateVersion` (so the pre-restore state is preserved and the restore itself is undoable).
  4. Loads the target version via `versions.GetByPatternAndVersion`.
  5. Deserializes the snapshot and calls `snapshot.ToPattern()` to get a template `*Pattern`.
  6. Copies the restored fields onto the existing pattern (name, description, type, hook size, yarn weight, difficulty, pattern stitches, instruction groups, stitch entries).
  7. Calls `patterns.Update` with the restored pattern.

  This means restoring version N creates version N+M (a snapshot of the state just before restoration), then applies version N's data as the current state. The user can always "undo" a restore by restoring the version created in step 3.

- `GetVersionForPreview(ctx context.Context, userID, patternID int64, versionNumber int) (*domain.Pattern, string, error)` — Convenience method for the handler. Verifies ownership, loads the version, deserializes the snapshot, calls `snapshot.ToPattern()`, and also calls `RenderPatternText` on the reconstructed pattern to produce a text preview. Returns the reconstructed pattern and the text preview string.

**Version cap and pruning**:

The `maxVersions` cap (default 50, configurable via `MAX_PATTERN_VERSIONS` environment variable) prevents unbounded storage growth. When `CreateVersion` is called and the pattern already has `maxVersions` versions, the oldest version is deleted before the new one is inserted. This is a simple FIFO strategy. The cap applies per-pattern, not globally.

Rationale for the default of 50: a typical pattern might be saved 20–50 times during active development. A cap of 50 preserves the full editing history for most patterns while keeping storage bounded. At approximately 2–5KB per snapshot (a medium-complexity pattern with 10 groups and 50 stitch entries), 50 versions is ~100–250KB per pattern — negligible for SQLite.

##### 3. Integration with Pattern Update Flow

**`PatternService.Update` changes** (`internal/service/pattern.go`):

The existing `Update` method currently:
1. Loads existing pattern via `GetByID`
2. Checks ownership, shared-pattern lock, explicit lock
3. Validates the new pattern
4. Resolves pattern stitches
5. Calls `patterns.Update`

The change adds a single call between steps 2 and 3:

```go
func (s *PatternService) Update(ctx context.Context, userID int64, pattern *domain.Pattern) error {
    existing, err := s.patterns.GetByID(ctx, pattern.ID)
    if err != nil {
        return fmt.Errorf("get pattern: %w", err)
    }
    if existing.UserID != userID {
        return domain.ErrUnauthorized
    }
    if existing.SharedFromUserID != nil {
        return domain.ErrPatternLocked
    }
    if existing.Locked {
        return domain.ErrPatternLocked
    }

    // NEW: Snapshot the current state before applying changes.
    if s.versions != nil {
        if err := s.versions.CreateVersion(ctx, existing); err != nil {
            // Log the error but don't fail the save — versioning is a
            // convenience feature, not a gate on saving.
            slog.Error("failed to create pattern version", "pattern_id", pattern.ID, "error", err)
        }
    }

    if err := validate(pattern); err != nil {
        return err
    }
    // ... rest unchanged
}
```

The `versions` field is a `*VersionService` added to `PatternService`. It is `nil`-safe — if not injected (e.g., in tests that don't care about versioning), the versioning step is silently skipped. This keeps existing tests passing without modification.

**Important design decision**: Version creation failure does **not** fail the save. Versioning is a safety net, not a gate. If snapshot creation fails (e.g., JSON marshaling error, disk full), the save proceeds and the error is logged. This ensures versioning never blocks the user from saving their work.

##### 4. Version Routes & Handler

**Handler** (`internal/handler/version.go`):

```go
type VersionHandler struct {
    versions *service.VersionService
    patterns *service.PatternService
}
```

Routes:

- `GET /patterns/{id}/versions` — **Version History page**. Loads the pattern (ownership check), calls `versions.ListVersions`, renders the version history page.
- `GET /patterns/{id}/versions/{version}` — **Version Preview page**. Loads the specific version, renders a read-only view of the pattern at that point in time.
- `POST /patterns/{id}/versions/{version}/restore` — **Restore a version**. Calls `versions.RestoreVersion`, redirects to `/patterns/{id}` (the pattern view page) with a flash message ("Restored to version {N}").

**Route registration** (`internal/handler/routes.go`):

```go
// Pattern version history (authenticated, owner only)
mux.Handle("GET /patterns/{id}/versions", RequireAuth(auth, http.HandlerFunc(versionHandler.HandleVersionList)))
mux.Handle("GET /patterns/{id}/versions/{version}", RequireAuth(auth, http.HandlerFunc(versionHandler.HandleVersionView)))
mux.Handle("POST /patterns/{id}/versions/{version}/restore", RequireAuth(auth, http.HandlerFunc(versionHandler.HandleRestore)))
```

##### 5. UI — Version History Page

**New templ** (`internal/view/pattern_versions.templ`):

The version history page shows a list of all saved versions for a pattern:

- **Page header**: "Version History — {Pattern Name}" with a "Back to Pattern" link.
- **Version list**: A table or card list, one row per version, ordered newest first:
  - **Version number**: "v{N}" (e.g., "v12")
  - **Timestamp**: "Saved on Feb 20, 2026 at 3:45 PM" — human-readable, with relative time ("2 hours ago") for recent versions
  - **Summary**: "{M} groups, {N} total stitches" — computed from the snapshot metadata (stored in `PatternSnapshot.Summary()` or computed at display time from the snapshot)
  - **Actions**: "View" (link to version preview page), "Restore" (POST button with confirmation)
- **Empty state**: "No version history yet. Versions are created automatically each time you save the pattern." (This state only appears for patterns created before versioning was added, or patterns that have only been saved once.)
- **Version cap notice**: If the pattern has reached the maximum version count, show a subtle note: "Showing the last {maxVersions} versions. Older versions are automatically removed."

The "Restore" button on each version row triggers a confirmation modal (consistent with the existing save confirmation pattern in the editor): "Restore version {N} from {timestamp}? Your current pattern will be saved as a new version before restoring."

**Templ signature**:

```go
templ PatternVersionsPage(
    displayName string,
    pattern *domain.Pattern,
    versions []domain.PatternVersion,
    summaries map[int]string, // version_number → summary string
    maxVersions int,
)
```

##### 6. UI — Version Preview Page

**New templ** (`internal/view/pattern_version_view.templ`):

Renders a read-only view of the pattern as it existed at a specific version. Visually similar to `PatternViewPage` but with distinct chrome:

- **Header**: "Version {N} — {Pattern Name}" with "Saved on {timestamp}"
- **Banner**: A colored info banner at the top: "You are viewing a previous version of this pattern. This is a read-only preview."
- **Pattern content**: Renders the full pattern from the deserialized snapshot — metadata, instruction groups with stitch entries, and pattern text preview. Reuses the same rendering components as the current pattern view (extracted into shared templ components if not already).
- **Images note**: If the current pattern has images, show a note: "Images reflect the current version and may differ from when this version was saved." (Since images are not versioned.)
- **Actions**:
  - "Restore this Version" button (POST to `/patterns/{id}/versions/{version}/restore` with confirmation modal)
  - "Back to Version History" link
  - "Back to Current Version" link (to `/patterns/{id}`)
- **No edit/delete/duplicate/share buttons** — this is purely a read-only preview.

**Templ signature**:

```go
templ PatternVersionViewPage(
    displayName string,
    originalPattern *domain.Pattern, // for context (current name, ID)
    version *domain.PatternVersion,
    snapshot *domain.PatternSnapshot,
    restoredPattern *domain.Pattern,  // the snapshot reconstructed as a Pattern (for rendering)
    patternText string,               // pre-rendered text preview
)
```

##### 7. UI — Version History Access Point

**Pattern View page changes** (`internal/view/pattern_view.templ`):

Add a "Version History" link/button to the action bar on the pattern view page, next to the existing Edit/Duplicate/Start Session buttons. Only shown for user-authored patterns (`SharedFromUserID == nil`) — received patterns don't have version history since they were never edited by the recipient.

Optionally show the version count as a badge: "History (12)" to give users a sense of how many versions exist.

**Pattern Editor page changes** (`internal/view/pattern_editor.templ`):

No changes to the editor itself. The version is created automatically on save — the user doesn't need to interact with versioning from the editor. A subtle text note below the save button could say "All changes are automatically versioned" for discoverability, but this is optional.

##### 8. Snapshot Serialization Details

**What IS captured** in the snapshot:
- Pattern metadata: name, description, pattern type, hook size, yarn weight, difficulty
- All `PatternStitch` records: abbreviation, name, description, category, library stitch ID reference
- All `InstructionGroup` records: sort order, label, repeat count, expected count, notes
- All `StitchEntry` records: sort order, pattern stitch index, count, into stitch, repeat count

**What is NOT captured**:
- **Images**: `pattern_images` and `file_blobs` are not included in snapshots. Images are large binary data that would bloat the version table significantly. Restoring a version does not affect images — they remain as they are on the current pattern. This is an acceptable trade-off: images change less frequently than stitch instructions, and the primary use case for versioning is recovering stitch/group changes. A future enhancement could add image versioning as a separate feature.
- **Pattern ID, User ID, timestamps**: These are pattern identity/metadata, not content. They don't change between versions.
- **Locked flag, SharedFromUserID/SharedFromName**: These are access control fields, not pattern content. Restoring a version should not change whether a pattern is locked or shared.
- **Work session state**: Sessions track a user's progress through a pattern. They are independent of the pattern's version history.

**JSON size estimates**: A medium-complexity pattern (10 instruction groups, 5 stitch entries per group, 15 unique pattern stitches) produces approximately 3–5KB of JSON. A very complex pattern (50 groups, 10 entries each, 30 pattern stitches) produces approximately 15–25KB. At 50 versions, that's 150KB–1.25MB per pattern — well within SQLite's comfort zone.

##### 9. Restoration Mechanics

When restoring version N:

1. The current pattern state is snapshotted as a new version (version M+1, where M is the current highest version). This ensures the restore is undoable.
2. The snapshot from version N is deserialized into a `PatternSnapshot`.
3. `PatternSnapshot.ToPattern()` reconstructs a `*domain.Pattern` with:
   - `PatternStitches` rebuilt from `PatternStitchSnapshot` entries, each with `ID = 0` (so the repository treats them as new inserts).
   - `InstructionGroups` rebuilt with `StitchEntries` referencing the **index** into the `PatternStitches` slice (matching the convention used by `resolvePatternStitches` in the service layer, and by `insertPatternStitches` / `insertGroups` in the repository).
   - All IDs set to 0 so the repository's delete-and-reinsert logic handles them as fresh inserts.
4. The reconstructed pattern's content fields are copied onto the existing loaded pattern (preserving `ID`, `UserID`, `Locked`, `SharedFromUserID`, etc.).
5. The pattern is saved via `PatternRepository.Update`, which performs its normal delete-and-reinsert within a transaction.

**The restore bypasses `resolvePatternStitches`** in the service layer. Since the snapshot already contains fully resolved pattern stitches (not library stitch IDs), the service's stitch resolution step must be skipped for restores. This is handled by having `RestoreVersion` call `patterns.Update` directly (via the repository) rather than going through `PatternService.Update`, or by adding a flag/separate method on `PatternService` that skips stitch resolution. The recommended approach is a `PatternService.RestoreFromSnapshot(ctx, userID, patternID, snapshot)` method that handles the restore-specific logic.

```go
// RestoreFromSnapshot applies a snapshot to an existing pattern, bypassing
// stitch resolution (since the snapshot already contains resolved stitches).
func (s *PatternService) RestoreFromSnapshot(ctx context.Context, userID int64, patternID int64, snapshot *domain.PatternSnapshot) error {
    existing, err := s.patterns.GetByID(ctx, patternID)
    if err != nil {
        return fmt.Errorf("get pattern: %w", err)
    }
    if existing.UserID != userID {
        return domain.ErrUnauthorized
    }
    if existing.SharedFromUserID != nil || existing.Locked {
        return domain.ErrPatternLocked
    }

    restored := snapshot.ToPattern()
    existing.Name = restored.Name
    existing.Description = restored.Description
    existing.PatternType = restored.PatternType
    existing.HookSize = restored.HookSize
    existing.YarnWeight = restored.YarnWeight
    existing.Difficulty = restored.Difficulty
    existing.PatternStitches = restored.PatternStitches
    existing.InstructionGroups = restored.InstructionGroups

    return s.patterns.Update(ctx, existing)
}
```

##### 10. Authorization & Business Rules

- Only the **pattern owner** can view version history, view individual versions, or restore versions. Ownership is verified by loading the pattern and checking `UserID == userID`.
- **Received patterns** (`SharedFromUserID != nil`) have no version history and cannot be restored. The version history link is not shown on the view page for received patterns.
- **Locked patterns** cannot be restored (restore modifies the pattern). The version history can still be *viewed* for locked patterns, but the "Restore" buttons are disabled/hidden. This allows users to see past versions even if the pattern is currently locked.
- Version creation failure does **not** block pattern saves. Versioning is best-effort. Errors are logged via `slog.Error`.
- The version cap (default 50) is enforced per pattern. When the cap is reached, the oldest version is pruned before a new one is inserted. This is a hard cap — there is no way to exceed it.
- Restoring a version always creates a new version first (capturing the pre-restore state). This means a restore consumes one version slot toward the cap.
- Deleting a pattern cascades to delete all its versions (via `ON DELETE CASCADE`).
- **Work sessions are not affected by version restoration.** If a user is mid-session on a pattern and restores a version, the session's position indices may no longer align with the restored pattern structure. The work session tracker already handles out-of-bounds indices gracefully (redirecting to the start of the pattern). An optional enhancement: detect when a pattern has been modified since the session's `last_activity_at` and warn the user. This is a future improvement, not required for v1.

---

#### Design Rationale

##### JSON Snapshots vs. Relational Versioning

Two approaches were considered:

**Approach A — JSON snapshots** (chosen): Store a JSON blob of the entire pattern state in a single `pattern_versions` row. Simple schema (one table, one blob column). Easy to create, easy to restore, easy to reason about. The snapshot is self-contained and immutable.

**Approach B — Relational versioning**: Create versioned copies of all related tables (`patterns_v`, `instruction_groups_v`, `stitch_entries_v`, `pattern_stitches_v`) with a `version_id` FK. More normalized but significantly more complex — every query that touches versions must join across 4+ tables, restoration requires multi-table inserts, and the version schema must evolve in lockstep with the main schema.

JSON snapshots were chosen because:

1. **Simplicity**: One table, one column. No version-aware FKs, no parallel table sets, no multi-table joins for version reads.
2. **Decoupled from schema evolution**: If the main schema changes (new columns on `patterns`, new fields on `stitch_entries`), old version snapshots remain valid — they simply omit the new fields. Deserialization with `json.Unmarshal` naturally handles missing fields by leaving them at zero values. New fields can be added to the snapshot type with `omitempty` tags, and old snapshots remain readable.
3. **Performance**: Creating a version is a single INSERT with a pre-serialized JSON string. Reading a version list is a single indexed SELECT. The hot path (saving a pattern) adds only one INSERT — no multi-table copying.
4. **Storage is bounded**: The per-pattern version cap (default 50) and the modest size of JSON snapshots (3–25KB each) keep total storage well-bounded. A user with 100 patterns, each at the 50-version cap, uses approximately 15–125MB of version storage — manageable for SQLite.
5. **Already consistent with the IMP-12 design rationale**: The IMP-12 sharing design explicitly chose snapshot duplication over relational versioning (see "Design Rationale: Snapshot Duplication vs. Version-on-Save" in IMP-12). Using JSON snapshots for versioning maintains this architectural consistency.

##### Automatic vs. Manual Versioning

Versions are created **automatically on every save**, not manually by the user. Rationale:

1. **No cognitive overhead**: Users don't need to remember to "save a version" before making changes. Every save is versioned.
2. **Matches the problem statement**: The problem is accidental destructive edits. Automatic versioning catches these by default, whereas manual versioning only helps if the user anticipated the need.
3. **Simplicity**: No UI for "create version," "name this version," or "choose what to version." The feature is invisible during normal use and only surfaces when the user needs to recover.

The trade-off is that automatic versioning creates versions for trivial changes (fixing a typo in a pattern name). The version cap mitigates unbounded growth, and the per-version storage cost is small enough that this trade-off is acceptable.

##### Images Not Versioned

Images are excluded from version snapshots because:

1. **Size**: Image file blobs can be 100KB–5MB each. Including them in JSON snapshots would increase version storage by 10–100x.
2. **Frequency of change**: Images change less frequently than stitch instructions. Most versioning use cases involve recovering stitch/group changes, not images.
3. **Complexity**: Image restoration would require re-inserting `file_blobs` rows and updating `pattern_images` FKs — significantly more complex than restoring text data.
4. **Acceptable trade-off**: When restoring a version, images remain as they are on the current pattern. Users can re-upload images if needed after a restore.

---

#### Affected Files

- **New**: `internal/domain/version.go` (`PatternVersion`, `PatternSnapshot` and related snapshot types, `PatternVersionRepository` interface, `SnapshotFromPattern`/`ToPattern`/`Summary` helpers), `internal/repository/sqlite/version.go` (repository implementation), `internal/repository/sqlite/migrations/009_pattern_versions.sql`, `internal/service/version.go` (`VersionService`), `internal/handler/version.go` (`VersionHandler`), `internal/view/pattern_versions.templ` (version history page), `internal/view/pattern_version_view.templ` (version preview page)
- **Modified**: `internal/domain/pattern.go` (add `RestoreFromSnapshot` to `PatternRepository` interface or keep it in the service layer only), `internal/service/pattern.go` (add `versions *VersionService` field to `PatternService`, call `CreateVersion` in `Update`, add `RestoreFromSnapshot` method), `internal/handler/routes.go` (register version routes), `internal/view/pattern_view.templ` (add "Version History" link to action bar for user-authored patterns), `main.go` (wire `VersionService` and `VersionHandler`, read `MAX_PATTERN_VERSIONS` env var)

#### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MAX_PATTERN_VERSIONS` | `50` | Maximum number of versions retained per pattern. Oldest versions are pruned when the cap is reached. |

#### Regression Gate

All existing tests pass. New tests cover: automatic version creation on pattern update (save pattern, verify version count incremented, verify snapshot content matches pre-update state), version list retrieval (correct ordering, no snapshot in list results, ownership check), version view (correct snapshot deserialization, ownership check, non-existent version → 404), restore version (pre-restore state snapshotted as new version, pattern content matches restored version, images unchanged, locked pattern → error, shared pattern → error, ownership check), version cap enforcement (save 51 times with cap=50, verify oldest version pruned, exactly 50 versions remain), restore-then-undo (restore v3, verify new version created, restore that new version to get back to pre-restore state), snapshot serialization roundtrip (create pattern → snapshot → JSON → deserialize → ToPattern → verify all fields match), empty version history (newly created pattern has zero versions, version list returns empty), pattern deletion cascades (delete pattern → verify all versions deleted), `RenderPatternText` on restored pattern (text output matches original). Pattern CRUD, work sessions, sharing, stitch library, and image management unaffected.
