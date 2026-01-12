# Whatomate Docker Setup for Windows

**The easiest way to run Whatomate on Windows with the Embedded Signup feature!**

Everything runs in Docker containers - no need to install Go, Node.js, PostgreSQL, or Redis locally.

---

## Prerequisites

### Required:
- ✅ **Docker Desktop for Windows** - Download from https://www.docker.com/products/docker-desktop/
  - Make sure Docker Desktop is running before proceeding
  - Check version: `docker --version` (should be 20.10+)

### NOT Required:
- ❌ Go installation
- ❌ Node.js installation
- ❌ PostgreSQL installation
- ❌ Redis installation

Everything is handled by Docker!

---

## Quick Start (3 Steps)

### Step 1: Download/Clone Whatomate

If you haven't already:
```cmd
cd C:\Development
git clone https://github.com/yourusername/whatomate.git
cd whatomate
```

### Step 2: Start Docker Desktop

1. Open **Docker Desktop** from Start Menu
2. Wait for it to fully start (icon turns green)
3. Keep it running

### Step 3: Run Setup Script

Double-click or run:
```cmd
START_DOCKER_WINDOWS.bat
```

This wizard will:
1. ✅ Check if Docker is running
2. ✅ Build Whatomate from source (including embedded signup feature)
3. ✅ Start PostgreSQL, Redis, Backend, and Frontend
4. ✅ Run database migrations automatically
5. ✅ Open Whatomate in your browser

**That's it!** 🎉

---

## Access Your Instance

After setup completes:

- **Frontend**: http://localhost:5173
- **Backend API**: http://localhost:8080
- **Default Login**:
  - Email: `admin@admin.com`
  - Password: `admin`

**To see Embedded Signup:**
1. Login at http://localhost:5173
2. Click **Settings** in sidebar
3. Click **Embedded Signup**
4. Create your first signup configuration

---

## Managing Services

### View Logs
See what's happening in real-time:
```cmd
docker-logs.bat
```
Or manually:
```cmd
docker-compose -f docker-compose.dev.yml logs -f
```

### Stop Services
Stop all containers (data is preserved):
```cmd
docker-stop.bat
```

### Restart Services
After making changes:
```cmd
docker-restart.bat
```

### Start Services Again
After stopping:
```cmd
docker-compose -f docker-compose.dev.yml up -d
```

### Check Status
```cmd
docker-compose -f docker-compose.dev.yml ps
```

You should see:
```
NAME                 STATUS
whatomate_backend    Up (healthy)
whatomate_db         Up (healthy)
whatomate_frontend   Up
whatomate_redis      Up (healthy)
```

---

## Complete Cleanup (Nuclear Option)

⚠️ **Warning**: This deletes your database!

If you want to start completely fresh:
```cmd
docker-clean.bat
```

This will:
- Stop all containers
- Remove all containers
- Delete all volumes (including database)

Then run `START_DOCKER_WINDOWS.bat` again to start fresh.

---

## Troubleshooting

### "Docker is not running"

**Solution:**
1. Open Docker Desktop from Start Menu
2. Wait for it to fully start (green icon in system tray)
3. Run the script again

### "Port already in use"

If you see errors about ports 5173, 8080, 5432, or 6379:

**Option 1: Stop the conflicting service**
```cmd
# Find what's using port 8080 (example)
netstat -ano | findstr :8080

# Kill the process (replace PID with actual number)
taskkill /PID <PID> /F
```

**Option 2: Change ports in docker-compose.dev.yml**
```yaml
services:
  backend:
    ports:
      - "8081:8080"  # Change 8081 to any free port
```

### "Build failed" or "npm install failed"

**Solution:**
```cmd
# Clean everything and rebuild
docker-clean.bat

# Start fresh
START_DOCKER_WINDOWS.bat
```

### "Embedded Signup not showing in menu"

**Check 1: User Role**
- Only **Admin** and **Manager** roles can see Embedded Signup
- Agents won't see this menu item
- Login as admin: `admin@admin.com` / `admin`

**Check 2: Clear browser cache**
- Press `Ctrl + Shift + Delete`
- Clear cache and reload
- Or try Incognito mode: `Ctrl + Shift + N`

