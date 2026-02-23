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

---

### IMP-8: UI Consistency — Spacing, Button Grouping & Datastar Adoption

**Problem**: The UI has accumulated inconsistent button spacing, inline `style` attributes that duplicate Bulma utilities, forms with `style="display:inline"` that break flex/button-group spacing, and vanilla JavaScript for interactions that Datastar handles more cleanly. This improvement standardises spacing across all pages using Bulma's built-in layout components and migrates remaining vanilla JS interaction patterns to Datastar where it simplifies the code or improves maintainability.

**Scope**: This is a purely front-end change — no database migrations, no domain/service changes, no new HTTP endpoints. All changes are in `.templ` files and the layout `<style>` block.

---

#### Part A: Spacing & Button Grouping

These changes replace ad-hoc inline styles with idiomatic Bulma layout patterns. The guiding rules:

- **Buttons side-by-side**: wrap in `<div class="buttons">`. Bulma handles the gap automatically.
- **Form submit/cancel actions**: use `<div class="field is-grouped">` with `<div class="control">` wrappers.
- **Left/right distribution** (title + action buttons): use `<div class="level">` with `level-left` / `level-right`.
- **Vertical spacing between sections**: use Bulma spacing helpers (`mb-4`, `mt-5`, etc.) instead of inline `style="gap: …"` or `style="margin-top: …"`.
- **Never use `style="display:inline"`** on `<form>` elements — it collapses flex spacing. Instead, either (a) remove the form wrapper entirely and use a Datastar `data-on:click="@post(…)"` on the button directly (preferred, see Part B), or (b) leave the form unstyled and let it participate normally in flex layout.

##### A1. Dashboard — Quick Links buttons (`dashboard.templ`)

**Before**: `<div class="buttons are-medium" style="flex-direction: column; align-items: flex-start;">`

**After**: Replace with a simple vertical stack using individual `mb-2` spacing on `is-fullwidth` buttons (no `buttons` wrapper needed, since these are stacked, not side-by-side):

```html
<div>
    <a class="button is-primary is-fullwidth is-medium mb-2" href="/patterns">My Patterns</a>
    <a class="button is-link is-fullwidth is-medium mb-2" href="/patterns/new">New Pattern</a>
    <a class="button is-info is-fullwidth is-medium" href="/stitches">Stitch Library</a>
</div>
```

##### A2. Pattern View — Action buttons (`pattern_view.templ`)

**Before**: `<form style="display:inline;">` inside `.buttons` — the inline form breaks Bulma's flexbox gap.

**After**: Remove the form wrapper for the Duplicate button; use a Datastar `data-on:click="@post(…)"` directly on the button (see B3). If keeping forms, remove `style="display:inline;"` and let the form participate in flex layout normally.

##### A3. Work Session — Control buttons (`worksession.templ`, lines 31–49)

**Before**: Multiple `<form style="display:inline;">` elements inside a `.buttons` container.

**After**: Remove form wrappers; use Datastar `data-on:click="@post(…)"` on the buttons directly (see B2). This eliminates the inline-form spacing issue entirely.

##### A4. Work Session — Navigation buttons (`worksession.templ`, lines 154–165)

**Before**: `<div class="is-flex is-justify-content-center" style="gap: 1rem;">` with `<form style="display:inline;">` wrappers.

**After**: Use a `<div class="buttons is-centered">` container. Remove form wrappers; use Datastar `data-on:click="@post(…)"` on the buttons directly (see B2). Bulma's `.buttons` provides consistent gap without inline styles.

##### A5. Work Session — Parts overview strip (`worksession.templ`, line 54)

**Before**: `<div class="tags are-medium" style="flex-wrap: wrap; gap: 0.25rem;">`

**After**: Bulma's `.tags` already wraps and provides spacing. Remove the inline `style` entirely — the default `.tags` behaviour is correct.

##### A6. Work Session — Current stitch display (`worksession.templ`, line 94)

