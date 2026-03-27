# God-Hunter: Professional Bug Bounty Framework

## Overview

God-Hunter is a modular, high-performance bug bounty suite written in Go, designed for professional security researchers operating within authorized scope. It targets vulnerability classes consistently missed by public scanners: Business Logic flaws, IAM Trust Relationship abuse, and Mobile Deep Link exploitation.

## Architecture

### Core Principles

1. **State-Machine Over Stateless** — Every operation exists within a Session, stored in SQLite with persistent cross-module data flow
2. **Contextual Pattern Recognition** — AST-based static analysis and LLM-assisted review, not keyword matching
3. **Silent Execution** — Built-in Jitter Engine with realistic pacing and no tool-identifiable signatures
4. **Cross-Module Data Flow** — Automatic seeding of downstream modules from reconnaissance outputs

## CLI Syntax

```bash
god-hunter <module>:<action> [target] [flags]
```

### Examples

```bash
god-hunter api:record https://api.example.com
god-hunter api:replay --session sess_001 --mutate race
god-hunter api:fuzz https://api.example.com/endpoint --wordlist payloads.txt
god-hunter api:chain https://api.example.com --record
god-hunter api:race https://api.example.com --concurrent 20
```

## Modules

### API Module

Records, replays, and fuzzes API call sequences with race condition detection.

**Actions:**
- `record` — Intercept and store API call sequences as Chains
- `replay` — Replay a Chain with optional mutation (race, order, param, timing)
- `fuzz` — Parameter fuzzing with contextual wordlists
- `chain` — Multi-step API sequence builder
- `race` — Concurrent request engine for race condition PoC

**Flags:**
- `--session` — Session ID (default: auto-generated)
- `--proxy` — Upstream proxy for interception
- `--jitter` — Request jitter range (default: 800-2400ms)
- `--rate` — Requests per minute cap (default: 30)
- `--mutate` — Mutation strategy for replay
- `--concurrent` — Number of concurrent requests for race detection
- `--iterations` — Number of test iterations
- `--wordlist` — Custom fuzzing wordlist
- `--param` — Target specific parameter

## Installation

### Prerequisites

- Go 1.21 or higher
- SQLite3 support (included via modernc.org/sqlite)
- Optional: Burp Suite or similar proxy for interception

### Build

```bash
go build -o god-hunter ./cmd/main.go
```

### Run Tests

```bash
go test ./...
```

## Usage Examples

### Basic API Reconnaissance

```bash
god-hunter api:record https://api.example.com/v1/users
```

This creates a session, records the initial request, and stores it in the SQLite database.

### Fuzzing an Endpoint

```bash
god-hunter api:fuzz https://api.example.com/api/transfer --session myapp --silent
```

Sends multiple payloads to detect parameter anomalies or injection points.

### Race Condition Testing

```bash
god-hunter api:race https://api.example.com/checkout --concurrent 15 --iterations 50
```

Sends concurrent requests to detect race conditions in state-modifying endpoints.

### Chain Replay with Mutations

```bash
god-hunter api:replay --session myapp --mutate race
```

Replays a previously recorded chain, optionally with mutations (race, timing variance, parameter injection).

## Session Management

Sessions persist across runs and are stored in `~/.god-hunter/sessions.db`.

### View Sessions

```bash
god-hunter session:list
```

### Resume a Session

```bash
god-hunter session:resume sess_001
```

### Export Findings

```bash
god-hunter session:export sess_001 --format json --out ./report.json
```

## Output Formats

- **JSON** — Machine-readable findings
- **Markdown** — Human-readable report with context and PoC
- **Burp XML** — Import findings into Burp Suite

## Scope Enforcement

Every session requires a scope file to enforce authorized boundaries:

```yaml
allowed_domains:
  - api.example.com
  - example.com
allowed_ips:
  - 10.0.0.0/8
excluded_paths:
  - /admin
  - /internal
program_name: "Example Bug Bounty"
platform: "hackerone"
```

## Findings Database

All findings are stored in SQLite with:
- Vulnerability type (RACE_CONDITION, TOCTOU, IDOR, etc.)
- Severity level (CRITICAL, HIGH, MEDIUM, LOW)
- Endpoint and parameter details
- Proof-of-concept payloads
- Evidence and reproducibility notes

## Evasion Features

- **Jitter Engine** — Randomized delays (800-2400ms by default) and realistic pacing
- **User-Agent Rotation** — Rotates browser fingerprints from curated set
- **TLS Fingerprint Normalization** — Matches Chrome/Firefox profiles
- **Header Sanitization** — No X-Scanner or custom tool headers
- **Adaptive Rate Limiting** — Backs off on 429/503 responses

## Privacy & Compliance

- No telemetry or beacon functionality
- All operations local to your machine
- Session data encrypted at rest (optional)
- Session scope enforcement mandatory

## Contributing

God-Hunter is a private security tool. Contributions are accepted for bug fixes and feature enhancements within the authorized scope of use.

## License

Proprietary — For use in authorized security research only.

---

**God-Hunter v1.0** — *Find what others miss.*
