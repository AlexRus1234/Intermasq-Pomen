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

# tests/lib/http.sh — curl wrappers + JSON helpers for Pomen smoke.
# Adapted from Intermasq: dropped Bearer/KGET machinery (no auth in
# Pomen dev mode). PGET/GET/POST/DELETE — все без авторизации, т.к. dev-режим
# Pomen работает без auth proxy.

jval() { jq -r "$1" 2>/dev/null; }

# Все хелперы пишут тело в /tmp/pomen-smoke.body, на stdout отдают HTTP-код.
GET()    { curl -s -o /tmp/pomen-smoke.body -w "%{http_code}" "$BASE$1"; }
POST()   { curl -s -o /tmp/pomen-smoke.body -w "%{http_code}" -H "Content-Type: application/json" -X POST -d "$2" "$BASE$1"; }
DELETE() { curl -s -o /tmp/pomen-smoke.body -w "%{http_code}" -X DELETE "$BASE$1"; }

body() { cat /tmp/pomen-smoke.body; }