**Before**: `style="gap: 2rem;"` and `style="min-width: 80px;"` on child divs.

**After**: Add a small CSS utility class in the layout `<style>` block (e.g., `.stitch-display-row { gap: 2rem; }` and `.stitch-context { min-width: 80px; }`). Reference these classes instead of inline styles. This keeps presentation in CSS rather than scattered across templates.

##### A7. Work Session — Current stitch highlight border (`worksession.templ`, line 65)

**Before**: `style="border: 2px solid hsl(171, 100%, 41%); font-weight: bold;"`

**After**: Define a `.tag-current` CSS class in the layout `<style>` block and reference it. Bulma doesn't have a built-in modifier for this, so a tiny custom class is appropriate.

##### A8. Stitch Library — "Add" button column (`stitch_library.templ`, lines 132–139)

**Before**: The "Add" button sits in a `column is-1` with an `&nbsp;` label to align it with sibling form fields. The narrow column makes the button cramped, especially on mobile.

**After**: Move the submit button out of the `columns` row entirely and place it below the form inputs in a `<div class="field">` wrapper. This avoids the cramped single-column alignment issue and gives the button room to breathe.

##### A9. Stitch Library — Delete button form (`stitch_library.templ`, line 165)

**Before**: `<form style="display:inline;">` in a table cell.

**After**: Remove the form wrapper; use Datastar `data-on:click` to handle the delete flow (see B4).

##### A10. Pattern List — Card footer forms (`pattern_list.templ`, lines 60–79)

**Before**: `<form style="display:contents;">` wrappers with buttons styled via `style="border:none;background:none;cursor:pointer;"`.

**After**: Keep the card-footer structure (it works well for card actions), but replace the inline `style` attributes with a small CSS class (`.card-footer-button`) defined in the layout `<style>` block:

```css
.card-footer-button {
    border: none;
    background: none;
    cursor: pointer;
}
```

##### A11. Pattern Editor — Part box remove button (`pattern_editor.templ`, lines 213–222)

**Before**: Remove button positioned with `style="position: absolute; top: 0.75rem; right: 0.75rem;"`.

**After**: Define a `.box-close-btn` CSS class in the layout `<style>` block for this positioning pattern. This keeps the template markup clean and ensures consistent positioning if the same pattern is used elsewhere.

##### A12. Pattern View — Pattern text preview (`pattern_view.templ`, line 49)

**Before**: `style="white-space: pre-wrap; background: #f5f5f5; padding: 1rem; border-radius: 4px; font-family: monospace;"`

**After**: Define a `.pattern-text` CSS class in the layout `<style>` block (the class is already referenced but has no definition — only inline styles). Similarly update the same inline style in `pattern_editor.templ` line 189.

```css
.pattern-text {
    white-space: pre-wrap;
    background: #f5f5f5;
    padding: 1rem;
    border-radius: 4px;
    font-family: monospace;
    font-size: 0.85rem;
}
```

---

#### Part B: Datastar Adoption

These changes convert vanilla JavaScript interactions to Datastar declarative patterns. The goal is not to eliminate all JS, but to use Datastar where it results in less code, better consistency with the existing SSE-driven architecture, and fewer `<script>` blocks scattered across templates.

**Guiding rule**: If a button's only job is to POST to an endpoint and the response is a page redirect or SSE patch, use `data-on:click="@post('/…')"` instead of wrapping it in a `<form>`. If a button toggles UI state (modal visibility, navbar menu), use Datastar signals instead of inline `onclick` with `classList.toggle`.

##### B1. Navbar burger toggle (`layout.templ`, lines 37–48)

**Before**: Inline `onclick` that toggles `is-active` on the navbar menu via `document.getElementById` and updates `aria-expanded`.

**After**: Use a Datastar signal for menu state:

```html
<div data-signals="{navOpen: false}">
    <a role="button" class="navbar-burger"
       data-class-is-active="$navOpen"
       aria-expanded="false"
       data-attr-aria-expanded="$navOpen"
       data-on:click="$navOpen = !$navOpen">
        …
    </a>
</div>
```