**Check 3: Check backend logs**
```cmd
docker-logs.bat
```
Look for any errors in the backend startup

### Container keeps restarting

**Check logs:**
```cmd
docker-compose -f docker-compose.dev.yml logs backend
```

Common issues:
- Database connection failed → Check if PostgreSQL container is healthy
- Redis connection failed → Check if Redis container is running
- Config file error → Check config.docker.toml syntax

### Frontend shows "Cannot connect to backend"

**Check if backend is running:**
```cmd
docker-compose -f docker-compose.dev.yml ps backend
```

**Test backend directly:**
```cmd
curl http://localhost:8080/health
```

Should return:
```json
{"status": "ok"}
```

### Database migration errors

**Check PostgreSQL logs:**
```cmd
docker-compose -f docker-compose.dev.yml logs db
```

**Manually run migrations:**
```cmd
docker-compose -f docker-compose.dev.yml exec backend ./whatomate server -config config.toml -migrate
```

---

## Configuration

### Default Settings

The `config.docker.toml` file contains default settings for Docker:

```toml
[database]
host = "db"        # Docker service name
port = 5432
user = "whatomate"
password = "whatomate"
name = "whatomate"

[redis]
host = "redis"     # Docker service name
port = 6379
```

### Changing Database Credentials

Edit `config.docker.toml` AND `docker-compose.dev.yml`:

**In config.docker.toml:**
```toml
[database]
user = "myuser"
password = "mypassword"
```

**In docker-compose.dev.yml:**
```yaml
db:
  environment:
    POSTGRES_USER: myuser
    POSTGRES_PASSWORD: mypassword
```

Then restart:
```cmd
docker-restart.bat
```

---

## Architecture

When you run the Docker setup, the following happens:

```
┌─────────────────────────────────────────────────────┐
│                 Your Windows PC                      │
│                                                      │
│  ┌──────────────────────────────────────────────┐  │
│  │           Docker Desktop                      │  │
│  │                                               │  │
│  │  ┌─────────────┐  ┌──────────────┐          │  │
│  │  │ PostgreSQL  │  │    Redis     │          │  │
│  │  │  (port      │  │  (port 6379) │          │  │
│  │  │   5432)     │  │              │          │  │
│  │  └──────┬──────┘  └──────┬───────┘          │  │
│  │         │                │                   │  │
│  │         └────────┬───────┘                   │  │
│  │                  │                           │  │
│  │         ┌────────▼────────┐                  │  │
│  │         │  Whatomate      │                  │  │
│  │         │  Backend        │                  │  │
│  │         │  (port 8080)    │                  │  │
│  │         └────────┬────────┘                  │  │
│  │                  │                           │  │
│  │         ┌────────▼────────┐                  │  │
│  │         │  Whatomate      │                  │  │
│  │         │  Frontend       │                  │  │
│  │         │  (port 5173)    │◄─── Browser     │  │
│  │         └─────────────────┘     Connects    │  │
│  │                                              │  │
│  └──────────────────────────────────────────────┘  │
│                                                      │
└─────────────────────────────────────────────────────┘
```

**All services communicate within Docker network, but are accessible from Windows via localhost**

---

## Development Workflow

### Making Code Changes

1. **Edit backend code** (Go files)
   ```cmd
   # Rebuild backend only
   docker-compose -f docker-compose.dev.yml build backend
   docker-compose -f docker-compose.dev.yml restart backend
   ```

2. **Edit frontend code** (Vue files)
   ```cmd
   # Rebuild frontend only
   docker-compose -f docker-compose.dev.yml build frontend
   docker-compose -f docker-compose.dev.yml restart frontend
   ```

3. **Edit both**
   ```cmd
   # Rebuild everything
   docker-compose -f docker-compose.dev.yml build
   docker-compose -f docker-compose.dev.yml restart
   ```

### Accessing Database

**Via command line:**
```cmd
docker-compose -f docker-compose.dev.yml exec db psql -U whatomate -d whatomate
```

**Via external tool (like pgAdmin, DBeaver):**
- Host: `localhost`
- Port: `5432`
- User: `whatomate`
- Password: `whatomate`
- Database: `whatomate`

### Accessing Redis

