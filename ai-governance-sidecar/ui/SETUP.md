# React UI Setup Guide

Production-ready React UI for AI Governance Sidecar approval management.

## Project Structure

```
ui/
├── public/
│   └── index.html
├── src/
│   ├── components/
│   │   ├── ApprovalDashboard.js    # Main dashboard
│   │   ├── ApprovalCard.js         # Individual approval cards
│   │   └── AuditLog.js             # Audit trail viewer
│   ├── services/
│   │   ├── api.js                  # API calls
│   │   └── WebSocketProvider.js   # Real-time WebSocket
│   ├── App.js                      # Main app component
│   ├── App.css                     # App styles
│   ├── index.js                    # Entry point
│   └── index.css                   # Global styles
├── package.json
├── tailwind.config.js
└── SETUP.md (this file)
```

## Prerequisites

- Node.js 16+ and npm
- Backend service running on http://localhost:8080

## Installation

```
# Navigate to ui directory
cd ui

# Install dependencies
npm install

# Install Tailwind CSS
npm install -D tailwindcss postcss autoprefixer
npx tailwindcss init
```

## Development

```
# Start development server
npm start

# Opens at http://localhost:3000
# Proxies API calls to http://localhost:8080
```

## Build for Production

```
# Create production build
npm run build

# Output: ui/build/
```

## Environment Variables

Create `.env` file in `ui/` directory:

```
# API Base URL
REACT_APP_API_URL=http://localhost:8080

# WebSocket URL
REACT_APP_WS_URL=ws://localhost:8080/ws
```

For production:
```
REACT_APP_API_URL=https://your-domain.com
REACT_APP_WS_URL=wss://your-domain.com/ws
```

## Features

### ✅ Real-time Updates
- WebSocket connection for instant approval notifications
- Automatic reconnection on disconnect
- Live connection status indicator

### ✅ Approval Management
- View all pending approvals
- Approve with approver name and optional comment
- Deny with required reason
- Auto-refresh on WebSocket updates
- Optimistic UI updates

### ✅ Audit Trail
- Complete history of all decisions
- Filter by status
- Expandable details for each entry
- Timestamp formatting

### ✅ Production Ready
- Error handling with user-friendly messages
- Loading states
- Responsive design
- Accessibility features
- Clean, modern UI

## Testing the UI

### 1. Start Backend
```
# In project root
make docker-up
```

### 2. Start Frontend
```
# In ui/ directory
npm start
```

### 3. Create Test Approval
```
# In another terminal
curl -X POST http://localhost:8080/proxy \
  -H "Content-Type: application/json" \
  -d '{
    "tool": "database",
    "action": "query",
    "parameters": {"query": "SELECT password FROM users"}
  }'
```

### 4. Check UI
- Go to http://localhost:3000
- Should see the approval request appear immediately
- Click "Approve" or "Deny"
- Enter your name/email
- Add optional comment
- Submit

### 5. Check Audit Log
- Click "Audit Log" tab
- Should see your approval/denial decision

## WebSocket Testing

Open browser console:
```
// Should see these logs:
// "WebSocket connected"
// "WebSocket message: {type: 'approval_update', ...}"
```

## API Endpoints Used

- `GET /approvals/pending` - Fetch pending approvals
- `POST /approvals/:id/approve` - Approve request
- `POST /approvals/:id/deny` - Deny request
- `GET /audit` - Fetch audit log
- `GET /health` - Health check
- `WS /ws` - WebSocket connection

## Troubleshooting

### WebSocket Won't Connect
```
# Check backend WebSocket endpoint
curl -i -N \
  -H "Connection: Upgrade" \
  -H "Upgrade: websocket" \
  http://localhost:8080/ws
```

### API Calls Failing
```
# Check backend is running
curl http://localhost:8080/health

# Check CORS (should allow localhost:3000)
```

### Build Errors
```
# Clear cache and reinstall
rm -rf node_modules package-lock.json
npm install
```

### Tailwind Not Working
```
# Ensure tailwind is installed
npm install -D tailwindcss postcss autoprefixer

# Rebuild
npm start
```

## Deployment Options

### Option 1: Static Hosting (Recommended)

```
# Build
npm run build

# Serve with any static host:
# - Netlify
# - Vercel
# - AWS S3 + CloudFront
# - GitHub Pages
```

### Option 2: Docker

Create `ui/Dockerfile`:
```
FROM node:18-alpine AS build

WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=build /app/build /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

Create `ui/nginx.conf`:
```
server {
  listen 80;
  location / {
    root /usr/share/nginx/html;
    index index.html;
    try_files $uri /index.html;
  }
  location /api {
    proxy_pass http://governance-sidecar:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection 'upgrade';
    proxy_set_header Host $host;
    proxy_cache_bypass $http_upgrade;
  }
  location /ws {
    proxy_pass http://governance-sidecar:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "Upgrade";
    proxy_set_header Host $host;
  }
}
```

Add to `docker-compose.yml`:
```
  ui:
    build: ./ui
    container_name: governance-ui
    ports:
      - "3000:80"
    depends_on:
      - governance-sidecar
    networks:
      - sidecar-net
```

### Option 3: Serve from Backend

```
# Build UI
cd ui && npm run build

# Copy to Go backend static directory
cp -r build ../internal/server/static

# Update Go server to serve static files
```

## Production Checklist

- [ ] Environment variables configured for production URLs
- [ ] WebSocket uses WSS (secure WebSocket)
- [ ] API uses HTTPS
- [ ] Build optimized: `npm run build`
- [ ] CORS configured on backend
- [ ] Authentication implemented (if needed)
- [ ] Error boundaries added
- [ ] Analytics integrated (optional)
- [ ] Performance testing completed
- [ ] Mobile responsiveness verified

## Next Steps

1. Add authentication/authorization
2. Implement user preferences
3. Add notification system
4. Create advanced filters
5. Add export functionality
6. Implement analytics dashboard
7. Add dark mode

## Support

Check backend logs:
```
docker-compose logs -f governance-sidecar
```

Check frontend console for errors in browser DevTools.
