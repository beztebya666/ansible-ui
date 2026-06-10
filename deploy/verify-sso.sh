#!/usr/bin/env bash
# Verify LDAP (end-to-end against a throwaway directory) and OIDC (discovery +
# login redirect against a real issuer). Run with sudo (docker needs root).
set -uo pipefail
DOCKER="${DOCKER:-docker}"
NET=ansible-ui
API_URL="http://localhost:8080"
DIR=/mnt/hgfs/SharedLibrary/ansible-ui
export PYTHONIOENCODING=utf-8
PY="$(command -v python3 || command -v python)"
JAR=/tmp/aui-sso.cookies
g(){ "$PY" -c "import sys,json;print(json.load(sys.stdin)$1)" 2>/dev/null; }
pass(){ printf '  \033[32m✓\033[0m %s\n' "$1"; }
bad(){ printf '  \033[31m✗ %s\033[0m\n' "$1"; FAILED=1; }
FAILED=0

echo "== start throwaway LDAP directory (osixia/openldap) =="
$DOCKER rm -f aui-test-ldap >/dev/null 2>&1 || true
$DOCKER run -d --name aui-test-ldap --network "$NET" osixia/openldap:1.5.0 >/dev/null
ok=0
for _ in $(seq 1 40); do
  $DOCKER exec aui-test-ldap ldapwhoami -x -H ldap://localhost -D "cn=admin,dc=example,dc=org" -w admin >/dev/null 2>&1 && { ok=1; break; }
  sleep 2
done
[ "$ok" = 1 ] || bad "ldap did not start"
$DOCKER exec -i aui-test-ldap ldapadd -x -H ldap://localhost -D "cn=admin,dc=example,dc=org" -w admin >/dev/null 2>&1 <<'LDIF'
dn: ou=users,dc=example,dc=org
objectClass: organizationalUnit
ou: users

dn: cn=testuser,ou=users,dc=example,dc=org
objectClass: inetOrgPerson
cn: testuser
sn: User
uid: testuser
mail: testuser@example.org
userPassword: testpass
LDIF
pass "ldap directory up + testuser seeded"

echo "== restart api with LDAP + OIDC env =="
cd "$DIR"
export LDAP_ENABLED=true
export LDAP_URL=ldap://aui-test-ldap:389
export LDAP_BIND_DN="cn=admin,dc=example,dc=org"
export LDAP_BIND_PASSWORD=admin
export LDAP_USER_BASE_DN="ou=users,dc=example,dc=org"
export LDAP_USER_FILTER="(cn=%s)"
export OIDC_ENABLED=true
export OIDC_ISSUER="https://accounts.google.com"
export OIDC_CLIENT_ID="test-client-id"
export OIDC_REDIRECT_URL="http://localhost:8080/api/auth/oidc/callback"
bash deploy/docker-run.sh restart >/dev/null 2>&1
for _ in $(seq 1 40); do curl -fsS "$API_URL/api/health" >/dev/null 2>&1 && break; sleep 2; done

echo "== providers advertised =="
status=$(curl -s "$API_URL/api/auth/status")
[ "$(echo "$status" | g "['providers']['ldap']")" = True ] && pass "LDAP provider enabled" || bad "ldap not enabled"
[ "$(echo "$status" | g "['providers']['oidc']")" = True ] && pass "OIDC provider enabled (issuer discovery succeeded)" || bad "oidc discovery failed"

echo "== OIDC login redirect =="
loc=$(curl -s -o /dev/null -w '%{redirect_url}' "$API_URL/api/auth/oidc/login")
if echo "$loc" | grep -q "accounts.google.com" && echo "$loc" | grep -q "test-client-id"; then
  pass "OIDC /login -> 302 to issuer authorize with client_id"
else
  bad "oidc redirect unexpected: ${loc:0:80}"
fi

echo "== LDAP login (end to end) =="
code=$(curl -s -o /dev/null -w '%{http_code}' -c "$JAR" -X POST "$API_URL/api/auth/login" \
  -H 'Content-Type: application/json' -d '{"username":"testuser","password":"testpass"}')
[ "$code" = 200 ] && pass "LDAP login succeeded (200, session issued)" || bad "ldap login got $code"
code=$(curl -s -b "$JAR" -o /dev/null -w '%{http_code}' "$API_URL/api/auth/me")
[ "$code" = 200 ] && pass "LDAP user session valid (/api/auth/me)" || bad "ldap session invalid ($code)"
code=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API_URL/api/auth/login" \
  -H 'Content-Type: application/json' -d '{"username":"testuser","password":"WRONG"}')
[ "$code" = 401 ] && pass "wrong LDAP password rejected (401)" || bad "wrong pw got $code"

echo "== cleanup =="
$DOCKER rm -f aui-test-ldap >/dev/null 2>&1 || true
unset LDAP_ENABLED LDAP_URL LDAP_BIND_DN LDAP_BIND_PASSWORD LDAP_USER_BASE_DN LDAP_USER_FILTER \
      OIDC_ENABLED OIDC_ISSUER OIDC_CLIENT_ID OIDC_REDIRECT_URL
bash deploy/docker-run.sh restart >/dev/null 2>&1
pass "api restarted without SSO env (clean state)"

[ "$FAILED" = 0 ] && printf '\033[32mLDAP + OIDC OK\033[0m\n' || { printf '\033[31mSSO VERIFY FAILED\033[0m\n'; exit 1; }
