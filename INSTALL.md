# WarmDesk — Installation Manual

## Requirements

| Component | Requirement |
|-----------|-------------|
| Go | 1.22 or later |
| Node.js | 20 or later |
| GCC | Required for SQLite (not needed for MySQL/PostgreSQL) |

---

## 1. Install Prerequisites

### Go

Download and install from https://go.dev/dl/

```bash
# Verify
go version
```

### Node.js

Download and install from https://nodejs.org/ (LTS recommended)

```bash
# Verify
node --version
npm --version
```

### GCC (for SQLite only)

- **Ubuntu / Debian**: `sudo apt install gcc`
- **RHEL / Fedora**: `sudo dnf install gcc`
- **macOS**: `xcode-select --install`
- **Windows**: Install [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) or use WSL

---

## 2. Build

```bash
git clone <repo-url>
cd warmdesk
make build
```

Output is placed in `dist/`:

```
dist/
  warmdesk               # server binary (Linux/macOS) or warmdesk.exe (Windows)
  warmdesk-seed          # demo data seeder
  warmdesk-export        # migration: WarmDesk → Jira / Trello / OpenProject / Ryver
  warmdesk-import        # migration: Jira / Trello / OpenProject / Ryver → WarmDesk
  web/                   # compiled frontend assets
  warmdesk.yaml.example  # annotated server config template
  warmdesk-migrate.yaml.example  # migration tool config template
  deploy/                # systemd / nginx / Apache templates
  docs/                  # user, API, and admin documentation
```

---

## 3. Configure

WarmDesk looks for a `warmdesk.yaml` file in its working directory.
Copy the example and edit it:

```bash
cp warmdesk.yaml.example dist/warmdesk.yaml
# Edit dist/warmdesk.yaml with your database, secret, and domain settings
```

You can also specify a config file path on the command line — useful when running
multiple instances or keeping configs outside the working directory:

```bash
./warmdesk --config /etc/warmdesk/production.yaml
```

Priority order (highest wins): CLI `--config` flag → `CONFIG_FILE` env var → `warmdesk.yaml` in working directory → built-in defaults.

Alternatively, use environment variables — they always override any config file.

---

## 4. Run

```bash
cd dist

# With config file (recommended)
WEB_DIR=./web ./warmdesk

# Or with environment variables only
PORT=8080 \
DB_DRIVER=sqlite \
DB_DSN=./warmdesk.db \
JWT_SECRET=your-secret-key \
ALLOWED_ORIGINS=https://yourdomain.com \
WEB_DIR=./web \
./warmdesk
```

Open the application at **http://localhost:8080** (or your configured port).

---

## 5. Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP port |
| `DB_DRIVER` | `sqlite` | `sqlite`, `mysql`, or `postgres` |
| `DB_DSN` | `./warmdesk.db` | Database connection string / file path |
| `JWT_SECRET` | `change-me-in-production` | Token signing secret — **always change this** |
| `ALLOWED_ORIGINS` | `http://localhost:5173` | CORS allowed origins (`*` for any) |
| `WEB_DIR` | *(empty)* | Path to built frontend files (required in production) |

---

## 6. Database Options

### SQLite (default — zero configuration)

```bash
DB_DRIVER=sqlite
DB_DSN=./warmdesk.db
```

### PostgreSQL

```bash
DB_DRIVER=postgres
DB_DSN="host=localhost user=warmdesk password=secret dbname=warmdesk port=5432 sslmode=disable"
```

### MySQL / MariaDB

```bash
DB_DRIVER=mysql
DB_DSN="warmdesk:secret@tcp(localhost:3306)/warmdesk?charset=utf8mb4&parseTime=True&loc=Local"
```

The schema is created automatically on first start via GORM's AutoMigrate.

---

## 7. Running as a System Service (Linux)

A ready-to-use service file is provided at `deploy/warmdesk.service`.

```bash
# Create a dedicated user
sudo useradd -r -s /bin/false warmdesk

# Copy files
sudo mkdir -p /opt/warmdesk/data
sudo cp -r dist/. /opt/warmdesk/
sudo chown -R warmdesk:warmdesk /opt/warmdesk

# Edit the service file to set your JWT_SECRET and domain, then install
sudo cp deploy/warmdesk.service /etc/systemd/system/warmdesk.service
sudo systemctl daemon-reload
sudo systemctl enable --now warmdesk
sudo systemctl status warmdesk
```

---

## 8. Reverse Proxy

A ready-to-use configuration for each web server is provided in the `deploy/` directory.
Both configurations handle HTTP→HTTPS redirect, SSL termination, and WebSocket proxying.

### Nginx (`deploy/nginx.conf`)

```bash
sudo cp deploy/nginx.conf /etc/nginx/sites-available/warmdesk
# Edit the file: replace yourdomain.com and update SSL paths
sudo ln -s /etc/nginx/sites-available/warmdesk /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

Obtain a free SSL certificate (if needed):
```bash
sudo certbot --nginx -d yourdomain.com
```

### Apache (`deploy/apache.conf`)

```bash
# Enable required modules
sudo a2enmod proxy proxy_http proxy_wstunnel ssl headers rewrite

sudo cp deploy/apache.conf /etc/apache2/sites-available/warmdesk.conf
# Edit the file: replace yourdomain.com and update SSL paths
sudo a2ensite warmdesk
sudo systemctl reload apache2
```

Obtain a free SSL certificate (if needed):
```bash
sudo certbot --apache -d yourdomain.com
```

Set `ALLOWED_ORIGINS=https://yourdomain.com` in the systemd service environment.

---

## 9. First Admin Account

The first registered user is a regular user. To grant admin rights:

**SQLite**
```bash
sqlite3 /opt/warmdesk/data/warmdesk.db \
  "UPDATE users SET global_role='admin' WHERE id=1;"
```

**PostgreSQL / MySQL**
```sql
UPDATE users SET global_role = 'admin' WHERE id = 1;
```

Once an admin account exists, further admin promotion can be done through
**Admin → Users → Edit** in the web interface.

---

## 10. Development Mode

Run backend and frontend separately with hot-reloading:

```bash
# Terminal 1 — backend API server on :8080
make dev-backend

# Terminal 2 — frontend dev server on :5173
make dev-frontend
```

Open **http://localhost:5173** during development.

---

## 11. Updating

```bash
git pull
make build
# restart the service
sudo systemctl restart warmdesk
```

---

## 12. Distribution Package

To create a portable archive for deployment on another machine:

```bash
make build
tar -czf warmdesk-$(date +%Y%m%d).tar.gz -C dist .
```

Extract on the target machine:

```bash
tar -xzf warmdesk-*.tar.gz -C /opt/warmdesk
```

Then follow steps 3–7 above.
