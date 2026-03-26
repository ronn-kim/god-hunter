# God-Hunter API Module — Quick Start Guide

## Installation

Build the project:

```bash
cd /home/ryan-kipruto/god-hunter
go build -o god-hunter ./cmd/main.go
```

## Usage Examples

### 1. Record an API Chain

Record a single request or entire sequence of API calls:

```bash
./god-hunter api record https://api.example.com/v1/users --session myapp
```

**Output:**
```
[+] Recorded request to https://api.example.com/v1/users (Status: 200, Time: 567ms)
[+] Chain ID: chain_ab98a4a83abd83b5
[+] Session ID: myapp
```

The chain is stored in `~/.god-hunter/sessions.db` for later replay and mutation.

### 2. Fuzz Parameters

Send multiple payloads to detect injection vulnerabilities:

```bash
./god-hunter api fuzz https://api.example.com/api/transfer --session myapp
```

Detects anomalies including:
- SQLi patterns (`' OR '1'='1`)
- Command injection (`; whoami`, `$(whoami)`)
- Path traversal (`../../../etc/passwd`)
- XSS payloads (`<script>alert(1)</script>`)
- Template injection (`{{7*7}}`, `#{7*7}`)
- Log4Shell (`${jndi:ldap://...}`)

### 3. Race Condition Testing

Detect TOCTOU, IDOR, and race-based business logic flaws:

```bash
./god-hunter api race https://api.example.com/checkout --concurrent 15 --iterations 50
```

Sends 750 concurrent requests and looks for:
- Inconsistent response codes
- Timing variance anomalies
- State modification conflicts

### 4. Replay with Mutations

Replay a recorded chain with different mutation strategies:

```bash
./god-hunter api replay --session myapp --mutate race
```

Mutation strategies:
- `race` — Send all requests concurrently
- `order` — Reverse request sequence
- `param` — Inject payloads into parameters
- `timing` — Vary delays between requests

### 5. Build Complex API Sequences

Create multi-step sequences with dependencies:

```bash
./god-hunter api chain https://api.example.com/v1 --record
```

Example workflow:
1. Authenticate (POST /auth)
2. Create resource (POST /resource)
3. Modify as admin (PATCH /resource/{id})
4. Replay with race condition to trigger race

## Global Flags

```
--session, -s    Named session ID
--proxy, -p      Upstream proxy (Burp, Caido, etc.)
--jitter, -j     Delay range in ms (default: 800-2400)
--rate, -r       Requests per minute cap (default: 30)
--silent, -q     Suppress output (findings still stored)
--out, -o        Output directory for findings
--format, -f     Output format: json | markdown | burp-xml
```

## Session Persistence

All sessions stored in `~/.god-hunter/sessions.db`:

**Tables:**
- `sessions` — Top-level session metadata
- `chains` — Ordered request sequences
- `requests` — Individual HTTP request/response pairs
- `findings` — Discovered vulnerabilities
- `wordlists_entries` — Dynamically generated fuzzing wordlists
- `iam_graph` — Cloud IAM permission relationships

## Architecture

### Jitter Engine

Requests include randomized delays (800-2400ms by default) to mimic human behavior:

```go
// Every request is delayed before execution
delay := rand.Intn(maxMs - minMs) + minMs
time.Sleep(time.Duration(delay) * time.Millisecond)
```

### Mutation System

Chains can be mutated for advanced testing:

```go
MutateChain(MutationRace, chain)   // Concurrent requests
MutateChain(MutationOrder, chain)  // Reverse sequence
MutateChain(MutationParam, chain)  // Parameter injection
MutateChain(MutationTiming, chain) // Timing variances
```

### Human-Like Headers

Every request includes realistic browser headers:

```
User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) ...
Accept: application/json, text/plain, */*
Accept-Language: en-US,en;q=0.9
Accept-Encoding: gzip, deflate, br
Cache-Control: no-cache
Connection: keep-alive
```

## Troubleshooting

**"Session not found"**
```bash
# Check existing sessions
./god-hunter session:list
```

**Fuzzing too slow?**
```bash
# Adjust rate limiting
./god-hunter api fuzz <url> --rate 100
```

**Large target with many endpoints?**
```bash
# Use silent mode to suppress output, focus on findings
./god-hunter api fuzz <url> --silent --out ./findings
```

## Next Steps

- Integrate with Burp Suite via proxy (`--proxy localhost:8080`)
- Add scope file for authorizedomain boundaries (~/.god-hunter/scope.yaml)
- Export findings to JSON/Markdown for bounty report generation
- Extend with custom wordlists for fuzzing

---

**God-Hunter v1.0** — Professional bug bounty framework for authorized security research.
