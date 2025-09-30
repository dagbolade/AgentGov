# AI Governance Sidecar

[![Test](https://github.com/dagbolade/ai-governance-sidecar/workflows/Test/badge.svg)](https://github.com/dagbolade/ai-governance-sidecar/actions)
[![codecov](https://codecov.io/gh/dagbolade/ai-governance-sidecar/branch/main/graph/badge.svg)](https://codecov.io/gh/dagbolade/ai-governance-sidecar)

A powerful AI governance system that provides real-time policy enforcement, approval workflows, and audit logging for AI tool calls. Features a modern React dashboard with WebSocket-based real-time updates and OPA (Open Policy Agent) integration.

## 🚀 Features

- **Policy Enforcement**: Uses Open Policy Agent (OPA) for flexible, declarative policy management
- **Real-Time Dashboard**: React-based UI with live updates via WebSockets
- **Approval Workflows**: Human-in-the-loop approval system for sensitive AI operations
- **Audit Logging**: Complete audit trail of all AI tool calls and decisions
- **Embedded UI**: Single binary deployment with embedded web assets

## 📁 Project Structure

```
ai-governance-sidecar/
├── cmd/sidecar/           # Main application entry point
├── internal/
│   ├── approval/          # Approval queue and workflow management
│   ├── audit/            # Audit logging and storage
│   ├── policy/           # OPA policy engine integration
│   ├── proxy/            # HTTP proxy for tool calls
│   └── server/           # HTTP server and API endpoints
│       └── web/          # React dashboard source code
├── policies/             # OPA policy files (.rego)
├── db/                  # SQLite database storage
└── web/                 # Built React assets (generated)
```

## 🛠️ Quick Start

### Prerequisites

- Go 1.21 or higher
- Node.js 18+ (for UI development)

### 1. Build the Application

```bash
cd ai-governance-sidecar
go build -o sidecar ./cmd/sidecar
```

### 2. Run the Server

```bash
./sidecar
```

The server will start on `http://localhost:8080` and show:
```
2025-09-30T15:45:16Z INF starting AI Governance Sidecar
2025-09-30T15:45:16Z INF policy loaded policy=allow_all
2025-09-30T15:45:16Z INF starting HTTP server port=8080
```

### 3. Access the Dashboard

Open `http://localhost:8080` in your browser to access the governance dashboard.

## 🔧 Configuration

### Policy Management

Policies are written in Rego (OPA's policy language) and stored in the `policies/` directory:

**Example Policy** (`policies/allow_all.rego`):
```rego
package policy

default allow = true
```

**Restrictive Policy Example**:
```rego
package policy

default allow = false

# Allow specific tools only
allow {
    input.tool_name == "safe_tool"
}

# Require human approval for sensitive operations
allow {
    input.tool_name == "sensitive_tool"
    input.metadata.approved_by
}
```

### Environment Variables

- `PORT`: HTTP server port (default: 8080)
- `DB_PATH`: SQLite database path (default: ./db/audit.db)
- `POLICY_DIR`: Policy directory path (default: ./policies)
- `APPROVAL_TIMEOUT`: Approval timeout in seconds (default: 300)

## 📡 API Endpoints

### Tool Call Proxy
```bash
POST /tools
Content-Type: application/json

{
  "tool_name": "example_tool",
  "args": {"param": "value"},
  "upstream": "http://api.example.com/endpoint"
}
```

### Approval Management
```bash
# Get pending approvals
GET /pending

# Approve/deny a request
POST /approval/{id}
{
  "approved": true,
  "reason": "Looks safe to proceed"
}
```

### Audit Logs
```bash
# Get audit history
GET /audit?limit=100&offset=0
```

### WebSocket Updates
```bash
# Real-time updates
GET /ws
```

## 🧪 Testing the System

### 1. Test Policy Evaluation
```bash
curl -X POST http://localhost:8080/tools \
  -H "Content-Type: application/json" \
  -d '{
    "tool_name": "test_tool",
    "args": {"param": "value"},
    "upstream": "http://httpbin.org/post"
  }'
```

### 2. Check Audit Logs
```bash
curl http://localhost:8080/audit
```

### 3. Monitor Real-Time Updates
Open the dashboard at `http://localhost:8080` and watch for live updates as requests are processed.

## 🏗️ Development

### UI Development

The React dashboard is located in `internal/server/web/`:

```bash
cd internal/server/web
npm install
npm run dev    # Development server
npm run build  # Production build
```

### Running Tests

```bash
go test ./... -v
```

### Building from Source

```bash
# Build UI assets
cd internal/server/web && npm run build

# Build Go binary
go build -o sidecar ./cmd/sidecar
```

## 🔍 Key Components

### Policy Engine (`internal/policy/`)
- **OPA Integration**: Uses the official OPA Go SDK for policy evaluation
- **File Watching**: Automatically reloads policies when changed
- **Flexible Evaluation**: Supports complex policy rules and conditions

### Approval System (`internal/approval/`)
- **Queue Management**: Thread-safe approval queue with timeouts
- **Real-Time Updates**: WebSocket notifications for approval status changes
- **Audit Integration**: All approval decisions are logged

### Dashboard (`internal/server/web/`)
- **React + Vite**: Modern, fast UI development stack
- **Real-Time Updates**: WebSocket integration for live data
- **Responsive Design**: Works on desktop and mobile devices

## 🚨 Security Considerations

- All tool calls are evaluated against OPA policies before execution
- Audit logs are immutable and stored in SQLite with transaction safety
- WebSocket connections are authenticated and rate-limited
- Sensitive operations can require human approval via policy configuration

## 📊 Monitoring

The system provides comprehensive logging:
- Policy evaluation results
- Approval workflow status
- HTTP request/response details
- WebSocket connection events
- Database operations

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 🆘 Troubleshooting

### Server Won't Start
- Check if port 8080 is available
- Verify Go version (1.21+)
- Ensure `policies/` directory exists with at least one `.rego` file

### UI Not Loading
- Verify web assets are built: `ls web/dist/`
- Check browser console for errors
- Ensure server is running on correct port

### Policy Not Loading
- Check `.rego` file syntax
- Verify policy package name is `policy`
- Check server logs for policy evaluation errors

### Database Issues
- Ensure `db/` directory exists and is writable
- Check SQLite database permissions
- Review audit store initialization logs

# Policy directory
POLICY_DIR=/app/policies
```

## Policies

Policies are rules that determine if a tool call should be allowed. They're written in WASM for performance and security.

### Using Example Policies

We provide ready-to-use policies in `policies/examples/`:

1. **allow_all.wasm** - Allows everything (development only)
2. **business_hours.wasm** - Only allows calls during 9am-5pm
3. **rate_limit.wasm** - Limits to 100 calls per hour

**To use a policy:**
```bash
cp policies/examples/allow_all.wasm policies/
```

The sidecar automatically reloads policies when files change.

### Writing Custom Policies

See `policies/README.md` for a guide on writing your own policies.

## Stopping the Service

```bash
docker-compose down
```

## Troubleshooting

### Service won't start
- **Check Docker is running**: `docker ps`
- **Check ports**: Make sure port 8080 is not in use
- **View logs**: `docker-compose logs`

### All requests are denied
- **Check policies**: Make sure you have at least one `.wasm` file in `./policies/`
- **Copy example policy**: `cp policies/examples/allow_all.wasm policies/`
- **Restart**: `docker-compose restart`

### Can't see audit logs
- **Check database**: `ls -la db/` should show `audit.db`
- **Check permissions**: Database directory must be writable

## Architecture

```
┌─────────┐     ┌──────────────────┐     ┌───────┐
│ AI Agent│────▶│ Governance Sidecar│────▶│ Tools │
└─────────┘     │  • Policy Check   │     └───────┘
                │  • Audit Log      │
                │  • Human Approval │
                └──────────────────┘
                        │
                        ▼
                   ┌─────────┐
                   │ SQLite  │
                   │ Audit DB│
                   └─────────┘
```

## Support

- **Documentation**: See `internal/README.md` for technical details
- **Issues**: Report problems on GitHub
- **Examples**: Check `examples/` directory for integration samples

## Production Checklist

Before deploying to production:

- [ ] Replace `allow_all.wasm` with proper policies
- [ ] Set up database backups (backup `db/audit.db`)
- [ ] Configure `TOOL_UPSTREAM` to your production service
- [ ] Set `LOG_LEVEL=warn` or `LOG_LEVEL=error`
- [ ] Review audit logs regularly
- [ ] Set up monitoring alerts
- [ ] Use HTTPS in front of the sidecar

## License
[LICENSE](https://github.com/dagbolade/AgentGov/blob/main/LICENSE)
