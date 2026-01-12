# Whatomate Quick Start Guide

## Choose Your Setup Method

### 🪟 Windows Users (Docker - Easiest!)

**Best for:** Windows users who want the fastest setup with no manual configuration.

**Requirements:** Docker Desktop only

**Steps:**
1. Install Docker Desktop for Windows
2. Double-click `START_DOCKER_WINDOWS.bat`
3. Wait 5-10 minutes for build and startup
4. Access at http://localhost:5173

**Documentation:** [DOCKER_WINDOWS_SETUP.md](DOCKER_WINDOWS_SETUP.md)

---

### 🪟 Windows Users (Local Development)

**Best for:** Active development on Windows without Docker.

**Requirements:** Go 1.24+, Node.js 18+, PostgreSQL, Redis

**Steps:**
1. Install all prerequisites
2. Run database migrations: `go run cmd/whatomate/main.go server -config config.toml -migrate`
3. Start backend: `go run cmd/whatomate/main.go server -config config.toml`
4. Start frontend: `cd frontend && npm run dev`
5. Access at http://localhost:5173

**Documentation:** [WINDOWS_SETUP.md](WINDOWS_SETUP.md)

---

### 🐧 Linux/Mac Users (Local Development)

**Best for:** Active development on Linux or Mac.

**Requirements:** Go 1.24+, Node.js 18+, PostgreSQL, Redis

**Steps:**
```bash
# Clone repo
git clone https://github.com/shridarpatil/whatomate.git
cd whatomate

# Copy config
cp config.example.toml config.toml
# Edit config.toml with your settings

# Terminal 1: Backend
make run-migrate

# Terminal 2: Frontend
cd frontend
npm install
npm run dev
```

Access at http://localhost:5173 (dev) or http://localhost:8080 (if using production build)

**Documentation:** [EMBEDDED_SIGNUP_SETUP.md](EMBEDDED_SIGNUP_SETUP.md)

---

### 🐳 Docker (Production)

**Best for:** Production deployment on any platform.

**Requirements:** Docker & Docker Compose

**Steps:**
```bash
# Download files
curl -LO https://raw.githubusercontent.com/shridarpatil/whatomate/main/docker/docker-compose.yml
curl -LO https://raw.githubusercontent.com/shridarpatil/whatomate/main/config.example.toml

# Configure
cp config.example.toml config.toml
# Edit config.toml

# Start services
docker compose up -d
```

Access at http://localhost:8080

---

### 📦 Binary Release

**Best for:** Simple production deployment without Docker.

**Requirements:** PostgreSQL, Redis

**Steps:**
1. Download binary from [releases](https://github.com/shridarpatil/whatomate/releases)
2. Extract and configure:
   ```bash
   cp config.example.toml config.toml
   # Edit config.toml
   ```
3. Run with migrations:
   ```bash
   ./whatomate server -migrate
   ```

Access at http://localhost:8080

---

## After Installation

### Default Login
- **Email:** admin@admin.com
- **Password:** admin

**⚠️ Change this immediately in production!**

### Accessing Embedded Signup Feature

1. Login to Whatomate
2. Click **Settings** in the left sidebar
3. Click **Embedded Signup** in the submenu
4. Click **"+ Create Signup"** to configure your first signup

**Note:** Only **Admin** and **Manager** roles can access Embedded Signup.

---

## Setup Comparison

| Method | Setup Time | Complexity | Best For |
|--------|-----------|------------|----------|
| **Windows Docker** | 5-10 min | ⭐ Easy | Windows quick start |
| **Windows Local** | 20-30 min | ⭐⭐⭐ Medium | Windows development |
| **Linux/Mac Local** | 15-20 min | ⭐⭐ Easy | Linux/Mac development |
| **Docker Production** | 10-15 min | ⭐⭐ Easy | Production deployment |
| **Binary Release** | 5-10 min | ⭐⭐ Easy | Simple production |

---

## Troubleshooting

### "Embedded Signup" not showing in menu

**Solution 1:** Check your role
- Login as admin: `admin@admin.com` / `admin`
- Only Admin/Manager roles can see this feature

**Solution 2:** Clear browser cache
- Press Ctrl+Shift+Delete
- Or use Incognito mode

**Solution 3:** Verify database migrations ran
- Check backend logs for migration messages

### Port conflicts

**Check what's using a port:**
```bash
# Linux/Mac
lsof -i :8080

# Windows
netstat -ano | findstr :8080
```

**Kill the process or change ports in config.**

### Database connection errors

**Check if PostgreSQL is running:**
```bash
# Linux/Mac
pg_isready

# Windows
psql -U postgres -c "SELECT 1"
```

**Verify config.toml has correct credentials:**
- For Docker: `host = "db"`
- For local: `host = "localhost"`

### Redis connection errors

**Check if Redis is running:**
```bash
# Linux/Mac
redis-cli ping

# Windows
redis-cli ping
# Or if in Docker:
docker ps | findstr redis
```

**Verify config.toml has correct host:**
- For Docker: `host = "redis"`
- For local: `host = "localhost"`

---

## Documentation

- **Embedded Signup Feature:** [docs/embedded-signup.md](docs/embedded-signup.md)
- **Windows Docker Setup:** [DOCKER_WINDOWS_SETUP.md](DOCKER_WINDOWS_SETUP.md)
- **Windows Local Setup:** [WINDOWS_SETUP.md](WINDOWS_SETUP.md)
- **Linux/Mac Setup:** [EMBEDDED_SIGNUP_SETUP.md](EMBEDDED_SIGNUP_SETUP.md)
- **Configuration Reference:** [config.example.toml](config.example.toml)

---

## Support

- **GitHub Issues:** https://github.com/shridarpatil/whatomate/issues
- **Documentation:** https://github.com/shridarpatil/whatomate/tree/main/docs

---

## Next Steps

After successful installation:

1. ✅ **Change default password**
2. ✅ **Configure WhatsApp Business API credentials**
3. ✅ **Set up your first Embedded Signup**
4. ✅ **Test the signup flow**
5. ✅ **Integrate on your website**

See [docs/embedded-signup.md](docs/embedded-signup.md) for detailed feature documentation.
