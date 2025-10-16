# Health Endpoint Tests
# Tests the /health endpoint to verify system status

# Test 1: Health endpoint returns 200 OK
GET http://localhost:8080/health

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.status" == "healthy"
jsonpath "$.vms" isInteger
jsonpath "$.ready" isInteger
jsonpath "$.running" isInteger
# With mock mode and pool size of 4, we expect all 4 VMs to be ready initially
jsonpath "$.vms" == 4
