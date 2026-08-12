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

# tests/suites/30-vms-crud.sh — VM CRUD: POST (add), GET (list), DELETE.
# Webhook URL указывает на webhook-mock (запущенный отдельно от pomen-ci).
# Готовит ВМ "vm0" для последующих suites (containers/provision/state).

section "vms (CRUD)"

if [ ${#FATALS[@]} -gt 0 ]; then
    skip "vms checks (pre-flight failed)"
    exit 0
fi

WEBHOOK_URL="${WEBHOOK_MOCK_URL:-http://localhost:18091}"

# Начальный список должен быть пустым (state-файл создан заново в CI).
S=$(GET /api/vms)
check "GET /api/vms (initial empty) → 200" 200 "$S" || true

# Добавляем vm0.
S=$(POST /api/vms "{\"name\":\"vm0\",\"node\":\"node0\",\"ip\":\"10.0.0.5\",\"webhook_url\":\"$WEBHOOK_URL\",\"secret\":\"smoke-secret\"}")
check "POST /api/vms (vm0) → 200" 200 "$S" || true
echo "  body: $(body)"

# Список должен содержать vm0.
GET /api/vms >/dev/null
if body | grep -q '"vm0"'; then
    printf "  ${GREEN}✓${RESET} vm0 visible in GET /api/vms\n"
    PASS=$((PASS + 1))
else
    printf "  ${RED}✗${RESET} vm0 missing after POST\n"
    FAIL=$((FAIL + 1))
fi

# REGRESSION (этап 2): секрет НЕ должен утекать через GET /api/vms.
if body | grep -q "smoke-secret"; then
    printf "  ${RED}✗${RESET} SECRET LEAKED: 'smoke-secret' present in response\n"
    FAIL=$((FAIL + 1))
else
    printf "  ${GREEN}✓${RESET} secret hidden in GET /api/vms\n"
    PASS=$((PASS + 1))
fi

# Невалидные запросы.
S=$(POST /api/vms '{"name":"bad","node":"no-such","ip":"1.2.3.4","webhook_url":"http://x"}')
check "POST /api/vms with unknown node → 400" 400 "$S" || true

S=$(POST /api/vms '{"name":"incomplete"}')
check "POST /api/vms with missing fields → 400" 400 "$S" || true
