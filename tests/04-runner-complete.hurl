# Runner Complete Endpoint Tests
# Tests the /api/runner-complete/{vmName} endpoint that VMs use to notify job completion

# Test 1: Valid VM (runner-1) completion notification
POST http://localhost:8080/api/runner-complete/runner-1

HTTP 200
[Asserts]
body contains "OK"


# Test 2: Valid VM (runner-2) completion notification
POST http://localhost:8080/api/runner-complete/runner-2

HTTP 200
[Asserts]
body contains "OK"


# Test 3: Valid VM (runner-3) completion notification
POST http://localhost:8080/api/runner-complete/runner-3

HTTP 200
[Asserts]
body contains "OK"


# Test 4: Valid VM (runner-4) completion notification
POST http://localhost:8080/api/runner-complete/runner-4

HTTP 200
[Asserts]
body contains "OK"


# Test 5: Non-existent VM (returns 200 even if VM doesn't exist)
# The server doesn't validate VM existence in this endpoint
POST http://localhost:8080/api/runner-complete/runner-99

HTTP 200
[Asserts]
body contains "OK"