**Via command line:**
```cmd
docker-compose -f docker-compose.dev.yml exec redis redis-cli
```

---

## Updating Whatomate

When new code is available:

```cmd
# Pull latest changes
git pull

# Rebuild and restart
docker-compose -f docker-compose.dev.yml build
docker-compose -f docker-compose.dev.yml up -d

# Check logs
docker-logs.bat
```

---

## Production Deployment

For production, use the standard `docker-compose.yml` instead:

```cmd
docker-compose up -d
```

This uses the pre-built image and is more efficient.

**Remember to:**
1. Change JWT secret in config.toml
2. Use strong database passwords
3. Set `environment = "production"`
4. Set `debug = false`

---

## Comparison: Docker vs Local Setup

| Feature | Docker Setup | Local Setup |
|---------|-------------|-------------|
| **Installation** | Only Docker Desktop needed | Go, Node, PostgreSQL, Redis needed |
| **Setup Time** | 5-10 minutes | 20-30 minutes |
| **Configuration** | One config file | Multiple config files |
| **Cleanup** | One command | Manual uninstall of all tools |
| **Isolation** | Fully isolated | Can conflict with other apps |
| **Updates** | Rebuild containers | Update each tool separately |
| **Best for** | Testing, quick start | Active development |

---

## FAQ

### Q: Do I need to install Go or Node.js?
**A:** No! Everything runs in Docker containers.

### Q: Can I access the database from other tools?
**A:** Yes! PostgreSQL is exposed on `localhost:5432`. Use any database client.

### Q: What happens to my data when I stop containers?
**A:** Data is preserved in Docker volumes. It persists across container restarts.

### Q: How do I delete everything and start fresh?
**A:** Run `docker-clean.bat` then `START_DOCKER_WINDOWS.bat`

### Q: Can I run this on Linux or Mac?
**A:** Yes! Use the same docker-compose.dev.yml file, but run:
```bash
docker-compose -f docker-compose.dev.yml up -d
```

### Q: Is this production-ready?
**A:** This setup is for development. For production, use the standard docker-compose.yml with proper secrets.

### Q: Can I use Docker Desktop alternatives (Podman, etc.)?
**A:** The docker-compose files should work with docker-compatible tools, but Docker Desktop is officially supported.

---

## Next Steps

Once your Docker setup is running:

1. ✅ Login at http://localhost:5173
2. ✅ Go to Settings → Embedded Signup
3. ✅ Create your first signup configuration:
   - Name: "Test Signup"
   - Select a WhatsApp account
   - Enter Meta App credentials (or use dummy values for testing UI)
4. ✅ Click the `</>` icon to get integration code
5. ✅ Test the embedded signup on a local HTML file

See `docs/embedded-signup.md` for complete feature documentation.

---

## Support

- **Docker Desktop Issues**: https://docs.docker.com/desktop/troubleshoot/overview/
- **Whatomate Issues**: Check logs with `docker-logs.bat`
- **Embedded Signup Docs**: `docs/embedded-signup.md`

---

## Success Checklist

- [ ] Docker Desktop is installed and running
- [ ] Ran `START_DOCKER_WINDOWS.bat` successfully
- [ ] All 4 containers are running (check: `docker-compose -f docker-compose.dev.yml ps`)
- [ ] Can access http://localhost:5173
- [ ] Can login with admin credentials
- [ ] See "Embedded Signup" in Settings menu
- [ ] Can create test signup configuration

If all checked ✅, you're ready to use Whatomate with Embedded Signup! 🎉

---

## Quick Command Reference

| Task | Command |
|------|---------|
| **Start everything** | `START_DOCKER_WINDOWS.bat` |
| **Stop services** | `docker-stop.bat` |
| **View logs** | `docker-logs.bat` |
| **Restart services** | `docker-restart.bat` |
| **Clean everything** | `docker-clean.bat` |
| **Check status** | `docker-compose -f docker-compose.dev.yml ps` |
| **Rebuild after code change** | `docker-compose -f docker-compose.dev.yml build` |
| **Access database** | `docker-compose -f docker-compose.dev.yml exec db psql -U whatomate` |
| **Access Redis** | `docker-compose -f docker-compose.dev.yml exec redis redis-cli` |