And on the `#navbarMenu` div: `data-class-is-active="$navOpen"`.

##### B2. Work Session — Pause/Resume/Abandon/Navigation buttons (`worksession.templ`)

**Before**: Each button is wrapped in a `<form method="POST" style="display:inline;">` with `type="submit"`.

**After**: Remove form wrappers. Use `data-on:click="@post('/sessions/{id}/pause')"` (and similarly for resume, abandon, prev, next) directly on the button elements. For abandon, the confirmation modal is still shown first (see B5).

This eliminates:
- All `style="display:inline;"` on forms
- The need for form IDs (`prev-form`, `next-form`, `abandon-form`)
- The vanilla JS keyboard handler that submits forms by ID (replaced in B6)

##### B3. Pattern View — Duplicate button (`pattern_view.templ`, line 33)

**Before**: `<form method="POST" style="display:inline;"><button type="submit">Duplicate</button></form>`

**After**: `<button class="button is-info" data-on:click="@post('/patterns/{id}/duplicate')">Duplicate</button>`

##### B4. Stitch Library — Delete custom stitch (`stitch_library.templ`, lines 165–170)

**Before**: `<form style="display:inline;">` with a `<button type="button" onclick={deleteStitchOnclick(…)}>` that calls `showConfirmModal` → `form.submit()`.

**After**: Remove the form wrapper. Use a Datastar-driven confirmation flow (see B5) that posts directly via `@post('/stitches/{id}/delete')`.

##### B5. Confirmation modal — Datastar signal-driven (`layout.templ`, lines 86–119)

**Before**: The shared confirm modal uses three global JS functions (`showConfirmModal`, `executeConfirmAction`, `closeConfirmModal`) and a global `__confirmCallback` variable. Callers invoke it with inline `onclick` handlers and pass a callback that typically calls `form.submit()`.

**After**: Convert to a Datastar signal-driven modal. Define signals on the `<body>` or a wrapper element:

```html
<div data-signals="{confirmOpen: false, confirmTitle: '', confirmMsg: '', confirmUrl: ''}">
```

The modal uses `data-class-is-active="$confirmOpen"`. The "Confirm" button uses `data-on:click="@post($confirmUrl); $confirmOpen = false"`. Callers open the modal by setting the signals:

```html
data-on:click="$confirmTitle = 'Delete Pattern'; $confirmMsg = 'Delete this pattern?'; $confirmUrl = '/patterns/5/delete'; $confirmOpen = true"
```

This eliminates:
- The global `__confirmCallback` variable
- The three global JS functions
- All `<script>` blocks for `deletePatternOnclick`, `deleteStitchOnclick`, and the work session abandon handler
- Form wrappers that existed solely to be `.submit()`ed from the callback

**Note**: The pattern editor's `removePartOnclick` and `removeEntryOnclick` perform client-side DOM removal (not a server POST), so they do **not** fit this pattern. Keep those as-is — they are legitimate client-side operations that remove DOM nodes from the form before submission.

##### B6. Work Session — Keyboard navigation (`worksession.templ`, lines 170–205)

**Before**: A `<script>` block with `document.addEventListener('keydown', …)` that submits forms by ID, and a touch/swipe handler.

**After**: Use Datastar's `data-on:keydown.window` on the stitch display container:

```html
<div id="stitch-display"
     data-on:keydown.window="
         if (evt.target.tagName === 'INPUT' || evt.target.tagName === 'TEXTAREA') return;
         if (evt.key === ' ' || evt.key === 'ArrowRight') { evt.preventDefault(); @post('/sessions/{id}/next'); }
         else if (evt.key === 'Backspace' || evt.key === 'ArrowLeft') { evt.preventDefault(); @post('/sessions/{id}/prev'); }
         else if (evt.key === 'p' || evt.key === 'P') { @post('/sessions/{id}/pause'); }
         else if (evt.key === 'Escape') { $confirmTitle='Abandon Session'; $confirmMsg='Abandon this session? Your progress will be lost.'; $confirmUrl='/sessions/{id}/abandon'; $confirmOpen=true; }
     ">
```

