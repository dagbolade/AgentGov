# React UI Deployment Guide

Complete guide for deploying the React UI with the AI Governance Sidecar.

## Quick Start

```
# Option 1: Development (Hot Reload)
make ui-setup
make docker-up       # Start backend
make ui-dev          # Start UI (separate terminal)

# Option 2: Production (Docker)
make docker-up-with-ui
```

## Directory Structure

```
ai-governance-sidecar/
├── ui/                          # React UI (NEW)
│   ├── public/
│   │   └── index.html
│   ├── src/
│   │   ├── components/
│   │   │   ├── ApprovalDashboard.js
│   │   │   ├── ApprovalCard.js
│   │   │   └── AuditLog.js
│   │   ├── services/
│   │   │   ├── api.js
│   │   │   └── WebSocketProvider.js
│   │   ├── App.js
│   │   ├── App.css
│   │   ├── index.js
│   │   └── index.css
│   ├── Dockerfile
│   ├── nginx.conf
│   ├── package.json
│   ├── tailwind.config.js
│   ├── setup.sh
│   └── SETUP.md
├── policies/                    # WASM policies
├── cmd/                         # Go backend
├── internal/                    # Go backend
├── docker-compose.yml           # Backend only
├── docker-compose.with-ui.yml   # Backend + UI
└── Makefile                     # Build automation
```

## Setup Options

### Option 1: Development Mode (Recommended for Development)

**Best for**: Active development, hot reload, debugging

```
# 1. Setup UI (one-time)
cd ui
chmod +x setup.sh
./setup.sh

# OR use Make
make ui-setup

# 2. Start backend
make docker-up

# 3. Start UI dev server (in new terminal)
cd ui
npm start

# OR use Make
make ui-dev
```

Access:
- **UI**: http://localhost:3000 (auto-reload on changes)
- **Backend**: http://localhost:8080
- **WebSocket**: ws://localhost:8080/ws

### Option 2: Docker Production Mode

**Best for**: Production deployment, testing production build

```
# 1. Build everything
make policies              # Build WASM policies
make ui-build             # Build React production bundle

# 2. Start with Docker Compose
make docker-up-with-ui

# OR manually
docker-compose -f docker-compose.with-ui.yml up -d
```

Access:
- **UI**: http://localhost:3000 (served by nginx)
- **Backend**: http://localhost:8080
- **WebSocket**: Proxied through nginx

### Option 3: Separate Deployments

**Best for**: Microservices architecture, CDN hosting

**Backend:**
```
make docker-up
```

**UI (Static Hosting):**
```
cd ui
npm run build

# Deploy build/ directory to:
# - Netlify
# - Vercel
# - AWS S3 + CloudFront
# - GitHub Pages
```

Update `ui/.env.production`:
```
REACT_APP_API_URL=https://api.yourdomain.com
REACT_APP_WS_URL=wss://api.yourdomain.com/ws
```

## Features

### 🎯 Approval Management
- Real-time pending approvals list
- Approve/deny with approver identification
- Required comments for denials
- Expandable request details
- Time remaining indicators
- Confidence score visualization

### 📊 Audit Trail
- Complete decision history
- Filterable by status (approved/denied/expired)
- Expandable entry details
- Formatted timestamps
- Request details in JSON

### 🔄 Real-time Updates
- WebSocket connection with auto-reconnect
- Live status indicator
- Instant approval notifications
- Optimistic UI updates
- Connection status display

### 💎 Production Features
- Error boundaries
- Loading states
- Responsive design
- Accessibility (WCAG)
- Clean, modern UI
- Performance optimized
- SEO friendly

## Testing the Complete System

### 1. Start Services

```
# Backend + UI
make docker-up-with-ui

# Verify both are running
curl http://localhost:8080/health  # Backend
curl http://localhost:3000         # UI
```

### 2. Create Test Approval

```
# Send request with sensitive data
curl -X POST http://localhost:8080/proxy \
  -H "Content-Type: application/json" \
  -d '{
    "tool": "database",
    "action": "query",
    "parameters": {
      "query": "SELECT password, credit_card FROM users WHERE id=1"
    },
    "context": {}
  }'
```

### 3. Check UI

1. Open http://localhost:3000
2. Should see approval request appear immediately
3. WebSocket status should show "Live" (green dot)
4. Click request to expand details
5. Review request parameters

### 4. Approve/Deny

1. Click "Approve" or "Deny"
2. Enter your name/email
3. Add comment (optional for approve, required for deny)
4. Submit
5. Request disappears from list immediately
6. Check "Audit Log" tab to see decision

### 5. Verify Real-time Updates

Open UI in two browser windows:
1. Window 1: Leave on "Pending Approvals"
2. Window 2: Send approval request via curl
3. Window 1: Should see new request appear immediately
4. Window 2: Approve the request in Window 1
5. Both windows: Request disappears instantly

## Environment Configuration

### Development (.env)
```
REACT_APP_API_URL=http://localhost:8080
REACT_APP_WS_URL=ws://localhost:8080/ws
```

### Production (.env.production)
```
REACT_APP_API_URL=https://governance.yourdomain.com
REACT_APP_WS_URL=wss://governance.yourdomain.com/ws
```

### Docker (docker-compose.with-ui.yml)
```
environment:
  - REACT_APP_API_URL=http://localhost:8080
  - REACT_APP_WS_URL=ws://localhost:8080/ws
```

