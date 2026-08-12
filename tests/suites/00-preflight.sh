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

# tests/suites/00-preflight.sh — sanity probe.
# Гарантирует, что pomen-ci запущен и отвечает на базовые запросы.
# Без этого последующие suite'ы будут бессмысленны.

section "pre-flight"

S=$(GET /api/version)
if check "GET /api/version (alive probe)" 200 "$S"; then
    echo "  body: $(body)"
else
    fatal "pomen-ci unreachable at $BASE — subsequent suites will be skipped"
fi
