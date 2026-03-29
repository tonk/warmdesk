# Coworker

A self-hosted, multi-user project management tool with Kanban boards, real-time
collaboration, direct messaging, time tracking, and a ticket API.

## Experiment

This is an experiment, and a biggie :-)

I haven't written a single line of code, I only created and updated `what.md`
and asked Claude Code to generate the app.

## Screenshots

| | |
|---|---|
| ![Login](screenshots/01-login.png) | ![Dashboard](screenshots/02-dashboard.png) |
| *Login* | *Dashboard* |
| ![Kanban board](screenshots/03-board.png) | ![Card detail](screenshots/04-card-detail.png) |
| *Kanban board* | *Card detail with checklist, comments and time tracking* |
| ![Topics](screenshots/05-topics.png) | ![Direct messages](screenshots/06-messages.png) |
| *Threaded project discussions* | *Direct messages and group chat* |
| ![Time report](screenshots/07-report.png) | ![Admin panel](screenshots/08-admin-users.png) |
| *Time report with PDF/Excel export* | *Admin panel — user management* |
| ![Admin settings](screenshots/09-admin-settings.png) | ![User settings](screenshots/10-user-settings.png) |
| *Admin settings* | *User settings* |

## Features

- **Kanban boards** — columns, cards, drag-and-drop reorder, labels, priorities, due dates, assignees, watchers, markdown descriptions and comments
- **Card sorting** — sort column cards by date, assignee, or priority (ascending / descending)
- **Comment replies** — reply to any comment; replies are visually indented
- **Time tracking** — log hours and minutes spent directly on a card
- **Multi-project** — each project has its own board, members, and chat
- **Role-based access** — global roles (admin / user / viewer) and per-project roles (owner / member / viewer)
- **Real-time** — board changes, card moves, and chat messages sync instantly across all connected users via WebSocket
- **Internal chat** — per-project team chat and direct messages between users
- **Unread DM notifications** — pulsing indicator in the sidebar and header when there are unread direct messages
- **Sidebar** — starred projects, live online-users list, auto-refreshes when users are added or removed
- **Dark / light / system theme** — defaults to light
- **Multi-language** — English, Dutch (Nederlands), German (Deutsch), Spanish (Español), French (Français)
- **User settings** — display name, avatar, email, locale, theme, date/time format, timezone, font, password change
- **Admin panel** — manage all users (create, edit, assign projects, disable, delete) and all projects; toggle public registration on/off; configure global defaults (theme, locale, date format, timezone, font); configure SMTP email; set company name and logo
- **SMTP email** — configurable from the admin panel without a server restart; username and password are optional for relay servers
- **Session timeout** — configurable idle timeout (default 60 minutes); set to 0 to disable
- **Topics** — threaded discussions per project with markdown support and replies
- **Checklists** — add checklist items to cards with completion tracking
- **Multiple assignees** — assign more than one user to a card
- **Watchers** — subscribe to card activity
- **Favourite people** — mark users for quick access
- **Time reports** — generate a time overview filtered by period (all / year / month / week) and project; export to PDF (report only, no sidebar) or Excel (XLSX); time displayed as H:MM
- **Company branding** — set a company name and logo that appears on reports
- **Configurable initial columns** — admin can define which columns are created when a new project is made (defaults to "Backlog")
- **Ticket API** — create cards, add comments, and move cards via API key (for CI/CD pipelines and external integrations)
- **Database support** — SQLite (zero configuration), PostgreSQL, MySQL/MariaDB
- **Horizontal scaling** — Redis pub/sub for multi-instance WebSocket broadcast
- **Desktop app** — native Tauri app for Linux (AppImage), macOS (DMG), and Windows (installer)

## Quick Start

### Development

```bash
# Terminal 1 — backend (Go)
cd backend
go run .

# Terminal 2 — frontend (Vue 3 + Vite)
cd frontend
npm install
npm run dev
```

Open **http://localhost:5173** in your browser.

### Production build

```bash
make build
cd dist
WEB_DIR=./web ./coworker
```

Open **http://localhost:8080**.

### Load demo data

A seed tool is included in the distribution to populate the database with
realistic demo content (users, projects, cards, comments, and time entries):

```bash
cd dist
./coworker-seed           # seed demo data
./coworker-seed --reset   # wipe and re-seed
```

Demo accounts created (password for all: `demo1234`):

| Username | Display name | Role |
|---|---|---|
| `demo.admin` | Alex Admin | admin |
| `demo.sarah` | Sarah Chen | user |
| `demo.marc` | Marc Dubois | user |
| `demo.lisa` | Lisa Park | user |
| `demo.viewer` | Victor Viewer | viewer |

## Configuration

Copy the example config file and edit it:

```bash
cp coworker.yaml.example coworker.yaml
```

Settings can also be provided as environment variables, which always take precedence over the config file. Key options:

| Option | Env var | Default | Description |
|--------|---------|---------|-------------|
| `port` | `PORT` | `8080` | HTTP listen port |
| `db_driver` | `DB_DRIVER` | `sqlite` | `sqlite` / `postgres` / `mysql` |
| `db_dsn` | `DB_DSN` | `./coworker.db` | Database connection string |
| `jwt_secret` | `JWT_SECRET` | *(change this)* | Secret for signing JWT tokens |
| `allowed_origins` | `ALLOWED_ORIGINS` | `http://localhost:8080` | CORS allowed origins |
| `default_locale` | `DEFAULT_LOCALE` | `en` | Default UI language for new users |
| `gin_mode` | `GIN_MODE` | `debug` | `debug` or `release` |
| `api_log` | `API_LOG` | `true` | Log incoming HTTP requests |
| `db_log` | `DB_LOG` | `info` | DB query log level: `silent` / `error` / `warn` / `info` |
| `upload_dir` | `UPLOAD_DIR` | `./uploads` | Directory for uploaded files |
| `max_upload_mb` | `MAX_UPLOAD_MB` | `25` | Maximum upload file size in MB |

See [INSTALL.md](INSTALL.md) for full options and deployment instructions.

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.25, Gin, GORM, gorilla/websocket |
| Frontend | Vue 3, Vite, Pinia, vue-router, vue-i18n, EasyMDE, SheetJS |
| Database | SQLite / PostgreSQL / MySQL |
| Auth | JWT (access + refresh tokens), bcrypt |
| Desktop | Tauri 2 (Rust) |

## Ticket API

Automate ticket management from CI/CD pipelines or external tools using API keys.

Generate an API key under **Project Settings → API Keys**, then use it with any of the endpoints below.

```
POST  /api/v1/ticket/{slug}/cards                    — create a card
POST  /api/v1/ticket/{slug}/cards/{id}/comments      — add a comment
PATCH /api/v1/ticket/{slug}/cards/{id}/move          — move to a column
```

Pass the key in the `X-API-Key` header or as `?api_key=` query parameter.

## Installation

See [INSTALL.md](INSTALL.md) for full instructions including:
- Building from source
- Running as a systemd service
- Nginx and Apache reverse proxy configuration
- PostgreSQL / MySQL setup
- First admin account setup
