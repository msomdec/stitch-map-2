## Planned Improvements

These are post-v1 improvements to be implemented incrementally. Each item should be fully implemented, tested, and regression-checked before moving to the next.

---

### IMP-4: Custom CSS Design System (Replace Bulma) (Idea — Not Scheduled)

> **Status: NOT READY TO IMPLEMENT** — Design direction must be fully defined before any work begins. Do not implement any part of this improvement until the design specification below is complete and the placeholder notes have been replaced with concrete decisions.

**Problem**: Bulma provides a generic utility-class look that does not reflect the desired visual identity of StitchMap. The goal is a custom CSS design system with a bright, sleek, and modern aesthetic.

**Design Direction** *(to be defined)*:

The following questions must be answered and this section updated before implementation begins:

- **Color palette**: What are the primary, secondary, accent, background, surface, and text colors? (Target: bright, not dark-mode-first)
- **Typography**: What font(s) should be used? Google Fonts, system stack, or self-hosted? What is the type scale?
- **Component inventory**: Which Bulma components are actively used in the app and must have custom equivalents? (navbar, buttons, cards, form fields, notifications, progress bars, modals, tags, etc.)
- **Layout system**: CSS Grid, Flexbox, or a custom column system? What are the breakpoints?
- **Spacing & sizing scale**: What is the base unit (e.g., 4px or 8px grid)?
- **Border radius & elevation**: Rounded corners? Box shadows or flat design?
- **Interactive states**: How do buttons, inputs, and links behave on hover, focus, and active?
- **Delivery method**: Single `static/style.css` file served from the Go app? Or still CDN-loaded?

**Implementation notes** *(to be filled in once design is settled)*:

- Bulma is currently loaded via CDN in `view/layout.templ`. Removing it is a single-line change, but every template that relies on Bulma class names will need to be updated.
- Custom CSS should be served as a static file from the Go app (already has a `static/` directory) rather than from a CDN, so the design is fully self-contained.
- No CSS preprocessors or build tools — plain CSS only, consistent with the project's no-build-step philosophy.
- All Bulma helper classes used across `.templ` files must be catalogued before removal to ensure nothing is missed.

---

### IMP-5: Integration Test Layer — SSE Body & HTML Structure Assertions (Idea — Not Scheduled)

**Problem**: The existing `net/http/httptest` integration tests verify HTTP status codes and redirect targets but do not inspect SSE response payloads or the HTML structure of rendered fragments. This means Datastar wiring bugs (wrong target element ID, missing field in a re-rendered form, incorrect SSE event type) go undetected until manual testing.

**Approach**: Add `github.com/PuerkitoBio/goquery` as a test-only dependency. `goquery` is a pure-Go HTML parser with a jQuery-style selector API, widely used in the Go ecosystem. It adds no runtime overhead and requires no build tooling.

**What to assert with this layer**:
- SSE response bodies contain the expected `data-swap-target` / element ID being patched.
- Re-rendered form fragments contain the correct pre-populated field values after a validation error.
- Required field indicators are present in rendered form HTML.
- Session cards on the dashboard contain the pattern name, status badge, and position summary.
- Settings page sections render the current user's display name and email pre-populated.

**Scope**: Test helpers only — no new test binaries or separate test packages. Extend the existing `newTestServices` pattern with a `parseSSE(body string) *goquery.Document` helper that extracts the HTML payload from an SSE event and returns a queryable document.

**What this layer does NOT cover**: browser-executed JavaScript, keyboard/touch event handlers, `beforeunload` lifecycle events, or visual layout. Those are tracked separately in IMP-6.

**Dependency addition**: `github.com/PuerkitoBio/goquery` — add to `go.mod` as a direct dependency (it is used in `_test.go` files, but Go does not distinguish test-only module dependencies).

--- 

### IMP-6: User Settings Page

**Goal**: Authenticated users can update their account details from a dedicated settings page, accessed via the user dropdown in the top-right navbar.

**Entry point**: Add a "Settings" item to the authenticated user dropdown menu in `view/layout.templ`, linking to `GET /settings`.

**Editable fields** (all current user-level data points that can meaningfully be changed):

