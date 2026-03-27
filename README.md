# Coworker

A self-hosted, multi-user Kanban board application with real-time
collaboration, direct messaging, and a ticket API.

## Experiment

This is an experiment, and a biggie :-)

I haven't written a single line of code, I only created, and changed the
`what.md` and asked Claude Code to generate the APP.

## Features

- **Kanban boards** — columns, cards, drag-and-drop reorder, labels, priorities, due dates, assignees, markdown descriptions and comments
- **Multi-project** — each project has its own board, members, and chat
- **Role-based access** — global roles (admin / user) and per-project roles (owner / member / viewer)
- **Real-time** — board changes, card moves, and chat messages sync instantly across all connected users via WebSocket
- **Internal chat** — per-project team chat and direct messages between users
- **Sidebar** — starred projects and live online-users list
- **Dark / light / system theme**
- **Multi-language** — English and Dutch (Nederlands)
- **User settings** — display name, avatar, email, locale, theme, date/time format, timezone, password change
- **Admin panel** — manage all users (create, edit, disable, delete) and all projects; toggle public registration on/off
- **Ticket API** — create cards, add comments, and move cards via API key (for CI/CD pipelines and external integrations)
- **Database support** — SQLite (zero configuration), PostgreSQL, MySQL/MariaDB

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

## Configuration

Copy the example config file and edit it:

```bash
cp coworker.yaml.example coworker.yaml
```

Settings can also be provided as environment variables, which always take precedence over the config file. See [INSTALL.md](INSTALL.md) for all options.

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.22, Gin, GORM, gorilla/websocket |
| Frontend | Vue 3, Vite, Pinia, vue-router, vue-i18n |
| Database | SQLite / PostgreSQL / MySQL |
| Auth | JWT (access + refresh tokens), bcrypt |

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
