# Coworker — Installation Manual

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
cd coworker
make build
```

Output is placed in `dist/`:

```
dist/
  coworker        # server binary (Linux/macOS) or coworker.exe (Windows)
  web/            # compiled frontend assets
```

---

## 3. Configure

Coworker looks for a `coworker.yaml` file in its working directory.
Copy the example and edit it:

```bash
cp coworker.yaml.example dist/coworker.yaml
# Edit dist/coworker.yaml with your database, secret, and domain settings
```

You can also specify a config file path on the command line — useful when running
multiple instances or keeping configs outside the working directory:

```bash
./coworker --config /etc/coworker/production.yaml
```

Priority order (highest wins): CLI `--config` flag → `CONFIG_FILE` env var → `coworker.yaml` in working directory → built-in defaults.

Alternatively, use environment variables — they always override any config file.

---

## 4. Run

```bash
cd dist

# With config file (recommended)
WEB_DIR=./web ./coworker

# Or with environment variables only
PORT=8080 \
DB_DRIVER=sqlite \
DB_DSN=./coworker.db \
JWT_SECRET=your-secret-key \
ALLOWED_ORIGINS=https://yourdomain.com \
WEB_DIR=./web \
./coworker
```

Open the application at **http://localhost:8080** (or your configured port).

---

## 5. Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP port |
| `DB_DRIVER` | `sqlite` | `sqlite`, `mysql`, or `postgres` |
| `DB_DSN` | `./coworker.db` | Database connection string / file path |
| `JWT_SECRET` | `change-me-in-production` | Token signing secret — **always change this** |
| `ALLOWED_ORIGINS` | `http://localhost:5173` | CORS allowed origins (`*` for any) |
| `WEB_DIR` | *(empty)* | Path to built frontend files (required in production) |

---

## 6. Database Options

### SQLite (default — zero configuration)

```bash
DB_DRIVER=sqlite
DB_DSN=./coworker.db
```

### PostgreSQL

```bash
DB_DRIVER=postgres
DB_DSN="host=localhost user=coworker password=secret dbname=coworker port=5432 sslmode=disable"
```

### MySQL / MariaDB

```bash
DB_DRIVER=mysql
DB_DSN="coworker:secret@tcp(localhost:3306)/coworker?charset=utf8mb4&parseTime=True&loc=Local"
```

The schema is created automatically on first start via GORM's AutoMigrate.

---

## 7. Running as a System Service (Linux)

A ready-to-use service file is provided at `deploy/coworker.service`.

```bash
# Create a dedicated user
sudo useradd -r -s /bin/false coworker

# Copy files
sudo mkdir -p /opt/coworker/data
sudo cp -r dist/. /opt/coworker/
sudo chown -R coworker:coworker /opt/coworker

# Edit the service file to set your JWT_SECRET and domain, then install
sudo cp deploy/coworker.service /etc/systemd/system/coworker.service
sudo systemctl daemon-reload
sudo systemctl enable --now coworker
sudo systemctl status coworker
```

---

## 8. Reverse Proxy

A ready-to-use configuration for each web server is provided in the `deploy/` directory.
Both configurations handle HTTP→HTTPS redirect, SSL termination, and WebSocket proxying.

### Nginx (`deploy/nginx.conf`)

```bash
sudo cp deploy/nginx.conf /etc/nginx/sites-available/coworker
# Edit the file: replace yourdomain.com and update SSL paths
sudo ln -s /etc/nginx/sites-available/coworker /etc/nginx/sites-enabled/
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

sudo cp deploy/apache.conf /etc/apache2/sites-available/coworker.conf
# Edit the file: replace yourdomain.com and update SSL paths
sudo a2ensite coworker
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
sqlite3 /opt/coworker/data/coworker.db \
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
sudo systemctl restart coworker
```

---

## 12. Distribution Package

To create a portable archive for deployment on another machine:

```bash
make build
tar -czf coworker-$(date +%Y%m%d).tar.gz -C dist .
```

Extract on the target machine:

```bash
tar -xzf coworker-*.tar.gz -C /opt/coworker
```

Then follow steps 3–7 above.