1. **Display Name** — straightforward update; re-issue JWT on save so the new display name is reflected in the token claims immediately.
2. **Email Address** — must check uniqueness against existing users before saving; re-issue JWT on success since email is embedded in token claims. Treat a changed email as a sensitive operation: require the user to confirm their current password before the change is applied.
3. **Password** — require the user to enter their current password for verification, then enter and confirm a new password. Bcrypt the new password at the configured cost before storing.

**Read-only info to display** (not editable, but useful context):
- Account created date (`CreatedAt`)

**Page structure** (`view/settings.templ`):
- Three separate form sections (or Bulma `box` panels), each with its own save button and independent SSE submission:
  - "Display Name" section
  - "Email Address" section (with current-password confirmation field)
  - "Change Password" section (current password + new password + confirm new password)
- Each section shows its own inline success/error feedback via Datastar SSE without affecting the other sections.
- Forms retain entered values on error.
- Required fields marked.

**Service layer** (`service/auth.go`):
- `UpdateDisplayName(ctx, userID, newDisplayName) (*User, string, error)` — updates display name, returns updated user and a fresh JWT.
- `UpdateEmail(ctx, userID, currentPassword, newEmail) (*User, string, error)` — verifies current password, checks email uniqueness, updates email, returns updated user and a fresh JWT.
- `UpdatePassword(ctx, userID, currentPassword, newPassword, confirmPassword) error` — verifies current password, validates new password match and strength, bcrypts and stores.

**Handler** (`handler/settings.go`):
- `GET /settings` — renders the settings page pre-populated with current user data.
- `POST /settings/display-name` — updates display name, re-sets `auth_token` cookie with new JWT.
- `POST /settings/email` — updates email, re-sets `auth_token` cookie with new JWT.
- `POST /settings/password` — updates password, no JWT change needed (password is not in token claims).

**Regression gate**: All existing tests pass. New integration test covers: update display name → verify JWT updated; update email (wrong password → error, duplicate email → error, success → JWT updated); update password (wrong current → error, mismatch confirm → error, success).

---

### IMP-7: Pattern Editor — Page Overhaul

**Problem**: The pattern editor page has accumulated UX clutter — redundant fields, confusing labels, an always-visible preview that wastes horizontal space, and no safeguards against accidental data loss. This improvement overhauls the editor into a cleaner, full-width layout with better terminology and user protection.

**Changes**:

#### Pattern Metadata Section

1. **Add a "Difficulty" dropdown** (optional). Values: `Beginner`, `Intermediate`, `Advanced`, `Expert`. Stored as a new `difficulty` column (`TEXT NOT NULL DEFAULT ''`) on the `patterns` table (new migration). The domain `Pattern` struct gains a `Difficulty string` field.
2. **Remove the Notes field.** The Description textarea already covers this logical space. Drop `notes` from the editor form. The `notes` column remains in the database (no destructive migration) but the editor no longer reads or writes it for new/edited patterns.

#### Instruction Groups → "Pattern Overview"

3. **Rename the "Instruction Groups" heading** to **"Pattern Overview"**.
4. **Rename "Group Label"** to **"Part Name"** — the input placeholder should update accordingly (e.g., "e.g., Brim, Body, Round 1").
5. **Rename "Repeat" on the group (part) level** to **"Quantity"**. The stitch entry "Repeat" column keeps its current name — only the group-level field changes.
6. **Remove "Expected Count"** from group fields entirely (both the input and the derived-count helper text). The derived count calculation (`derivedExpectedCount`) can remain in code for potential future use but should not appear in the UI.

#### Stitch Entries

