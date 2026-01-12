#!/bin/bash

echo "🚀 Starting Whatomate with Embedded Signup Feature"
echo "=================================================="
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Check if config exists
if [ ! -f "config.toml" ]; then
    echo -e "${YELLOW}⚠️  config.toml not found. Copying from example...${NC}"
    cp config.example.toml config.toml
    echo -e "${GREEN}✓ Created config.toml${NC}"
    echo -e "${YELLOW}⚠️  Please edit config.toml with your database credentials${NC}"
    echo ""
fi

# Step 1: Run migrations
echo -e "${YELLOW}📦 Step 1: Running database migrations...${NC}"
go run cmd/whatomate/main.go -config config.toml -migrate

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Migrations completed successfully${NC}"
else
    echo -e "${RED}✗ Migration failed. Please check your database connection.${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}✓ Setup complete!${NC}"
echo ""
echo "Next steps:"
echo "  1. Start backend:  make run    (or: go run cmd/whatomate/main.go -config config.toml)"
echo "  2. Start frontend: cd frontend && npm run dev"
echo "  3. Open: http://localhost:5173"
echo "  4. Navigate to: Settings → Embedded Signup"
echo ""
echo "Or use: make dev  (to start both at once)"
