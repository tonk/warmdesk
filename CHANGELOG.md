# Changelog

All notable changes to Coworker are documented here.

## v0.2.3 — 2026-03-29

### Added
- **Assignee filter on time reports** — the report page now has a multi-select dropdown to filter by one, several, or all assignees; selected names are shown as a summary label; passed to the backend as a comma-separated `assignees` query param
- **Direct message history** — opening a conversation (including via a sidebar user link) now immediately loads all stored messages from the database; history persists across sessions
- **Remove member from group chat** — any group member can remove another member via the × chip next to their name in the chat header; removal is confirmed and broadcast to remaining members via WebSocket
- **Demo conversations in seed** — `coworker-seed` now creates 5 conversations with 42 realistic messages (4 one-on-one DMs: Alex↔Sarah, Marc↔Lisa, Sarah↔Lisa, Alex↔Marc; plus a "Website Redesign Team" group chat) with historically-spread timestamps
- **Screenshots in README** — a 2-column screenshot grid has been added to the README covering all main views

### Fixed
- **DM sidebar navigation race condition** — clicking a user in the sidebar while conversations were still loading could create a new blank conversation instead of opening the existing one; the watch handler now waits for both conversations and users to be loaded before calling `openOrCreateDM`

## v0.2.2 — 2026-03-29

### Added
- **Configurable initial columns** — admin can define which columns are created automatically when a new project is made (Admin → Settings → New Project Defaults); one column name per line; defaults to "Backlog"
- **Delete empty column** — a trash icon appears on any column that has no cards; clicking it asks for confirmation and removes the column

### Fixed
- **Version number on login page** — app version is now shown below the login card, matching the footer
- **Frontend version follows git tag** — `__APP_VERSION__` is now derived from `git describe --tags --always` at build time instead of the static `package.json` version; the update-available banner no longer appears falsely after a release
- **Admin sidebar shows all projects** — admins now see all projects in the sidebar, not only the ones they were explicitly added to as a member
- **PDF report shows only the report** — the browser print dialog now hides the sidebar, header, and footer so only the time report content is printed
- **Time format in reports** — changed from "1h 30m" to `H:MM` (e.g. `1:30`, `100:05`); hours are unbounded, minutes are always zero-padded to two digits

### Changed
- Default initial column renamed from "Inbox" to "Backlog"

## v0.2.0 — 2026-03-29

### Added
- **Time Spent on cards** — log hours and minutes directly on a card; stored as `time_spent_minutes` and shown in the card detail dialog
- **Time Report** — new `/reports` page that generates a time overview grouped by project, filterable by period (all time / year / month / ISO week) and by project
- **Export to PDF** — print-optimised layout with company logo and period header; uses the browser's native print-to-PDF
- **Export to Excel (XLSX)** — downloads a formatted spreadsheet via SheetJS; includes ref, title, assignees, date, and time columns with subtotals per project and a grand total
- **Company branding** — admin can set a company name and logo (URL or uploaded image) under Admin → Settings → Branding; both appear on generated reports
- **Demo seed tool** — `coworker-seed` binary (included in the distribution) populates the database with four demo users, three projects, 32 cards with labels/assignees/checklists/comments/time, and three discussion topics; run with `--reset` to wipe and re-seed; idempotent on repeated runs
- **CLAUDE.md** — developer guide for AI-assisted development: architecture decisions, conventions, and how to add routes, models, and settings
- **Configurable idle session timeout** — admin setting (default 60 minutes); users are automatically logged out after the configured period of inactivity; set to 0 to disable
- **Update check** — on login the server is compared against the latest GitHub release; a dismissable banner is shown when a newer version is available (web and desktop)

### Fixed
- **SMTP settings could not be saved** — the save button shared a function with all auto-saving dropdowns (theme, timezone, etc.), causing SMTP fields to be sent in every general-settings request and potentially overwriting saved values; SMTP now has its own dedicated save
- **SMTP username and password made optional** — all SMTP credential fields are now pointer types in the backend; omitting them from a request leaves the stored value untouched, allowing auth-less SMTP relay configurations

### Changed
- `coworker-seed` is built alongside the main binary by `make build-backend` and included in distribution archives
- System settings handler splits SMTP saves from general settings saves to prevent cross-contamination

## 2026-03-28

### Added
- Tauri desktop app — distributable as AppImage (Linux), DMG (macOS), and installer (Windows)
- Topics — threaded discussions per project with markdown support and replies
- Checklists on cards
- Multiple assignees per card
- Viewer role — read-only access at project and global level
- Favourite people — mark users for quick access
- Card watchers — subscribe to card activity notifications
- Card sorting within columns by due date, assignee, or priority
- Direct message notifications
- Group direct messages
- Admin can assign users to projects directly
- Admin can reset user passwords

### Fixed
- Topics view was rendering its own header, causing duplicate search bar, language selector, and avatar
- Adding a new card showed it twice until page refresh (duplicate WebSocket event handling)
- Logo and favicon not served correctly
- Build artifacts (AppImage, DMG, Windows installer, Rust target/) excluded from git via .gitignore

### Changed
- Group DMs, markdown in chat, i18n expansion, and UI polish
