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

# tests/suites/70-replay.sh — POST /api/replay: повторная запись конфига
# Caddy из state-файла. Поднимаем state через provision, потом replay.

section "replay"

if [ ${#FATALS[@]} -gt 0 ]; then
    skip "replay checks (pre-flight failed)"
    exit 0
fi

# Подготовка: provision ещё раз, чтобы state не был пустым (предыдущий
# deprovision-suite почистил всё).
POST /api/provision '{"vm":"vm0","container_name":"athens"}' >/dev/null

S=$(POST /api/replay '{}')
check "POST /api/replay → 200" 200 "$S" || true
echo "  body: $(body)"

if [ "$S" = "200" ]; then
    ok=$(body | jval .ok)
    errs_len=$(body | jq '.errors | length' 2>/dev/null)
    if [ "$ok" = "1" ] && [ "$errs_len" = "0" ]; then
        printf "  ${GREEN}✓${RESET} replay ok=%s errors=%s\n" "$ok" "$errs_len"
        PASS=$((PASS + 1))
    else
        printf "  ${RED}✗${RESET} replay ok=%s errors=%s (want ok=1 errors=0)\n" "$ok" "$errs_len"
        FAIL=$((FAIL + 1))
    fi
fi
