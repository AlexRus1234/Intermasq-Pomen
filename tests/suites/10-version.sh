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

# tests/suites/10-version.sh — /api/version end-to-end.

section "version"

if [ "${FATALS+x}" != "" ] && [ ${#FATALS[@]} -gt 0 ]; then
    skip "version checks (pre-flight failed)"
else
    S=$(GET /api/version)
    check "GET /api/version → 200" 200 "$S" || true
    echo "  body: $(body)"

    if [ "$S" = "200" ]; then
        v=$(body | jval .version)
        if [ -n "$v" ] && [ "$v" != "null" ]; then
            printf "  ${GREEN}✓${RESET} version field present: %s\n" "$v"
            PASS=$((PASS + 1))
        else
            printf "  ${RED}✗${RESET} version field missing/null\n"
            FAIL=$((FAIL + 1))
        fi
    fi
fi
