# Embedded Signup Setup for Windows

## Prerequisites

Make sure you have installed:
- ✅ **Go 1.24+** - Download from https://go.dev/dl/
- ✅ **Node.js 18+** - Download from https://nodejs.org/
- ✅ **PostgreSQL** - Running locally or via Docker
- ✅ **Redis** - Running locally or via Docker

Check versions:
```cmd
go version
node --version
npm --version
```

---

## Quick Start (3 Steps)

### **Step 1: Run Database Migrations** 📦

Open **Command Prompt** or **PowerShell** in the project directory:

```cmd
cd C:\path\to\whatomate

REM Run the setup script
start-with-embedded-signup.bat
```

**Or manually:**
```cmd
REM If config.toml doesn't exist, copy from example
copy config.example.toml config.toml

REM Run migrations
go run cmd/whatomate/main.go -config config.toml -migrate
```

You should see:
```
✓ Running migrations
✓ Migration completed
```

---

### **Step 2: Start Backend** 🚀

Open a **new Command Prompt** window:

```cmd
cd C:\path\to\whatomate

REM Start the backend
go run cmd/whatomate/main.go -config config.toml
```

Wait for:
```
✓ Server started on :8080
✓ Connected to database
```

**Keep this window open!**

---

### **Step 3: Start Frontend** 🎨

Open **another Command Prompt** window:

```cmd
cd C:\path\to\whatomate\frontend

REM Install dependencies (first time only)
npm install

REM Start dev server
npm run dev
```

Wait for:
```
➜  Local:   http://localhost:5173/
```

**Keep this window open too!**

---

## ✅ Verify It's Working

1. Open browser: **http://localhost:5173**
2. Login with admin credentials
3. Click **Settings** in sidebar
4. Look for **"Embedded Signup"** in the submenu

You should see:
```
Settings
├── General
├── Chatbot
├── Accounts
├── Embedded Signup  ← HERE!
├── Canned Responses
└── Teams
```

---

## 🔧 Alternative: Using PowerShell

If you prefer PowerShell:

**Terminal 1 - Backend:**
```powershell
cd C:\path\to\whatomate
go run cmd/whatomate/main.go -config config.toml
```

**Terminal 2 - Frontend:**
```powershell
cd C:\path\to\whatomate\frontend
npm install
npm run dev
```

---

## 🐳 Using Docker Desktop (Easiest)

If you have Docker Desktop installed:

```cmd
cd C:\path\to\whatomate\docker

REM Start PostgreSQL and Redis
docker-compose up -d db redis

REM Wait 10 seconds for DB to be ready
timeout /t 10

REM Run migrations
cd ..
go run cmd/whatomate/main.go -config config.toml -migrate

REM Start backend
go run cmd/whatomate/main.go -config config.toml
```

In another terminal:
```cmd
cd C:\path\to\whatomate\frontend
npm run dev
```

---

## 🔍 Troubleshooting

### "Port already in use" Error

**Backend (port 8080):**
```cmd
REM Find process using port 8080
netstat -ano | findstr :8080

REM Kill the process (replace PID with actual number)
taskkill /PID <PID> /F
```

**Frontend (port 5173):**
```cmd
REM Find process using port 5173
netstat -ano | findstr :5173

REM Kill the process
taskkill /PID <PID> /F
```

### PostgreSQL Connection Error

Edit `config.toml`:
```toml
[database]
host = "localhost"  # Use "localhost" on Windows (not "db")
port = 5432
user = "postgres"   # Default Windows user
password = "your_password"
name = "whatomate"
```

Create database if it doesn't exist:
```cmd
REM Open psql
psql -U postgres

-- Inside psql:
CREATE DATABASE whatomate;
\q
```

### Redis Connection Error

If Redis is not installed:

**Option A: Docker**
```cmd
docker run -d --name redis -p 6379:6379 redis:alpine
```

**Option B: Download Redis for Windows**
- Download from: https://github.com/microsoftarchive/redis/releases
- Extract and run `redis-server.exe`

### "Embedded Signup" Not Showing

**Clear npm cache and rebuild:**
```cmd
cd frontend
rmdir /s /q node_modules
rmdir /s /q .vite
npm install
npm run dev
```

**Clear browser cache:**
- Press `Ctrl + Shift + Delete`
- Or use Incognito mode: `Ctrl + Shift + N`

### Database Tables Not Created

Check if tables exist:
```cmd
psql -U postgres -d whatomate

-- Inside psql:
\dt embedded_*
```

You should see:
- `embedded_signups`
- `embedded_signup_leads`

If not, run migrations again:
```cmd
go run cmd/whatomate/main.go -config config.toml -migrate
```

---

## 📝 Build Production Binary (Optional)

To create a single `.exe` file with embedded frontend:

```cmd
cd frontend
npm install
npm run build

cd ..
REM Copy frontend build
xcopy /E /I frontend\dist internal\frontend\dist

REM Build Windows executable
go build -ldflags "-s -w" -o whatomate.exe cmd/whatomate/main.go

REM Run it
whatomate.exe -config config.toml
```

---

## 🚀 Quick Commands Reference

**Start Everything (3 terminals needed):**

**Terminal 1 - Database Migration (one time):**
```cmd
cd C:\path\to\whatomate
go run cmd/whatomate/main.go -config config.toml -migrate
```

**Terminal 2 - Backend:**
```cmd
cd C:\path\to\whatomate
go run cmd/whatomate/main.go -config config.toml
```

**Terminal 3 - Frontend:**
```cmd
cd C:\path\to\whatomate\frontend
npm run dev
```

**Access:**
- Frontend: http://localhost:5173
- Backend API: http://localhost:8080
- Settings → Embedded Signup: http://localhost:5173/settings/embedded-signup

---

## 🎯 Quick Test

Once running, test the UI:

1. Go to: http://localhost:5173
2. Login (default: admin@admin.com / admin)
3. Click **Settings → Embedded Signup**
4. Click **"+ Create Signup"**
5. Fill dummy data:
   - Name: `Test Signup`
   - WhatsApp Account: Select any
   - Meta App ID: `123456789`
   - Meta Config ID: `test-config`
   - Meta App Secret: `test-secret`
6. Click **"Create"**
7. Click `</>` icon to see generated code

---

## 💡 Tips for Windows

- Use **Windows Terminal** for better experience
- Or use **Git Bash** if you have Git installed
- Use `CTRL + C` to stop servers
- Keep both terminals open while developing
- Use `code .` to open in VS Code

---

## 🆘 Need Help?

- Check backend logs in Terminal 2
- Check frontend logs in Terminal 3
- Open browser DevTools (F12) → Console tab
- See full docs: `docs/embedded-signup.md`

---

## ✅ Success Checklist

- [ ] PostgreSQL is running
- [ ] Redis is running (optional, but recommended)
- [ ] Migrations completed successfully
- [ ] Backend started on port 8080
- [ ] Frontend started on port 5173
- [ ] Can login to http://localhost:5173
- [ ] See "Embedded Signup" in Settings menu
- [ ] Can create test signup configuration

If all checked ✅, you're ready to go! 🎉
