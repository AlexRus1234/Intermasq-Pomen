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

# tests/suites/20-nodes.sh — GET /api/nodes отдаёт ключи из config.json.

section "nodes"

if [ ${#FATALS[@]} -gt 0 ]; then
    skip "nodes checks (pre-flight failed)"
    exit 0
fi

S=$(GET /api/nodes)
check "GET /api/nodes → 200" 200 "$S" || true
echo "  body: $(body)"

if [ "$S" = "200" ]; then
    if body | grep -q "node0"; then
        printf "  ${GREEN}✓${RESET} node0 listed\n"
        PASS=$((PASS + 1))
    else
        printf "  ${RED}✗${RESET} expected node0 in nodes list\n"
        FAIL=$((FAIL + 1))
    fi
fi