The touch/swipe handler can remain as a small `<script>` block — Datastar doesn't have a built-in swipe gesture directive, so vanilla JS is appropriate here. However, instead of calling `form.submit()`, the swipe handler should use `fetch('/sessions/{id}/next', {method:'POST'}).then(() => window.location.reload())` or simply navigate via `window.location.href`. Alternatively, the swipe handler can click the Datastar-enabled button to trigger the `@post`.

##### B7. Pattern Editor — Modal triggers (`pattern_editor.templ`, lines 138, 141, 144)

**Before**: `onclick="document.getElementById('save-modal').classList.add('is-active')"` and similar for preview and cancel modals.

**After**: Use Datastar signals:

```html
<div data-signals="{showSave: false, showPreview: false, showCancel: false}">
```

Each button: `data-on:click="$showSave = true"` (etc.)
Each modal: `data-class-is-active="$showSave"` on the `.modal` div.
Each close button/background: `data-on:click="$showSave = false"`.

This eliminates all `onclick` handlers in the modal section and makes modal state inspectable via Datastar devtools.

##### B8. Pattern List — Delete pattern confirmation (`pattern_list.templ`, lines 73–79, 91–95)

**Before**: `<form style="display:contents;">` with `onclick={deletePatternOnclick(…)}` that calls `showConfirmModal` → `form.submit()`.

**After**: Remove the form wrapper. Use the Datastar signal-driven confirm modal from B5:

```html
<button class="card-footer-item has-text-danger card-footer-button" type="button"
    data-on:click="$confirmTitle='Delete Pattern'; $confirmMsg='Delete \"{name}\"? This cannot be undone.'; $confirmUrl='/patterns/{id}/delete'; $confirmOpen=true">
    Delete
</button>
```

Remove the `deletePatternOnclick` templ `script` block entirely.

##### B9. Pattern List — Start Session & Duplicate forms (`pattern_list.templ`, lines 60–72)

**Before**: `<form method="POST" style="display:contents;">` wrappers for Start and Duplicate actions in card footers.

**After**: Remove form wrappers. Use `data-on:click="@post('/patterns/{id}/start-session')"` and `data-on:click="@post('/patterns/{id}/duplicate')"` on the button elements directly.

---

#### What NOT to change

- **Auth forms** (login/register) — these are traditional full-page form submissions that redirect on success. Datastar SSE would add complexity for no UX benefit here; the forms work correctly as-is.
- **Stitch Library search/filter form** — this is a `GET` form that filters results. It works well as a standard form submission with page reload. No benefit to Datastar here.
- **Pattern Editor save form** — the main form submission (`pattern-form`) uses traditional POST. The `beforeunload` protection and the save confirmation modal interact with this form. Converting to Datastar SSE would require rethinking the entire form submission flow, which is out of scope.
- **Pattern Editor `removePartOnclick` / `removeEntryOnclick`** — these perform client-side DOM removal, not server round-trips. They are appropriate as JS and would not benefit from Datastar `@post`.

---

#### Affected files

- `internal/view/layout.templ` — navbar burger (B1), confirm modal rewrite (B5), new CSS utility classes (A6, A7, A10, A11, A12), remove global JS functions
- `internal/view/dashboard.templ` — Quick Links button stack (A1)
- `internal/view/pattern_list.templ` — card footer forms → Datastar (A10, B8, B9), remove `deletePatternOnclick` script
- `internal/view/pattern_view.templ` — duplicate form → Datastar (A2, B3), pattern-text inline style → CSS class (A12)
- `internal/view/pattern_editor.templ` — modal triggers → Datastar signals (B7), pattern-text inline style → CSS class (A12)
- `internal/view/stitch_library.templ` — Add button repositioning (A8), delete form → Datastar (A9, B4), remove `deleteStitchOnclick` script
- `internal/view/worksession.templ` — all spacing fixes (A3–A7), button forms → Datastar (B2), keyboard navigation → Datastar (B6)

