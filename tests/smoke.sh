#!/usr/bin/env bash
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

# tests/smoke.sh — Pomen smoke suite orchestrator.
# Запускает все tests/suites/NN-*.sh против pomen-ci, поднятого в dev-режиме.
# Сам pomen-ci, caddy-mock и webhook-mock поднимаются отдельным шагом в CI
# (см. .forgejo/workflows/build.yml, шаг "L3 — smoke tests").
#
# Usage:
#   export BASE=http://localhost:18992
#   ./tests/smoke.sh
#
# Exits 0 на clean pass или когда все fail-ы помечены как KNOWN в
# tests/known-bugs.txt.

set -u

TESTS_DIR="$(cd "$(dirname "$0")" && pwd)"

source "$TESTS_DIR/lib/state.sh"
source "$TESTS_DIR/lib/common.sh"
source "$TESTS_DIR/lib/http.sh"

init_state

for _suite in "$TESTS_DIR/suites"/[0-9]*.sh; do
    [ -f "$_suite" ] || continue
    source "$_suite"
done

print_summary
exit $?
