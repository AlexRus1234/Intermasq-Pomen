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

# tests/lib/state.sh — global run state, counters, config, known-bugs loader.
# Adapted from Intermasq's tests/lib/state.sh: dropped JWT/SECRET machinery
# (Pomen в dev-режиме не имеет auth — прокси-слой Intermasq этим занимается в
# production). What remains: BASE, counters, KNOWN_BUGS map, print_summary.

BASE="${BASE:-http://localhost:18992}"

# Path to the known-bugs list. Override via env if needed.
KNOWN_BUGS_FILE="${KNOWN_BUGS_FILE:-$(dirname "$0")/known-bugs.txt}"

PASS=0
FAIL=0
KNOWN_FAIL=0
SKIP=0
FATALS=()

declare -A KNOWN_BUGS=()
KNOWN_BUGS_LIST=""

init_state() {
    if [ -f "$KNOWN_BUGS_FILE" ]; then
        # `read` returns failure on a final line lacking a trailing newline
        # (a common Windows-editor artifact); the `|| [ -n "$_line" ]`
        # guard processes that last line anyway. Pattern inherited from
        # Intermasq's state.sh.
        while IFS= read -r _line || [ -n "$_line" ]; do
            _line="${_line%%#*}"
            _line="$(echo "$_line" | xargs)"
            [ -z "$_line" ] && continue
            _id="${_line%%[[:space:]]*}"
            [ -n "$_id" ] && KNOWN_BUGS["$_id"]=1
        done < "$KNOWN_BUGS_FILE"
    fi
    KNOWN_BUGS_LIST="$(echo "${!KNOWN_BUGS[@]}" | tr ' ' '\n' | sort | tr '\n' ' ' | sed 's/ $//')"
}

print_summary() {
    local total
    total=$((PASS + FAIL + KNOWN_FAIL + SKIP))

    printf "\n${CYAN}=== SUMMARY ===${RESET}\n"
    echo
    printf "  ${GREEN}Pass:        %d${RESET} / %d\n" "$PASS" "$total"
    printf "  ${RED}Fail:        %d${RESET} / %d  (unexpected — investigate)\n" "$FAIL" "$total"
    printf "  ${YELLOW}Known-fail:  %d${RESET} / %d  (bugs: %s)\n" "$KNOWN_FAIL" "$total" "${KNOWN_BUGS_LIST:-(none)}"
    printf "  ${BLUE}Skipped:     %d${RESET} / %d  (pre-condition failed)\n" "$SKIP" "$total"
    echo

    if [ ${#FATALS[@]} -gt 0 ]; then
        printf "${RED}FATALS (pre-condition failures):${RESET}\n"
        for _f in "${FATALS[@]}"; do
            printf "  • %s\n" "$_f"
        done
        echo
    fi

    if [ "$FAIL" -gt 0 ]; then
        printf "${RED}UNEXPECTED FAILURES — investigate.${RESET}\n"
        return 1
    fi
    if [ ${#FATALS[@]} -gt 0 ]; then
        printf "${RED}Pipeline RED due to pre-condition failures.${RESET}\n"
        return 1
    fi
    if [ "$KNOWN_FAIL" -gt 0 ]; then
        printf "${YELLOW}All failures are known bugs (regression tests). Pipeline green.${RESET}\n"
        return 0
    fi
    printf "${GREEN}CLEAN PASS.${RESET}\n"
    return 0
}
