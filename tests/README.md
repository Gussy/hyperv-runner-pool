# API Tests

This directory contains Hurl test files for the Hyper-V Runner Pool API endpoints.

## Prerequisites

- [Hurl](https://hurl.dev/) must be installed
- The server must be running in mock mode

## Test Files

- `health.hurl` - Tests the `/health` endpoint
- `webhook.hurl` - Tests the `/webhook` endpoint with signature verification
- `runner-config.hurl` - Tests the `/api/runner-config/{vmName}` endpoint
- `runner-complete.hurl` - Tests the `/api/runner-complete/{vmName}` endpoint

## Running Tests

### Option 1: Using Task (Recommended)

```bash
# Start the server in one terminal
task run

# Run API tests in another terminal
task test-api

# Or run all tests (unit + API)
task test-all
```

### Option 2: Manual

```bash
# Start the server with mock VMs in one terminal
USE_MOCK=true go run main.go

# Run hurl tests in another terminal
hurl --test tests/*.hurl

# Or run individual test files
hurl --test tests/health.hurl
hurl --test tests/webhook.hurl
hurl --test tests/runner-config.hurl
hurl --test tests/runner-complete.hurl
```

## Test Coverage

### Health Endpoint (`/health`)
- ✅ Returns 200 OK
- ✅ Returns valid JSON structure
- ✅ Reports VM pool status

### Webhook Endpoint (`/webhook`)
- ✅ Rejects requests without signature (401)
- ✅ Rejects requests with invalid signature (401)
- ✅ Accepts requests with valid HMAC-SHA256 signature (200)
- ✅ Handles workflow job events

### Runner Config Endpoint (`/api/runner-config/{vmName}`)
- ✅ Returns 404 for non-existent VM
- ✅ Returns configuration for valid VMs (runner-1 through runner-4)
- ✅ Returns proper JSON structure with token, organization, repository, name, labels

### Runner Complete Endpoint (`/api/runner-complete/{vmName}`)
- ✅ Accepts completion notifications (200 OK)
- ✅ Handles all valid VMs
- ✅ Gracefully handles non-existent VMs

## Notes

- All tests run against `http://localhost:8080` by default
- Tests expect the server to be running in mock mode (`USE_MOCK=true`)
- Webhook tests use the mock secret: `mock-secret`
- Signatures in webhook tests are pre-computed HMAC-SHA256 values
