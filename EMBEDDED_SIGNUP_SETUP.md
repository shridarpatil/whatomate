# Embedded Signup Local Setup Guide

## Prerequisites
Make sure you have:
- Go 1.24+ installed
- Node.js 18+ installed
- PostgreSQL running
- Redis running

## Step-by-Step Setup

### 1. Run Database Migrations

The embedded signup feature requires new database tables. Run migrations:

```bash
# Stop any running instance first
pkill whatomate

# Run migrations
make run-migrate
# OR
go run cmd/whatomate/main.go -config config.toml -migrate
```

This will create:
- `embedded_signups` table
- `embedded_signup_leads` table
- Indexes for performance

### 2. Restart Backend

```bash
# If running directly
make run
# OR
go run cmd/whatomate/main.go -config config.toml

# If using Docker
cd docker
docker-compose restart
```

The backend will now have the new API endpoints:
- `GET /api/embedded-signups`
- `POST /api/embedded-signups`
- `GET /api/embedded-signup/{id}/config`
- `POST /api/embedded-signup/{id}/submit`
- etc.

### 3. Restart Frontend

**Option A: Development Mode (Recommended)**
```bash
# In a new terminal
cd frontend
npm install  # First time only
npm run dev
```

Frontend will be available at: http://localhost:5173

**Option B: Production Build**
```bash
make frontend-build
make build-prod
./whatomate -config config.toml
```

### 4. Verify Installation

1. **Login to Whatomate**
   - Open: http://localhost:5173 (dev) or http://localhost:8080 (prod)
   - Login with your admin credentials

2. **Navigate to Settings**
   - Click "Settings" in the left sidebar
   - You should see "Embedded Signup" in the submenu

3. **Check Navigation Menu**
   Look for this in the Settings submenu:
   ```
   Settings
   ├── General
   ├── Chatbot
   ├── Accounts
   ├── Embedded Signup  ← NEW!
   ├── Canned Responses
   └── ...
   ```

## Troubleshooting

### "Embedded Signup" Not Showing in Menu

**Check 1: User Role**
- Embedded Signup requires **Manager** or **Admin** role
- Agents won't see this option
- Verify your role: Go to Profile and check your role

**Check 2: Frontend Build**
```bash
# Rebuild frontend
cd frontend
rm -rf node_modules/.vite
npm run dev
```

**Check 3: Browser Cache**
- Clear browser cache
- Or open in Incognito/Private mode
- Hard refresh: Ctrl+Shift+R (Windows/Linux) or Cmd+Shift+R (Mac)

**Check 4: Check Console**
```bash
# Frontend console
# Open browser DevTools (F12)
# Look for any errors in Console tab

# Backend logs
# Check terminal where backend is running
# Look for route registration messages
```

### Database Migration Failed

```bash
# Check if PostgreSQL is running
psql -U whatomate -d whatomate -c "SELECT 1"

# Manually create tables (if needed)
psql -U whatomate -d whatomate

# Inside psql:
\dt embedded_*

# You should see:
# embedded_signups
# embedded_signup_leads
```

### API Endpoints Not Working

```bash
# Test backend is running with new routes
curl http://localhost:8080/api/embedded-signups \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"

# Should return:
# {"signups": []}
```

## Development Mode (Full Stack)

Run both backend and frontend together:

```bash
# Terminal 1: Backend with migrations
make run-migrate

# Terminal 2: Frontend dev server
cd frontend
npm run dev
```

- Backend: http://localhost:8080
- Frontend: http://localhost:5173
- Frontend proxies API calls to backend

## Configuration

Edit `config.toml` to ensure WhatsApp section is present:

```toml
[whatsapp]
webhook_verify_token = "your-webhook-verify-token"
api_version = "v24.0"
base_url = "https://graph.facebook.com"
```

## Next Steps

Once you see "Embedded Signup" in the settings:

1. Click "Create Signup"
2. Fill in the form (you'll need Meta credentials)
3. Click the `</>` icon to get integration code
4. Embed on your website!

## Need Help?

Check the main documentation:
- `docs/embedded-signup.md` - Complete feature documentation
- Backend code: `internal/handlers/embedded_signup.go`
- Frontend code: `frontend/src/views/settings/EmbeddedSignupView.vue`

## Quick Test Without Meta Credentials

You can test the UI without Meta credentials:

1. Go to Settings → Embedded Signup
2. Click "Create Signup"
3. Fill in dummy values:
   - Name: "Test Signup"
   - WhatsApp Account: Select any
   - Meta App ID: "123456789"
   - Meta Config ID: "test-config"
   - Meta App Secret: "test-secret"
4. Click "Create"
5. You should see the signup card
6. Click `</>` to see generated integration code

The signup won't work for real users yet (needs real Meta credentials), but you can verify the UI is working!
