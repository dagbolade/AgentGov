# AI Governance Sidecar

A production-ready governance layer for AI agents that enforces policies, maintains audit trails, and enables human oversight—without modifying your AI agent code.

## What Does This Do?

This service sits between your AI agent and the tools it calls, acting as a security checkpoint:

1. **Policy Enforcement**: Automatically allows or blocks tool calls based on your rules
2. **Audit Trail**: Records every decision in an immutable database
3. **Human Approval**: Optionally requires human review for sensitive operations

## Quick Start (3 Steps)

### Step 1: Install Docker

If you don't have Docker installed:
- **Mac**: Download [Docker Desktop for Mac](https://www.docker.com/products/docker-desktop)
- **Windows**: Download [Docker Desktop for Windows](https://www.docker.com/products/docker-desktop)
- **Linux**: Follow [Docker installation guide](https://docs.docker.com/engine/install/)

### Step 2: Start the Service

Open your terminal and run:

```bash
docker-compose up -d
```

That's it! The service is now running.

### Step 3: Verify It's Working

```bash
curl http://localhost:8080/health
```

You should see: `{"status":"healthy"}`

## How to Use It

### Point Your AI Agent to the Sidecar

Instead of calling tools directly, your AI agent sends requests to:
```
http://localhost:8080/tool/call
```

**Example Request:**
```bash
curl -X POST http://localhost:8080/tool/call \
  -H "Content-Type: application/json" \
  -d '{
    "tool_name": "send_email",
    "args": {
      "to": "user@example.com",
      "subject": "Hello",
      "body": "Test message"
    }
  }'
```

### View Audit Log

See all decisions made:
```bash
curl http://localhost:8080/audit
```

## Configuration

Edit the `.env` file to customize:

```bash
# Where tool calls are forwarded to
TOOL_UPSTREAM=http://your-tool-service:9000

# Logging level (debug, info, warn, error)
LOG_LEVEL=info

# Database location
DB_PATH=/app/db/audit.db

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
https://github.com/dagbolade/AgentGov/blob/main/LICENSE