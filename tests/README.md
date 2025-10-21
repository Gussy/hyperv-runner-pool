# Tests

This directory previously contained Hurl-based API tests for HTTP endpoints. With the migration to a serverless polling architecture, those tests have been removed.

## Current Testing Approach

The project now uses **Go unit tests** exclusively, located in `main_test.go` at the repository root.

## Running Tests

### Option 1: Using Task (Recommended)

```bash
# Run all unit tests
task test

# Run all tests (same as above)
task test-all

# Run quick tests only
task test-short
```

### Option 2: Manual

```bash
# Run all tests with verbose output
go test -v ./...

# Run tests with coverage
go test -v -cover ./...

# Run specific test
go test -v -run TestMockVMManager_CreateVM
```

## Test Coverage

### Mock VM Manager
- ✅ CreateVM - Simulates VM creation
- ✅ DestroyVM - Simulates VM destruction
- ✅ GetVMState - Returns VM power state
- ✅ InjectConfig - Simulates config injection into VHDX
- ✅ RunPowerShell - Mocks PowerShell command execution

### Orchestrator
- ✅ NewOrchestrator - Creates orchestrator instance
- ✅ RecreateVM - VM recreation flow
- ✅ VMSlot state transitions - Mutex-protected state changes
- ✅ Pool initialization - Concurrent VM creation

### Runner Configuration
- ✅ JSON serialization/deserialization
- ✅ Config structure validation

### Concurrent Operations
- ✅ Concurrent VM creation (10 VMs in parallel)
- ✅ Concurrent VM destruction
- ✅ Thread safety verification

### Utility Functions
- ✅ getEnvOrDefault - Environment variable handling

## Test Organization

Tests are organized into logical sections:

1. **Mock VM Manager Tests** - Test the mock implementation used for development
2. **Orchestrator Tests** - Test pool management and VM lifecycle
3. **RunnerConfig Tests** - Test configuration serialization
4. **Utility Function Tests** - Test helper functions
5. **Concurrent Operations Tests** - Test thread safety

## Development Workflow

```bash
# 1. Make code changes
vim main.go

# 2. Run tests
task test

# 3. If tests pass, run with mock VMs
task run

# 4. Build for production
task build
```

## Notes

- All tests run against the mock VM manager (no actual VMs required)
- Tests are fast (~10 seconds total)
- No external dependencies needed (no Hurl, no running server)
- Tests verify core logic, state management, and concurrency safety
- Production Hyper-V operations are tested manually on Windows