**No changes to**: `domain/`, `service/`, `repository/`, `handler/`, migrations, or tests. All handlers already accept POST requests and respond with redirects — the Datastar `@post` calls will trigger the same server-side code paths.

**Regression gate**: All existing tests pass. All pages render correctly. Pattern CRUD, work session navigation, stitch library management, delete confirmations, and keyboard shortcuts continue to function. No visual regressions — spacing should improve, not change semantics.

---

### IMP-10: Pattern Part Image Uploads

**Problem**: Users have no way to attach reference images to pattern parts. Photos of completed sections, stitch close-ups, or diagram sketches are commonly needed while following a pattern. Currently, users must keep these in a separate app or browser tab.

**Goal**: Allow users to upload and view images for each part (instruction group) of a pattern. Up to 5 images per part, max 10MB per file, JPEG and PNG only. All file storage and retrieval is behind a swappable interface so the initial SQLite BLOB implementation can later be replaced with filesystem, S3, or another backend.

---

#### Domain Layer

**New file: `internal/domain/image.go`**

Two new interfaces and one new entity:

```go
// PatternImage holds metadata about an image attached to an instruction group.
type PatternImage struct {
    ID                 int64
    InstructionGroupID int64
    Filename           string    // Original upload filename
    ContentType        string    // "image/jpeg" or "image/png"
    Size               int64     // File size in bytes
    StorageKey         string    // Key used to retrieve bytes from FileStore
    SortOrder          int       // Display order within the group
    CreatedAt          time.Time
}

// PatternImageRepository handles image metadata persistence.
type PatternImageRepository interface {
    Create(ctx context.Context, image *PatternImage) error
    GetByID(ctx context.Context, id int64) (*PatternImage, error)
    ListByGroup(ctx context.Context, groupID int64) ([]PatternImage, error)
    Delete(ctx context.Context, id int64) error
    CountByGroup(ctx context.Context, groupID int64) (int, error)
}

// FileStore abstracts raw file byte storage.
// The initial implementation stores BLOBs in SQLite; this interface
// allows swapping to filesystem, S3, or another backend later.
type FileStore interface {
    Save(ctx context.Context, key string, data []byte) error
    Get(ctx context.Context, key string) ([]byte, error)
    Delete(ctx context.Context, key string) error
}
```

The `PatternImageRepository` follows the existing repository pattern (metadata in the relational DB). The `FileStore` is a separate abstraction for raw bytes — the key design point that makes the storage backend independently swappable.

---

#### Database — Migration 006

**New file: `internal/repository/sqlite/migrations/006_create_pattern_images.sql`**