7. **Remove the "Into Stitch" column.** Drop the input and the column header from the entry row. The `into_stitch` column remains in the database but the editor no longer reads or writes it.
8. **Remove the "Notes" column from stitch entries entirely.** Unlike other removals, this is a full removal — drop the `notes` column from the `stitch_entries` table via migration (`ALTER TABLE stitch_entries DROP COLUMN notes`; SQLite 3.35.0+ supports this). Remove the field from the `StitchEntry` domain struct and all repository code. Stitch-level notes added no value — per-entry context belongs at the part level instead.
9. **Add a "Notes" field to each part (instruction group).** This is a new optional textarea within each part section, allowing users to attach free-form notes to a part (e.g., "work in BLO for this section", "join with sl st at end"). Stored as a new `notes` column (`TEXT NOT NULL DEFAULT ''`) on the `instruction_groups` table (included in the same migration as the stitch entry notes removal). The domain `InstructionGroup` struct gains a `Notes string` field.
10. **Stitch entry columns after cleanup**: Stitch (dropdown, required), Count (number, required), Repeat (number, required). This simplifies the entry row to three columns.

#### Numeric Inputs

11. **Hide native browser spinner arrows** on all `<input type="number">` fields in the editor. Add CSS: `input[type=number]::-webkit-inner-spin-button, input[type=number]::-webkit-outer-spin-button { -webkit-appearance: none; margin: 0; }` and `input[type=number] { -moz-appearance: textfield; }`. This can go in a `<style>` block in the layout or in a static CSS file.

#### Dynamic Add/Remove

12. **Users must be able to add and remove stitch entries within each part.** Each part section should have an "Add Stitch" button that appends a new empty entry row. Each entry row should have a remove/delete button (e.g., an `×` icon) to remove that entry — unless it is the only entry in the part (minimum 1 entry per part).
13. **Users must be able to add and remove parts.** An "Add Part" button below the last part section appends a new empty part. Each part should have a remove/delete button — unless it is the only part (minimum 1 part per pattern). Adding and removing parts and entries is done via Datastar SSE (server round-trip to re-render the relevant DOM fragment), consistent with the existing interaction pattern.

#### Preview → Modal

14. **Remove the always-visible preview panel** from the right column. The editor form should take the full page width (remove the `columns` wrapper that creates the 8/4 split).
15. **Add a "Preview" button** next to the "Save Pattern" button in the submit bar. Clicking it opens a Bulma modal containing the rendered pattern text preview and stitch count — the same content currently in the sidebar, but presented on-demand.
16. The preview modal should have a close button and be dismissible via Esc or clicking the background overlay.

#### Unsaved Changes Protection

17. **Confirmation prompt on Save**: Before the form submits, show a confirmation dialog ("Save changes to this pattern?") to prevent accidental overwrites. This can be a simple `data-on-click` that triggers a Bulma modal with Confirm/Cancel buttons, where Confirm performs the actual form submission.
18. **Confirmation prompt on Cancel**: The "Cancel" link should prompt ("Discard unsaved changes?") before navigating away. If confirmed, navigate to `/patterns`.
19. **Confirmation prompt on navigation away**: If the user clicks a navbar link or any other link while on the editor page, prompt before leaving. Use a `beforeunload` event listener (added via Datastar or a small inline script) to catch browser-level navigation. For in-app Datastar-driven navigation, intercept with a confirmation modal.

**Affected files**:
- `internal/view/pattern_editor.templ` — primary UI changes
- `internal/handler/pattern.go` — form parsing updates (remove into_stitch writes, add difficulty, add part notes, handle add/remove part/entry SSE endpoints)
- `internal/domain/pattern.go` — add `Difficulty` field to `Pattern` struct; add `Notes` field to `InstructionGroup` struct; remove `Notes` field from `StitchEntry` struct
- `internal/repository/sqlite/pattern.go` — update INSERT/UPDATE/SELECT to include `difficulty`, `instruction_groups.notes`; remove `stitch_entries.notes` from queries
- `internal/repository/sqlite/migrations/` — new migration: add `difficulty` column to `patterns`, add `notes` column to `instruction_groups`, drop `notes` column from `stitch_entries`
- `internal/service/pattern.go` — update validation if needed
- `internal/view/layout.templ` or `static/style.css` — CSS for hiding number spinners

**Regression gate**: All existing tests pass. Pattern create/edit/duplicate flows continue to work. Existing patterns with `into_stitch` data are not corrupted (column remains in DB, just not surfaced in the editor). The `stitch_entries.notes` column is dropped — any existing data there is lost (acceptable since this field was rarely used and part-level notes replace it).
