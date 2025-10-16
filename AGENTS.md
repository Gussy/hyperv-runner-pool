# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a **Hyper-V Runner Pool** - a production-ready orchestrator for running GitHub Actions workflows in ephemeral Hyper-V VMs on Windows. Each job runs in a fresh, isolated VM that is automatically destroyed and recreated after completion. The project enables cross-platform development (develop on macOS, deploy to Windows) through a mock VM manager.

## Core Architecture

### Single-File Design
The entire orchestrator is implemented in `main.go` (~700 lines). This is intentional - the codebase uses a **monolithic single-file architecture** to keep deployment simple and reduce complexity for a focused tool.

### Key Components

1. **VMManager Interface** (lines 80-84): Abstraction layer allowing platform-specific implementations
   - `HyperVManager`: Real Hyper-V operations on Windows (PowerShell commands)
   - `MockVMManager`: Simulated VMs for macOS development and testing

2. **Orchestrator** (lines 229-403): Manages the VM pool lifecycle
   - Maintains a fixed-size pool of `VMSlot` structs
   - Handles VM creation, registration with GitHub, and recreation after jobs
   - Uses goroutines for concurrent VM operations

3. **VM Lifecycle States** (lines 54-63):
   - `empty` → `creating` → `ready` → `running` → `destroying` → `empty`
   - State transitions are mutex-protected at the slot level

4. **HTTP API** (lines 409-543): REST endpoints for GitHub webhooks and VM coordination
   - `/webhook`: Receives GitHub webhook events (HMAC-SHA256 verified)
   - `/api/runner-config/{vmName}`: VMs fetch registration tokens on boot
   - `/api/runner-complete/{vmName}`: VMs notify job completion (triggers async recreation)
   - `/health`: Health check with pool status

### VM Template Architecture

VMs are created by copying a VHDX template file. The template contains:
- Windows Server 2022 (via Packer)
- GitHub Actions runner pre-installed
- Startup script that auto-registers with orchestrator

Two template options exist (see `packer/` directory):
- **Minimal** (`windows-runner.pkr.hcl`): 30GB, ~45min build, basic tools
- **Enhanced** (`windows-runner-enhanced.pkr.hcl`): 75GB, ~3hr build, GitHub-compatible toolset

## Development Commands

### Building
```bash
# Build Windows binary (from macOS)
task build
# or: GOOS=windows GOARCH=amd64 go build -o hyperv-runner-pool.exe

# Build for local OS (testing)
task build-local

# Create release snapshot (no publish)
task release-snapshot
```

### Testing
```bash
# Run Go unit tests
task test
# or: go test -v ./...

# Run API tests (Hurl-based, auto-starts mock server)
task test-api

# Run all tests
task test-all

# Run with mock VMs (macOS development)
task run
# or: USE_MOCK=true go run main.go
```

### Code Quality
```bash
task fmt        # Format code
task lint       # Run go vet
task deps       # Download and tidy dependencies
```

## Testing Strategy

The codebase uses **two complementary testing approaches**:

1. **Go Unit Tests** (`main_test.go`): Test individual components in isolation
   - Mock VM manager operations
   - HTTP handler behavior with httptest
   - Webhook signature verification
   - Concurrent operations

2. **API Integration Tests** (`tests/*.hurl`): Test the running server via HTTP
   - Real HTTP requests against mock server
   - Sequential test execution (01-health → 02-webhook → 03-runner-config → 04-runner-complete)
   - Automatically managed server lifecycle

When writing tests, **prefer unit tests for logic** and **use Hurl tests for API contract verification**.

## Configuration

Environment variables are loaded directly (no config files). Required in production:
- `GITHUB_PAT`: Personal Access Token (repo or admin:org scope)
- `GITHUB_ORG`: GitHub organization name
- `GITHUB_REPO`: Repository name (empty for org-level runners)
- `WEBHOOK_SECRET`: HMAC secret for webhook verification

Development-specific:
- `USE_MOCK=true`: Enable mock VM manager (automatically adds dummy credentials)
- `LOG_LEVEL`: debug|info|warn|error (default: info)
- `LOG_FORMAT`: text|json (default: text)

