# God-Hunter API Module — Command Reference

## Project Structure

```
/home/ryan-kipruto/god-hunter/
├── cmd/
│   └── main.go                 # Entry point
├── internal/
│   ├── api/
│   │   └── chains.go          # Chain recording, replay, fuzzing, race detection
│   ├── cmd/
│   │   ├── root.go            # Cobra CLI framework
│   │   ├── api.go             # API subcommands
│   │   └── init.go            # Initialization
│   ├── db/
│   │   └── store.go           # SQLite schema and CRUD
│   ├── http/
│   │   └── client.go          # HTTP client with jitter engine
│   └── session/
│       └── session.go         # Session management
├── tools/
│   └── dbstats.go             # Database statistics utility
├── go.mod                     # Go module definition
├── README.md                  # Main documentation
├── QUICKSTART.md              # Usage examples
├── API_MODULE.md              # Architecture details
└── IMPLEMENTATION_SUMMARY.md  # Project completion report
```

## Build & Run

### Build
```bash
cd /home/ryan-kipruto/god-hunter
go build -o god-hunter ./cmd/main.go
```

### Verify Build
```bash
./god-hunter --help
./god-hunter api --help
```

### Database Tool
```bash
go run tools/dbstats.go  # View session statistics
```

## CLI Commands

### API Record — Capture API Chains
```bash
./god-hunter api record <URL> [FLAGS]

Examples:
  ./god-hunter api record https://api.example.com --session myapp
  ./god-hunter api record https://example.com:8443/api/v1 --proxy localhost:8080
```

**Flags:**
- `--session, -s` — Session name
- `--proxy, -p` — Upstream proxy (Burp, Caido)
- `--silent, -q` — Suppress output

---

### API Fuzz — Detect Injection Vulnerabilities
```bash
./god-hunter api fuzz <URL> [FLAGS]

Examples:
  ./god-hunter api fuzz https://api.example.com/transfer --session myapp
  ./god-hunter api fuzz https://api.example.com --session myapp --silent
```

**Flags:**
- `--session, -s` — Session ID
- `--wordlist` — Custom payload file
- `--param` — Target specific parameter
- `--silent, -q` — Suppress output

---

### API Race — Detect Race Conditions
```bash
./god-hunter api race <URL> [FLAGS]

Examples:
  ./god-hunter api race https://api.example.com/checkout --concurrent 10
  ./god-hunter api race https://api.example.com --concurrent 15 --iterations 50
```

**Flags:**
- `--concurrent` — Parallel requests (default: 10)
- `--iterations` — Test iterations (default: 100)
- `--session, -s` — Session ID

---

### API Chain — Build Multi-Step Sequences
```bash
./god-hunter api chain <URL> [FLAGS]

Examples:
  ./god-hunter api chain https://api.example.com/v1 --session myapp
  ./god-hunter api chain https://api.example.com/v1 --record --session myapp
```

**Flags:**
- `--session, -s` — Session ID
- `--record` — Auto-record the chain

---

### API Replay — Replay Recorded Chains
```bash
./god-hunter api replay [FLAGS]

Examples:
  ./god-hunter api replay --session myapp
  ./god-hunter api replay --session myapp --mutate race
  ./god-hunter api replay --session myapp --mutate order
```

**Flags:**
- `--session, -s` — Session to replay (required)
- `--mutate` — Mutation strategy: race | order | param | timing

---

## Global Flags

```
--session, -s <name>      Session ID (auto-generated if not provided)
--proxy, -p <url>         Upstream proxy for MITM inspection
--jitter, -j <range>      Delay range in ms (default: 800-2400)
--rate, -r <num>          Requests per minute (default: 30)
--silent, -q              Suppress non-critical output
--out, -o <path>          Output directory (default: ./findings)
--format, -f <fmt>        Output format: json | markdown | burp-xml
```

---

## Common Workflows

