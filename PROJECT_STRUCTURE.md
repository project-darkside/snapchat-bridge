# Project Structure

```
snapchat-bridge/
├── README.md                    # Main project documentation
├── LICENSE                      # MIT License
├── go.mod                       # Go module definition
├── go.sum                       # Go dependencies
├── .gitignore                   # Git ignore rules
│
├── Core Implementation
├── main.go                      # Application entry point
├── bridge.go                    # Matrix bridge implementation
├── connector.go                 # Bridge connector
├── snapchat.go                  # Snapchat API client
├── picaboooo.go                 # Picaboooo service client (unused)
├── crypto.go                    # AES encryption for media
├── message.go                   # Message types and handlers
├── login.go                     # Authentication flow
├── database.go                  # SQLite persistence
├── config.go                    # Configuration management
│
├── Configuration
├── config.example.yaml          # Example configuration (commit this)
│
├── Tests
├── *_test.go                    # Unit tests (next to source files)
├── tests/                       # Additional test files
│   └── crypto_test.go
│
├── Scripts (for development/testing)
├── scripts/
│   ├── test_picaboo_login.go   # Standalone login test
│   ├── test_login.go            # Alternative test
│   ├── test_snapchat_login.sh  # Shell script test
│   └── quick_test.sh            # Quick test helper
│
├── Documentation
├── docs/
│   ├── QUICKSTART.md            # Quick start guide
│   ├── DEPLOYMENT.md            # Deployment instructions
│   ├── TESTING.md               # Testing guide
│   ├── IMPLEMENTATION_SUMMARY.md # Implementation details
│   ├── README.md                # Docs overview
│   │
│   ├── development/             # Development docs
│   │   ├── HANDOFF.md           # Project status and handoff
│   │   ├── TODO.md              # Next steps
│   │   ├── FINDINGS.md          # Investigation notes
│   │   ├── PIVOT.md             # Development decisions
│   │   └── TRAFFIC_ANALYSIS_GUIDE.md # mitmproxy guide
│   │
│   └── reference/               # Reference materials
│       ├── SnapchatSDK.txt      # PHP SDK reference
│       ├── go.txt               # mautrix-go reference
│       └── twilio-bridge.txt    # Bridge example reference
```

## Files NOT in Git (via .gitignore)

**Built binaries:**
- `snapchat-bridge` (main binary)
- `test_picaboo_login` (test binary)
- `*.test` (Go test binaries)

**Configuration & Data:**
- `config.yaml` (personal config with credentials)
- `registration.yaml` (generated registration)
- `*.db`, `*.sqlite` (database files)
- `*.log` (log files)

**Development temporary:**
- `QUICK_START.txt` (personal quick reference)
- `COMMIT_MESSAGE.md` (commit helper)
- `.claude/` (AI assistant cache)
- `/tmp/` (temporary files)

**IDE & System:**
- `.vscode/`, `.idea/` (IDE settings)
- `.DS_Store` (macOS)
- `*~`, `*.swp` (editor temp files)

## What Goes on GitHub

✅ **Commit these:**
- All `.go` source files
- `go.mod` and `go.sum`
- `config.example.yaml` (example only)
- All documentation in `docs/`
- Test scripts in `scripts/`
- `LICENSE`, `README.md`, `.gitignore`

❌ **Don't commit:**
- Compiled binaries
- Personal config files (`config.yaml`)
- Database files
- Log files
- IDE settings (optional to commit)
- Temporary development files

## Directory Purpose

**Root:**
- Main project files, documentation, and license

**docs/**
- All user-facing and developer documentation
- Organized into subdirectories for clarity

**docs/development/**
- Internal development documentation
- Status updates, TODOs, findings
- For contributors and developers

**docs/reference/**
- External reference materials
- SDK documentation, examples
- Not actively maintained, just reference

**scripts/**
- Helper scripts for testing and development
- Not part of main application
- Useful for contributors

**tests/**
- Additional test files that don't follow Go conventions
- Integration test resources

## For Contributors

When working on the project:
1. Keep source files in root
2. Put tests next to source (Go convention: `file.go` → `file_test.go`)
3. Document in `docs/`
4. Helper scripts go in `scripts/`
5. Update this file if structure changes
