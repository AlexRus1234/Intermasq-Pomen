# Pomen - plugin for Intermasq
# Copyright (C) 2026 AlexRus1234
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU Affero General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
# GNU Affero General Public License for more details.
#
# You should have received a copy of the GNU Affero General Public License
# along with this program. If not, see <https://www.gnu.org/licenses/>.

# tests/suites/50-provision.sh — POST /api/provision happy path:
# контейнер "athens" (running, port 8080) получает домен, state-record
# появляется в /api/state.

section "provision"

if [ ${#FATALS[@]} -gt 0 ]; then
    skip "provision checks (pre-flight failed)"
    exit 0
fi

# Happy path: athens (running, port 8080) → домен athens.vm0.node0.internal.
S=$(POST /api/provision '{"vm":"vm0","container_name":"athens"}')
check "POST /api/provision (athens) → 200" 200 "$S" || true
echo "  body: $(body)"

if [ "$S" = "200" ]; then
    if body | grep -q 'athens\.vm0\.node0\.internal'; then
        printf "  ${GREEN}✓${RESET} domain athens.vm0.node0.internal issued\n"
        PASS=$((PASS + 1))
    else
        printf "  ${RED}✗${RESET} expected domain in response\n"
        FAIL=$((FAIL + 1))
    fi
fi

# State-файл должен содержать выданный маршрут.
GET /api/state >/dev/null
if body | grep -q 'athens\.vm0\.node0\.internal'; then
    printf "  ${GREEN}✓${RESET} route visible in GET /api/state\n"
    PASS=$((PASS + 1))
else
    printf "  ${RED}✗${RESET} route missing in state after Provision\n"
    FAIL=$((FAIL + 1))
fi

# Невалидные запросы.
S=$(POST /api/provision '{"vm":"vm0","container_name":"non-existent"}')
check "POST /api/provision (missing container) → 404" 404 "$S" || true

S=$(POST /api/provision '{"vm":"vm0","container_name":"back"}')
check "POST /api/provision (container without port label) → 400" 400 "$S" || true
echo "  body: $(body)"

S=$(POST /api/provision '{"vm":"vm0"}')
check "POST /api/provision (missing container_name) → 400" 400 "$S" || true