## Nginx Configuration

The UI's nginx serves:
- **Static files**: React build artifacts
- **API proxy**: Forwards /approvals, /audit, /health to backend
- **WebSocket proxy**: Forwards /ws to backend with proper headers
- **Compression**: Gzip for text files
- **Caching**: 1 year for static assets
- **Security headers**: X-Frame-Options, CSP, etc.

## Troubleshooting

### WebSocket Not Connecting

**Symptom**: Red "Disconnected" indicator

**Solutions**:
```
# 1. Check backend WebSocket endpoint
curl -i -N -H "Connection: Upgrade" -H "Upgrade: websocket" \
  http://localhost:8080/ws

# 2. Check browser console for errors
# 3. Verify firewall allows WebSocket
# 4. Check backend logs
docker-compose logs -f governance-sidecar | grep -i websocket
```

### API Calls Failing

**Symptom**: Approvals not loading, errors in console

**Solutions**:
```
# 1. Check backend is running
curl http://localhost:8080/health

# 2. Check CORS settings in backend
# Backend must allow origin: http://localhost:3000

# 3. Check proxy configuration (if using nginx)
cat ui/nginx.conf

# 4. Verify environment variables
cat ui/.env
```

### UI Not Loading

**Symptom**: Blank page, build errors

**Solutions**:
```
# 1. Clear cache and rebuild
cd ui
rm -rf node_modules package-lock.json build
npm install
npm run build

# 2. Check for JavaScript errors
# Open browser DevTools > Console

# 3. Verify all files copied to build
ls ui/build

# 4. Check nginx configuration (Docker mode)
docker-compose exec governance-ui cat /etc/nginx/conf.d/default.conf
```

### Build Failures

**Symptom**: npm build fails

**Solutions**:
```
# 1. Check Node version (must be 16+)
node -v

# 2. Clear npm cache
npm cache clean --force

# 3. Delete and reinstall
rm -rf node_modules package-lock.json
npm install

# 4. Check for syntax errors
npm run build 2>&1 | grep -i error
```

## Production Deployment Checklist

### Pre-Deployment
- [ ] WASM policies built and tested
- [ ] Backend running and healthy
- [ ] UI dependencies installed
- [ ] Environment variables configured
- [ ] WebSocket tested
- [ ] HTTPS/WSS configured (production)
- [ ] CORS settings correct

### Build
- [ ] `make policies` completed
- [ ] `make ui-build` completed
- [ ] Build directory contains files
- [ ] No build errors/warnings
- [ ] Size optimization applied

### Docker
- [ ] Images built successfully
- [ ] Health checks passing
- [ ] Resource limits configured
- [ ] Volumes mounted correctly
- [ ] Networks configured
- [ ] Restart policies set

### Testing
- [ ] Approval workflow tested
- [ ] Real-time updates working
- [ ] Audit log accessible
- [ ] All browsers tested (Chrome, Firefox, Safari)
- [ ] Mobile responsiveness verified
- [ ] Performance acceptable (<2s load)

### Security
- [ ] HTTPS enabled (production)
- [ ] WSS enabled (production)
- [ ] Security headers configured
- [ ] No sensitive data in logs
- [ ] Rate limiting configured
- [ ] Authentication added (if needed)

### Monitoring
- [ ] Backend logs accessible
- [ ] UI error tracking (optional: Sentry)
- [ ] Health checks configured
- [ ] Metrics collection (optional: Prometheus)
- [ ] Alerts configured

## Performance Optimization

### Build Optimization
```
# Already configured in package.json
npm run build

# Produces optimized bundle:
# - Minified JavaScript
# - CSS extracted and minified
# - Images optimized
# - Source maps (separate)
```

### Nginx Caching
```
# Already configured in ui/nginx.conf
- Static assets: 1 year cache
- HTML: No cache (always fresh)
- Gzip compression enabled
```

### React Optimization
- Code splitting (automatic)
- Lazy loading (for future additions)
- Memo/useMemo for expensive operations
- WebSocket connection pooling

## Scaling Considerations

### Horizontal Scaling

**Backend:**
```
# In docker-compose.with-ui.yml
deploy:
  replicas: 3
```

**UI:**
- Static files scale infinitely
- Use CDN for global distribution
- Multiple nginx instances behind load balancer

### Load Balancing

```
upstream backend {
    server governance-sidecar-1:8080;
    server governance-sidecar-2:8080;
    server governance-sidecar-3:8080;
}

server {
    location /approvals {
        proxy_pass http://backend;
    }
}
```

### Database

Current SQLite setup works for:
- Single backend instance
- <1000 requests/day
- Development/small production

For larger scale, migrate to PostgreSQL.

## Next Steps

After successful deployment:

1. **Add Authentication**
   - Implement JWT tokens
   - Add login page
   - Secure WebSocket connections

2. **Enhanced Features**
   - User preferences
   - Advanced filters
   - Export functionality
   - Notification system

3. **Analytics**
   - Decision metrics
   - Response time tracking
   - User activity logs

4. **Monitoring**
   - APM integration (New Relic, DataDog)
   - Error tracking (Sentry)
   - Log aggregation (ELK Stack)

5. **Documentation**
   - API documentation
   - User guide
   - Admin handbook