Optional (with defaults):
- `PORT`: HTTP server port (default: 8080)
- `VM_TEMPLATE_PATH`: Path to VHDX template (default: `./vms/templates/runner-template.vhdx`)
- `VM_STORAGE_PATH`: VM storage directory (default: `./vms/storage`)
- `ORCHESTRATOR_IP`: IP for VMs to connect (default: localhost)

Pool size is hardcoded to 4 in `main.go` line 634. Change the literal value to adjust.

## Production Deployment

1. **Build VM template** (Windows only, see README Phase 2):
   ```powershell
   cd packer
   packer build windows-runner.pkr.hcl
   Copy-Item ".\output-windows-runner\Virtual Hard Disks\*.vhdx" "..\vms\templates\runner-template.vhdx"
   ```

2. **Configure GitHub webhook**: Repository/Org Settings → Webhooks
   - URL: `https://your-machine.tail-scale.ts.net/webhook`
   - Events: "Workflow jobs" only
   - Secret: matches `WEBHOOK_SECRET`

3. **Run orchestrator**:
   ```powershell
   .\start.ps1  # Loads .env and starts binary
   ```

## Release Process

Uses **GoReleaser** with GitHub Actions:
- Tag version: `git tag -a v1.0.0 -m "Release v1.0.0" && git push origin v1.0.0`
- GitHub Action automatically builds and creates release
- Manual build: `goreleaser build --snapshot --clean`

Release artifacts include binary, config templates, Packer files, and checksums.

## Code Style Conventions

- **Structured logging**: Use `slog` with structured fields: `logger.Info("message", "key", value)`
- **Error wrapping**: Always use `fmt.Errorf("context: %w", err)` for error chains
- **Mutex discipline**: Lock at VMSlot level, not orchestrator level (fine-grained concurrency)
- **Async patterns**: Long operations (RecreateVM) run in goroutines; handlers return immediately
- **No external config**: Configuration is environment-only (12-factor style)

## Critical Implementation Details

### GitHub API Integration
- Runner token generation hits `/repos/{org}/{repo}/actions/runners/registration-token` (repo) or `/orgs/{org}/actions/runners/registration-token` (org)
- Tokens expire; generated fresh for each VM creation
- Mock mode bypasses API with fake tokens (line 319)

### Webhook Verification
The `verifySignature` function (lines 550-564) implements GitHub's HMAC-SHA256 signature verification. This is **security-critical** - do not modify without understanding the GitHub webhook signature spec.

### VM Recreation Flow
When a VM completes a job (POST to `/api/runner-complete`):
1. Handler immediately returns 200 OK
2. Goroutine starts `RecreateVM` asynchronously
3. VM transitions: running → destroying → (destroy) → creating → (create) → ready
4. New GitHub token is generated during creation
5. Pool slot is reused with new VM

### Concurrent Operations
- VM pool initialization uses goroutines with WaitGroup (lines 252-284)
- Each VMSlot has its own mutex (fine-grained locking)
- Orchestrator mutex protects pool-level operations only

## Common Pitfalls

1. **Don't modify PowerShell commands without Windows testing**: Mock manager doesn't catch syntax errors
2. **Pool size changes**: Hardcoded at line 634, affects resource usage significantly (each VM = 2GB RAM)
3. **Path handling**: Windows paths use backslashes; Go escapes them (`\\`) in format strings
4. **Testing webhook signatures**: Must use valid HMAC; test helpers in `main_test.go` show correct generation
5. **Hermit package manager**: Project uses Hermit (`bin/hermit.hcl`) for Go, Task, GoReleaser, Hurl - use `bin/` prefix for commands

## Notes for Future Development

- The single-file architecture is intentional; resist splitting unless complexity truly demands it
- If adding new VM states, update `VMState` enum and health endpoint logic
- New API endpoints should follow the pattern: handler → orchestrator method → VM manager operation
- GitHub API rate limits aren't handled; consider implementing backoff if scaling beyond ~10 VMs
