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
