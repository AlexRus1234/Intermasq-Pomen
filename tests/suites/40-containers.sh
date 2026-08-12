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

# tests/suites/40-containers.sh — GET /api/containers?vm=vm0 дёргает
# webhook-mock и возвращает нормализованный список.

section "containers"

if [ ${#FATALS[@]} -gt 0 ]; then
    skip "containers checks (pre-flight failed)"
    exit 0
fi

# Без ?vm= параметра — 400.
S=$(GET /api/containers)
check "GET /api/containers (no vm param) → 400" 400 "$S" || true

S=$(GET "/api/containers?vm=vm0")
check "GET /api/containers?vm=vm0 → 200" 200 "$S" || true
echo "  body: $(body)"

if [ "$S" = "200" ]; then
    # webhook-mock отдаёт 2 контейнера: systemd-athens (running, port 8080)
    # и 01-backend (exited, без port label).
    count=$(body | jq 'length' 2>/dev/null)
    if [ "$count" = "2" ]; then
        printf "  ${GREEN}✓${RESET} 2 containers returned\n"
        PASS=$((PASS + 1))
    else
        printf "  ${RED}✗${RESET} expected 2 containers, got %s\n" "$count"
        FAIL=$((FAIL + 1))
    fi

    # Имя должно быть нормализовано: "systemd-athens" → "athens".
    if body | grep -q '"name":"athens"'; then
        printf "  ${GREEN}✓${RESET} systemd- prefix stripped (athens)\n"
        PASS=$((PASS + 1))
    else
        printf "  ${RED}✗${RESET} systemd- normalization failed\n"
        FAIL=$((FAIL + 1))
    fi

    # label port override должен примениться: port=8080 (не host_port 8081).
    if body | grep -q '"port":"8080"'; then
        printf "  ${GREEN}✓${RESET} label port-8080 override applied\n"
        PASS=$((PASS + 1))
    else
        printf "  ${RED}✗${RESET} port label override missing\n"
        FAIL=$((FAIL + 1))
    fi
fi
