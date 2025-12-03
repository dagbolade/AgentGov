# Tools

This directory contains helper scripts used during demo and local development.

Files
- `simulate_demo.sh` — conservative end-to-end demo script. Logs in, sends a sequence of tool calls, polls for a human approval, approves it, and prints the audit tail.
- `demo_variants.sh` — runs a short set of variant payloads (benign, sensitive, secret, and simulated EHR access), and demonstrates deterministic enqueue+approve via the server's demo endpoint.

Demo endpoint

The server exposes a temporary demo endpoint useful for deterministic demos when the WASM policies don't produce human_required decisions reliably:

- `POST /simulate/enqueue` (protected): creates a pending approval using the server's approval queue. Request body JSON:
  - `tool_name` (optional) — default `demo_tool`
  - `args` (optional) — JSON args forwarded into the approval request
  - `reason` (optional) — default `demo-enqueue`

Example (requires a valid JWT token):

```bash
TOKEN=$(curl -sS -X POST http://localhost:8080/login -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"admin"}' | jq -r .token)

curl -sS -X POST http://localhost:8080/simulate/enqueue \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"tool_name":"demo_tool","args":{"action":"test"}}' | jq .
```

Running the scripts

Both scripts assume the sidecar API is available at `http://localhost:8080` (the `REACT_APP_API_URL` set in `docker-compose.yml`).

To run the conservative demo:

```bash
cd /workspaces/AgentGov/ai-governance-sidecar
chmod +x tools/simulate_demo.sh
./tools/simulate_demo.sh
```

To run the variant payloads (this sends multiple payloads and demonstrates the demo enqueue/approve flow):

```bash
chmod +x tools/demo_variants.sh
./tools/demo_variants.sh
```

Hospital simulation
-------------------

A simple script is provided to enqueue a set of hospital-style approvals you can review and approve in the UI:

```bash
chmod +x tools/hospital_sim.sh
./tools/hospital_sim.sh
```

The script logs in as `admin@example.com` / `admin`, enqueues four demo approvals (EHR access, prescription, billing, lab fetch), and prints the pending approvals. Open the UI at `http://localhost:3000` to approve them.

Notes
- The demo endpoint is intended for demo/testing only and is protected by the service auth when `REQUIRE_AUTH=true`.
- If you run inside Docker Compose, the audit DB is mounted at `./db` on the host and at `/app/db` inside the container.
