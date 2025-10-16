# Runner Configuration Endpoint Tests
# Tests the /api/runner-config/{vmName} endpoint that VMs use to get their configuration

# Test 1: Non-existent VM returns 404
GET http://localhost:8080/api/runner-config/runner-99

HTTP 404
[Asserts]
body contains "VM not found"


# Test 2: Valid VM (runner-1) returns configuration
GET http://localhost:8080/api/runner-config/runner-1

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.token" isString
jsonpath "$.organization" == "mock-org"
jsonpath "$.repository" isEmpty
jsonpath "$.name" == "runner-1"
jsonpath "$.labels" == "self-hosted,Windows,X64,ephemeral"


# Test 3: Valid VM (runner-2) returns configuration
GET http://localhost:8080/api/runner-config/runner-2

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.token" isString
jsonpath "$.organization" == "mock-org"
jsonpath "$.name" == "runner-2"
jsonpath "$.labels" == "self-hosted,Windows,X64,ephemeral"


# Test 4: Valid VM (runner-3) returns configuration
GET http://localhost:8080/api/runner-config/runner-3

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.token" isString
jsonpath "$.organization" == "mock-org"
jsonpath "$.name" == "runner-3"


# Test 5: Valid VM (runner-4) returns configuration
GET http://localhost:8080/api/runner-config/runner-4

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.token" isString
jsonpath "$.organization" == "mock-org"
jsonpath "$.name" == "runner-4"
