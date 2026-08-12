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

# tests/suites/60-deprovision.sh — DELETE /api/deprovision/:route_id убирает
# маршрут из state (и best-effort из Caddy — caddy-mock принимает DELETE).

section "deprovision"

if [ ${#FATALS[@]} -gt 0 ]; then
    skip "deprovision checks (pre-flight failed)"
    exit 0
fi

# Находим route_id, выданный в 50-provision.
GET /api/state >/dev/null
RID=$(body | jq -r '.[0].route_id // empty' 2>/dev/null)

if [ -z "$RID" ] || [ "$RID" = "null" ]; then
    printf "  ${RED}✗${RESET} no route_id in state to deprovision\n"
    FAIL=$((FAIL + 1))
    exit 0
fi
printf "  route_id to delete: %s\n" "$RID"

S=$(DELETE "/api/deprovision/$RID")
check "DELETE /api/deprovision/$RID → 200" 200 "$S" || true

# State теперь пустой.
GET /api/state >/dev/null
count=$(body | jq 'length' 2>/dev/null)
if [ "$count" = "0" ]; then
    printf "  ${GREEN}✓${RESET} state empty after deprovision\n"
    PASS=$((PASS + 1))
else
    printf "  ${RED}✗${RESET} state still has %s records after deprovision\n" "$count"
    FAIL=$((FAIL + 1))
fi

# Повторное удаление того же route_id → 404.
S=$(DELETE "/api/deprovision/$RID")
check "DELETE /api/deprovision/<deleted> → 404" 404 "$S" || true
