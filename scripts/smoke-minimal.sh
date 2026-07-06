#!/usr/bin/env bash
# Smoke tests for RS_SETUP_PROFILE=minimal (auth, pipelines, pipeline vars, ES proxy).
#
# Usage:
#   ./scripts/smoke-minimal.sh
#   RS_URL=http://localhost:8000 RS_USER=foo RS_PASSWORD=bar ./scripts/smoke-minimal.sh

set -u

RS_URL="${RS_URL:-http://localhost:8000}"
RS_USER="${RS_USER:-foo}"
RS_PASSWORD="${RS_PASSWORD:-bar}"
AUTH="${RS_USER}:${RS_PASSWORD}"
PIPELINE_ID="${PIPELINE_ID:-minimal-smoke-test}"
PIPELINE_PATH="/api/${PIPELINE_ID}"

PASS=0
FAIL=0
SKIP=0

green() { printf '\033[32m%s\033[0m\n' "$*"; }
red() { printf '\033[31m%s\033[0m\n' "$*"; }
yellow() { printf '\033[33m%s\033[0m\n' "$*"; }

pass() { PASS=$((PASS + 1)); green "PASS: $*"; }
fail() { FAIL=$((FAIL + 1)); red "FAIL: $*"; }
skip() { SKIP=$((SKIP + 1)); yellow "SKIP: $*"; }

http_code() {
	curl -s -o /dev/null -w "%{http_code}" "$@"
}

http_body() {
	curl -s "$@"
}

echo "=== ReactiveSearch minimal profile smoke test ==="
echo "RS_URL=${RS_URL}  user=${RS_USER}"
echo

# --- 1. Health (no auth) ---
code="$(http_code "${RS_URL}/arc/health")"
if [ "$code" = "200" ]; then
	pass "GET /arc/health -> ${code}"
else
	fail "GET /arc/health -> ${code} (expected 200)"
fi

arc_health="$(http_body "${RS_URL}/arc/_health")"
if echo "$arc_health" | grep -q '"health":"ok"'; then
	pass "GET /arc/_health reports ok"
else
	fail "GET /arc/_health unexpected body: ${arc_health}"
fi

# --- 2. Auth ---
code="$(http_code "${RS_URL}/_users")"
if [ "$code" = "401" ]; then
	pass "GET /_users without auth -> 401"
else
	fail "GET /_users without auth -> ${code} (expected 401)"
fi

users_body="$(http_body -u "$AUTH" "${RS_URL}/_users")"
if echo "$users_body" | grep -q '"foo"'; then
	pass "GET /_users with auth lists user foo"
else
	fail "GET /_users with auth missing foo: ${users_body}"
fi

# --- 3. Meta indices (exactly 4 dot indices from minimal profile) ---
meta_indices="$(http_body -u "$AUTH" "${RS_URL}/_cat/indices/.users,.permissions,.pipelines,.pipeline_vars?h=index")"
for idx in .users .permissions .pipelines .pipeline_vars; do
	if echo "$meta_indices" | grep -qx "$idx"; then
		pass "meta index exists: ${idx}"
	else
		fail "meta index missing: ${idx}"
	fi
done

# --- 4. Pipelines ---
pipeline_json="$(cat <<EOF
{
  "id": "${PIPELINE_ID}",
  "enabled": true,
  "routes": [{"path": "${PIPELINE_PATH}", "method": "GET"}],
  "stages": [{
    "id": "respond",
    "script": "function handleRequest() { return { response: { body: JSON.stringify({ ok: true, pipeline: '${PIPELINE_ID}' }), code: 200 } }; }"
  }]
}
EOF
)"
pipeline_file="$(mktemp /tmp/rs-smoke-pipeline.XXXXXX.json)"
printf '%s' "$pipeline_json" > "$pipeline_file"

existing="$(http_body -u "$AUTH" "${RS_URL}/_pipeline/${PIPELINE_ID}" 2>/dev/null || true)"
if echo "$existing" | grep -q "\"id\":\"${PIPELINE_ID}\"" || echo "$existing" | grep -q "\"id\": \"${PIPELINE_ID}\""; then
	skip "pipeline ${PIPELINE_ID} already exists"
	rm -f "$pipeline_file"
else
	create_code="$(curl -s -o /tmp/rs-smoke-pipeline-create.json -w "%{http_code}" -u "$AUTH" \
		-X POST "${RS_URL}/_pipeline" \
		-F "pipeline=@${pipeline_file};filename=pipeline.json")"
	rm -f "$pipeline_file"
	if [ "$create_code" = "201" ] || [ "$create_code" = "200" ]; then
		pass "POST /_pipeline -> ${create_code}"
	else
		fail "POST /_pipeline -> ${create_code}: $(cat /tmp/rs-smoke-pipeline-create.json 2>/dev/null)"
	fi
fi

invoke_body="$(http_body -u "$AUTH" "${RS_URL}${PIPELINE_PATH}")"
if echo "$invoke_body" | grep -q '"ok":true'; then
	pass "GET ${PIPELINE_PATH} returns ok"
else
	fail "GET ${PIPELINE_PATH} unexpected: ${invoke_body}"
fi

# --- 5. Pipeline env vars ---
env_key="SMOKE_GREETING_$(date +%s)"
env_body="$(cat <<EOF
{"label":"Smoke test","key":"${env_key}","value":"hello"}
EOF
)"
env_code="$(curl -s -o /tmp/rs-smoke-env.json -w "%{http_code}" -u "$AUTH" \
	-X POST "${RS_URL}/_pipelines/env" \
	-H 'Content-Type: application/json' \
	-d "$env_body")"
if [ "$env_code" = "200" ] || [ "$env_code" = "201" ]; then
	pass "POST /_pipelines/env -> ${env_code}"
else
	fail "POST /_pipelines/env -> ${env_code}: $(cat /tmp/rs-smoke-env.json 2>/dev/null)"
fi

envs_body="$(http_body -u "$AUTH" "${RS_URL}/_pipelines/envs")"
if echo "$envs_body" | grep -q "$env_key"; then
	pass "GET /_pipelines/envs contains ${env_key}"
else
	fail "GET /_pipelines/envs missing ${env_key}: ${envs_body}"
fi

# --- 6. ES proxy ---
cluster_code="$(http_code -u "$AUTH" "${RS_URL}/_cluster/health")"
if [ "$cluster_code" = "200" ]; then
	pass "GET /_cluster/health -> 200"
else
	fail "GET /_cluster/health -> ${cluster_code}"
fi

sample_index="$(http_body -u "$AUTH" "${RS_URL}/_cat/indices?h=index" | grep -v '^\.' | grep -v '^$' | head -1 || true)"
if [ -n "$sample_index" ]; then
	search_code="$(curl -s -o /tmp/rs-smoke-search.json -w "%{http_code}" -u "$AUTH" \
		-X POST "${RS_URL}/${sample_index}/_search" \
		-H 'Content-Type: application/json' \
		-d '{"query":{"match_all":{}},"size":1}')"
	if [ "$search_code" = "200" ]; then
		pass "POST /${sample_index}/_search -> 200"
	else
		fail "POST /${sample_index}/_search -> ${search_code}"
	fi
else
	skip "no user index found for _search test"
fi

echo
echo "=== Summary: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped ==="
if [ "$FAIL" -gt 0 ]; then
	exit 1
fi
exit 0
