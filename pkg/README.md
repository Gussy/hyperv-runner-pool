# Package Structure

This directory contains the core packages for the hyperv-runner-pool application.

## Packages

### `config/`
Configuration management for the application.
- Loads and validates YAML configuration files
- Provides configuration structs for GitHub, Runners, Hyper-V, and Debug settings
- Handles default values and validation

### `github/`
GitHub API client for runner token management.
- Handles GitHub App authentication
- Generates runner registration tokens
- Supports both organization and repository-level runners
- Mock mode for development and testing

### `logger/`
Logging setup and configuration.
- Configures structured logging with slog
- Supports multiple log levels (debug, info, warn, error)
- Supports multiple formats (text, json)

### `orchestrator/`
VM pool orchestration and lifecycle management.
- Manages the pool of ephemeral VMs
- Coordinates VM creation, monitoring, and recreation
- Handles graceful shutdown and cleanup
- Monitors VM state and triggers recreation after job completion

### `vmmanager/`
VM management interface and implementations.
- Defines the `VMManager` interface for platform abstraction
- **Hyper-V Implementation**: Windows Hyper-V VM operations
  - Creates differencing disks from templates
  - Injects runner configuration via VHDX mounting
  - Executes scripts via PowerShell Direct
  - Manages VM lifecycle (create, start, stop, destroy)
- **Mock Implementation**: Testing and cross-platform development
- **VM State Management**: Tracks VM lifecycle states
- **Runner Configuration**: Structures for runner registration

## Usage

These packages are imported by the main CLI application in `cmd/hyperv-runner-pool/`.

Example:
```go
import (
    "hyperv-runner-pool/pkg/config"
    "hyperv-runner-pool/pkg/github"
    "hyperv-runner-pool/pkg/logger"
    "hyperv-runner-pool/pkg/orchestrator"
    "hyperv-runner-pool/pkg/vmmanager"
)
```

## Testing

Run all package tests:
```bash
go test ./pkg/...
```

Run tests for a specific package:
```bash
go test ./pkg/vmmanager
go test ./pkg/orchestrator
```

Run tests with verbose output:
```bash
go test -v ./pkg/...
```
