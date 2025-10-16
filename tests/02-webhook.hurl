# Webhook Endpoint Tests
# Tests the /webhook endpoint that receives GitHub webhook events
# Note: Webhook secret for mock mode is "mock-secret"

# Test 1: Reject request without signature
POST http://localhost:8080/webhook
Content-Type: application/json
{
  "action": "queued",
  "workflow_job": {
    "id": 123456,
    "status": "queued"
  }
}

HTTP 401
[Asserts]
body contains "Missing signature"


# Test 2: Reject request with invalid signature
POST http://localhost:8080/webhook
Content-Type: application/json
X-Hub-Signature-256: sha256=invalid_signature_here
{
  "action": "queued",
  "workflow_job": {
    "id": 123456,
    "status": "queued"
  }
}

HTTP 401
[Asserts]
body contains "Invalid signature"


# Test 3: Accept request with valid HMAC-SHA256 signature
# Signature for compact JSON: {"action":"queued","workflow_job":{"id":789012,"status":"queued"}}
POST http://localhost:8080/webhook
Content-Type: application/json
X-Hub-Signature-256: sha256=92e80297dd84b6554f79e3cdd6f1e0db65be8124327455e4c3638e344bafee0d
{"action":"queued","workflow_job":{"id":789012,"status":"queued"}}

HTTP 200
[Asserts]
body contains "OK"


# Test 4: Accept completed workflow job event
# Signature for compact JSON: {"action":"completed","workflow_job":{"id":789013,"status":"completed"}}
POST http://localhost:8080/webhook
Content-Type: application/json
X-Hub-Signature-256: sha256=5bbce00115f632134d0ef96f31ed6028b4457b6e220eeb0a91b929a4b67801bd
{"action":"completed","workflow_job":{"id":789013,"status":"completed"}}

HTTP 200
[Asserts]
body contains "OK"
