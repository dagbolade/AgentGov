#!/bin/bash

set -e

BOLD='\033[1m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BOLD}${BLUE}React UI Setup${NC}"
echo "=============="
echo ""

# Check Node.js
if ! command -v node &> /dev/null; then
    echo -e "${YELLOW}⚠ Node.js not found${NC}"
    echo "Install Node.js 16+ from: https://nodejs.org/"
    exit 1
fi

NODE_VERSION=$(node -v | cut -d'v' -f2 | cut -d'.' -f1)
if [ "$NODE_VERSION" -lt 16 ]; then
    echo -e "${YELLOW}⚠ Node.js version $NODE_VERSION is too old${NC}"
    echo "Please upgrade to Node.js 16 or higher"
    exit 1
fi

echo -e "${GREEN}✓ Node.js $(node -v) detected${NC}"

# Check npm
if ! command -v npm &> /dev/null; then
    echo -e "${YELLOW}⚠ npm not found${NC}"
    exit 1
fi

echo -e "${GREEN}✓ npm $(npm -v) detected${NC}"
echo ""

# Create .env if it doesn't exist
if [ ! -f .env ]; then
    echo -e "${BOLD}Creating .env file...${NC}"
    cat > .env << EOF
# API Base URL
REACT_APP_API_URL=http://localhost:8080

# WebSocket URL
REACT_APP_WS_URL=ws://localhost:8080/ws
EOF
    echo -e "${GREEN}✓ Created .env file${NC}"
else
    echo -e "${YELLOW}⚠ .env already exists, skipping${NC}"
fi

# Install dependencies
echo ""
echo -e "${BOLD}Installing dependencies...${NC}"
npm install

# Install Tailwind CSS if not present
if ! grep -q "tailwindcss" package.json; then
    echo ""
    echo -e "${BOLD}Installing Tailwind CSS...${NC}"
    npm install -D tailwindcss postcss autoprefixer
    npx tailwindcss init
fi

echo ""
echo -e "${GREEN}✓ Dependencies installed${NC}"

# Create postcss config if missing
if [ ! -f postcss.config.js ]; then
    echo ""
    echo -e "${BOLD}Creating PostCSS config...${NC}"
    cat > postcss.config.js << EOF
module.exports = {
  plugins: {
    tailwindcss: {},
    autoprefixer: {},
  },
}
EOF
    echo -e "${GREEN}✓ Created postcss.config.js${NC}"
fi

echo ""
echo -e "${BOLD}${GREEN}============================================${NC}"
echo -e "${BOLD}${GREEN}    Setup Complete! 🚀${NC}"
echo -e "${BOLD}${GREEN}============================================${NC}"
echo ""
echo -e "${BOLD}Next steps:${NC}"
echo ""
echo "  1. Start the backend:"
echo "     ${BLUE}cd .. && make docker-up${NC}"
echo ""
echo "  2. Start the UI development server:"
echo "     ${BLUE}npm start${NC}"
echo ""
echo "  3. Open in browser:"
echo "     ${BLUE}http://localhost:3000${NC}"
echo ""
echo -e "${BOLD}For production build:${NC}"
echo "     ${BLUE}npm run build${NC}"
echo ""
echo -e "${BOLD}Troubleshooting:${NC}"
echo "  - Check backend is running: ${BLUE}curl http://localhost:8080/health${NC}"
echo "  - View logs: ${BLUE}npm start${NC} (shows errors)"
echo "  - Clear cache: ${BLUE}rm -rf node_modules && npm install${NC}"
echo ""