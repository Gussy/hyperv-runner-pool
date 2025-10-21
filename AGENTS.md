# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a **Hyper-V Runner Pool** - a production-ready orchestrator for running GitHub Actions workflows in ephemeral Hyper-V VMs on Windows. Each job runs in a fresh, isolated VM that is automatically destroyed and recreated after completion. The project enables cross-platform development (develop on macOS, deploy to Windows) through a mock VM manager.

## Core Architecture

### Single-File Design
The entire orchestrator is implemented in `main.go` (~600 lines). This is intentional - the codebase uses a **monolithic single-file architecture** to keep deployment simple and reduce complexity for a focused tool.

### Key Components

1. **VMManager Interface**: Abstraction layer allowing platform-specific implementations
   - `HyperVManager`: Real Hyper-V operations on Windows (PowerShell commands)
   - `MockVMManager`: Simulated VMs for macOS development and testing
   - Key methods: `CreateVM`, `DestroyVM`, `GetVMState`, `InjectConfig`

2. **Orchestrator**: Manages the VM pool lifecycle
   - Maintains a fixed-size pool of `VMSlot` structs
   - Handles VM creation, registration with GitHub, and recreation after jobs
   - Uses goroutines for concurrent VM operations

3. **VM Lifecycle States**:
   - `empty` → `creating` → `ready` → `running` → `destroying` → `empty`
   - State transitions are mutex-protected at the slot level

4. **VM State Monitoring**: Serverless polling-based job completion detection
   - Each VM has a dedicated monitoring goroutine (`MonitorVMState`)
   - Polls VM state every 10 seconds via Hyper-V
   - When VM state = "Off", triggers automatic recreation
   - No external network access required

### VM Template Architecture

VMs are created by copying a VHDX template file. The template contains:
- Windows Server 2022 (via Packer)
- GitHub Actions runner pre-installed
- Startup script that reads config from injected file (`C:\runner-config.json`)

**Config Injection Process**:
1. Orchestrator generates GitHub runner token
2. Mounts VHDX, writes `runner-config.json`, unmounts
3. Creates and starts VM
4. VM boot script reads config from local file (no network call)
5. VM runs single job (`--once` flag) then shuts down
6. Orchestrator detects shutdown and recreates VM

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

The codebase uses **Go unit tests** (`main_test.go`) to test individual components in isolation:
- Mock VM manager operations (CreateVM, DestroyVM, GetVMState, InjectConfig)
- Orchestrator lifecycle management
- RunnerConfig JSON serialization
- Concurrent VM operations
- State transitions

Tests run fast and require no external dependencies.

## Configuration

Environment variables are loaded directly (no config files). Required in production:
- `GITHUB_PAT`: Personal Access Token (repo or admin:org scope)
- `GITHUB_ORG`: GitHub organization name
- `GITHUB_REPO`: Repository name (empty for org-level runners)

Development-specific:
- `USE_MOCK=true`: Enable mock VM manager (automatically adds dummy credentials)
- `LOG_LEVEL`: debug|info|warn|error (default: info)
- `LOG_FORMAT`: text|json (default: text)

Optional (with defaults):
- `VM_TEMPLATE_PATH`: Path to VHDX template (default: `./vms/templates/runner-template.vhdx`)
- `VM_STORAGE_PATH`: VM storage directory (default: `./vms/storage`)

Pool size is hardcoded to 4 in `main.go`. Change the literal value to adjust.

## Production Deployment

1. **Build VM template** (Windows only, see README Phase 2):
   ```powershell
   cd packer
   packer build windows-runner.pkr.hcl
   Copy-Item ".\output-windows-runner\Virtual Hard Disks\*.vhdx" "..\vms\templates\runner-template.vhdx"
   ```

2. **Run orchestrator**:
   ```powershell
   .\start.ps1  # Loads .env and starts binary
   ```

The orchestrator is fully self-contained and requires no external network access. It monitors VM state locally via Hyper-V and recreates VMs automatically when they shut down after job completion.

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
- Mock mode bypasses API with fake tokens

### VHDX Config Injection
- `InjectConfig` method mounts VHDX before VM starts
- Writes `runner-config.json` containing token, org, repo, labels
- Unmounts VHDX (critical - VM won't start with mounted disk)
- Config file is read by VM startup script on boot

### VM Recreation Flow
When a VM completes a job:
1. VM runs GitHub Actions runner with `--once` flag (single job)
2. Job completes, runner exits
3. Startup script executes `Stop-Computer -Force`
4. Monitoring goroutine (`MonitorVMState`) polls every 10s
5. Detects VM state = "Off"
6. Triggers `RecreateVM` asynchronously
7. VM transitions: running → destroying → (destroy) → creating → (create) → ready
8. New GitHub token is generated and injected during creation
9. Pool slot is reused with fresh VM

### Concurrent Operations
- VM pool initialization uses goroutines with WaitGroup
- Each VMSlot has its own mutex (fine-grained locking)
- Each VM has a dedicated monitoring goroutine (started after creation)
- Orchestrator mutex protects pool-level operations only

## Common Pitfalls

1. **Don't modify PowerShell commands without Windows testing**: Mock manager doesn't catch syntax errors
2. **Pool size changes**: Hardcoded in main.go, affects resource usage significantly (each VM = 2GB RAM)
3. **Path handling**: Windows paths use backslashes; Go escapes them (`\\`) in format strings
4. **VHDX mounting**: Must unmount before starting VM, or VM creation will fail
5. **Polling interval**: 10-second polling means ~10s delay before recreation starts (acceptable trade-off for simplicity)
6. **Hermit package manager**: Project uses Hermit (`bin/hermit.hcl`) for Go, Task, GoReleaser - use `bin/` prefix for commands

## Notes for Future Development

- The single-file architecture is intentional; resist splitting unless complexity truly demands it
- If adding new VM states, update `VMState` enum and monitoring logic
- The serverless polling architecture eliminates need for external network access or webhook configuration
- GitHub API rate limits aren't handled; consider implementing backoff if scaling beyond ~10 VMs
- Polling interval can be tuned (currently 10s) - lower = faster recreation but more CPU/API calls