```sql
-- 006_create_pattern_images.sql
-- Image metadata and blob storage for pattern part images.

CREATE TABLE IF NOT EXISTS pattern_images (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instruction_group_id INTEGER NOT NULL REFERENCES instruction_groups(id) ON DELETE CASCADE,
    filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size INTEGER NOT NULL,
    storage_key TEXT NOT NULL,
    sort_order INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pattern_images_group ON pattern_images(instruction_group_id);

CREATE TABLE IF NOT EXISTS file_blobs (
    storage_key TEXT PRIMARY KEY,
    data BLOB NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

The `file_blobs` table is owned by the SQLite `FileStore` implementation. A different backend (e.g., S3) would not use this table — it would store bytes externally and only implement the `FileStore` interface.

`ON DELETE CASCADE` on `instruction_group_id` ensures that deleting a part (or its parent pattern) removes image metadata rows. The corresponding `FileStore` cleanup (deleting the actual bytes) must be handled by the service layer before or after the cascade, since the `FileStore` is a separate abstraction.

---

#### Repository Layer

**New file: `internal/repository/sqlite/pattern_image.go`**

Implements `domain.PatternImageRepository` — standard CRUD against the `pattern_images` table. Follows the same patterns as existing repository files (constructor injection of `*sql.DB`, context-aware queries).

**New file: `internal/repository/sqlite/filestore.go`**

Implements `domain.FileStore` using the `file_blobs` table:

- `Save`: `INSERT INTO file_blobs (storage_key, data) VALUES (?, ?)`
- `Get`: `SELECT data FROM file_blobs WHERE storage_key = ?`
- `Delete`: `DELETE FROM file_blobs WHERE storage_key = ?`

---

#### Service Layer

**New file: `internal/service/image.go`**

`ImageService` orchestrates image uploads, retrieval, and deletion:

- **Dependencies**: `domain.PatternImageRepository`, `domain.FileStore`, `domain.PatternRepository` (for ownership verification)
- **`Upload(ctx, userID, groupID, filename, contentType string, data []byte) (*PatternImage, error)`**:
  1. Verify the instruction group exists and belongs to a pattern owned by `userID`
  2. Validate content type is `image/jpeg` or `image/png`
  3. Validate file size ≤ 10MB (10 × 1024 × 1024 bytes)
  4. Check `CountByGroup` < 5
  5. Generate a storage key (e.g., `"pattern-images/{uuid}"` using `crypto/rand`)
  6. Call `FileStore.Save` with the key and data
  7. Create `PatternImage` metadata via `PatternImageRepository.Create`
  8. Return the created image metadata
- **`GetFile(ctx, userID, imageID) ([]byte, string, error)`** — returns bytes + content type after ownership check
- **`Delete(ctx, userID, imageID) error`** — deletes from `FileStore` then `PatternImageRepository` after ownership check
- **`ListByGroup(ctx, groupID) ([]PatternImage, error)`** — passthrough to repository

---

#### Handler Layer

**New file: `internal/handler/image.go`**

- **`POST /patterns/{id}/parts/{groupIndex}/images`** — accepts `multipart/form-data` with a file field named `image`. Parses with `r.ParseMultipartForm(10 << 20)` (10MB limit). Calls `ImageService.Upload`. Responds with SSE patch to update the image section of the part.
- **`GET /images/{id}`** — serves image bytes with correct `Content-Type` header and `Cache-Control`. No SSE — this is a direct HTTP response for `<img src="...">` tags.
- **`DELETE /images/{id}`** — calls `ImageService.Delete`. Responds with SSE patch to remove the image thumbnail from the UI.

Register routes in `internal/handler/routes.go`.

---

#### View Layer

**`internal/view/pattern_editor.templ`** — within each part box, below the stitch entries and part notes:

- An "Images" subsection showing thumbnails of attached images (if any) in a flex row
- Each thumbnail has a delete button (×)
- An "Upload Image" button (visible only if < 5 images) that opens a file input
- Display count indicator: "2 / 5 images"

**`internal/view/pattern_view.templ`** — within each part section:

- Display attached images as a thumbnail gallery (clickable to view full size in a Bulma modal)

---

#### Constraints

| Constraint | Value |
|---|---|
| Max images per part | 5 |
| Max file size | 10MB (10,485,760 bytes) |
| Accepted content types | `image/jpeg`, `image/png` |

---

#### Affected files

- **New**: `internal/domain/image.go`, `internal/repository/sqlite/pattern_image.go`, `internal/repository/sqlite/filestore.go`, `internal/service/image.go`, `internal/handler/image.go`, `internal/repository/sqlite/migrations/006_create_pattern_images.sql`
- **Modified**: `internal/handler/routes.go` (register new routes), `internal/view/pattern_editor.templ` (image section in parts), `internal/view/pattern_view.templ` (image gallery in parts), `main.go` (wire new service and repositories)

#### Regression gate

All existing tests pass. New integration tests cover: upload image (success, wrong type → error, too large → error, 6th image → error), retrieve image, delete image, cascade delete when part/pattern is deleted. Pattern CRUD, work session, and stitch library flows unaffected.