### Workflow 1: Basic Reconnaissance
```bash
# Record initial request
./god-hunter api record https://api.example.com --session recon_001

# Fuzz parameters to identify injection points
./god-hunter api fuzz https://api.example.com/api/search --session recon_001

# Export findings
./god-hunter session:export recon_001 --format markdown --out ./report.md
```

### Workflow 2: Race Condition Testing
```bash
# Record a transaction endpoint
./god-hunter api record https://api.example.com/transfer --session race_001

# Test with concurrent requests
./god-hunter api race https://api.example.com/transfer --concurrent 20 --iterations 50

# Analyze findings in database
go run tools/dbstats.go
```

### Workflow 3: Complex Chain Testing
```bash
# Build multi-step API sequence
./god-hunter api chain https://api.example.com/v1 --session chain_test --record

# Replay with race mutation to find TOCTOU bugs
./god-hunter api replay --session chain_test --mutate race

# Replay with parameter mutation
./god-hunter api replay --session chain_test --mutate param
```

### Workflow 4: Proxy Integration
```bash
# Use with Burp Suite on localhost:8080
./god-hunter api record https://api.example.com --proxy localhost:8080 --session burp_001

# Manually modify requests in Burp, then:
./god-hunter api replay --session burp_001
```

---

## Troubleshooting

### Issue: "Session not found"
```bash
# Check available sessions
go run tools/dbstats.go
```

### Issue: Too many requests / Rate limiting
```bash
# Reduce rate or increase jitter
./god-hunter api fuzz <url> --rate 15 --jitter 2000-5000
```

### Issue: SQLite locked (concurrent writes)
```bash
# God-Hunter uses transaction safety; wait and retry
# Or review database at: ~/.god-hunter/sessions.db
```

### Issue: Proxy not working
```bash
# Verify proxy URL format: http://localhost:8080 (not https)
./god-hunter api record <url> --proxy http://localhost:8080
```

---

## Database Access

### Location
```bash
~/.god-hunter/sessions.db
```

### Quick Stats
```bash
go run tools/dbstats.go
```

### Manual Access (requires sqlite3)
```bash
sqlite3 ~/.god-hunter/sessions.db

# Inside sqlite3:
SELECT * FROM sessions;
SELECT * FROM chains;
SELECT * FROM findings;
.schema
.quit
```

---

## Performance Tuning

| Parameter | Default | Range | Effect |
|-----------|---------|-------|--------|
| `--jitter` | 800-2400ms | 0-10000ms | Delay between requests |
| `--rate` | 30 req/min | 1-1000 req/min | Request throttling |
| `--concurrent` | 10 workers | 1-100 workers | Race detection parallelism |
| `--iterations` | 100 times | 1-1000 times | Repeat count for pattern detection |

---

## Output Formats

### JSON Format (Default)
```bash
./god-hunter api fuzz <url> --format json --out ./findings.json
```

### Markdown Format
```bash
./god-hunter api fuzz <url> --format markdown --out ./report.md
```

### Burp XML Format
```bash
./god-hunter api fuzz <url> --format burp-xml --out ./burp_import.xml
```

---

## Key Features Summary

| Feature | Command | Status |
|---------|---------|--------|
| Chain Recording | `api record` | ✅ |
| Chain Replay | `api replay` | ✅ |
| Parameter Fuzzing | `api fuzz` | ✅ |
| Race Detection | `api race` | ✅ |
| Mutation Testing | `api replay --mutate` | ✅ |
| Session Persistence | Database auto-save | ✅ |
| Jitter Engine | Built-in | ✅ |
| Proxy Support | `--proxy` flag | ✅ |
| User-Agent Rotation | Automatic | ✅ |
| Finding Storage | SQLite | ✅ |

---

## Support & Documentation

- **README.md** — Overview and main documentation
- **QUICKSTART.md** — Quick reference and examples  
- **API_MODULE.md** — Architecture and technical details
- **IMPLEMENTATION_SUMMARY.md** — Completion report and metrics

---

**God-Hunter v1.0** — Professional bug bounty framework for authorized security researchers
