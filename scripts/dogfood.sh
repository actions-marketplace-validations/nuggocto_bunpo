#!/usr/bin/env bash
# Runs Bunpō's own tests through Bunpō, one machine's share at a time. Each
# Go package with tests is one test file to Bunpō, named by its directory.
#
#   scripts/dogfood.sh packages     print every package directory with tests
#   INDEX=0 TOTAL=2 scripts/dogfood.sh
#                                   split, run this machine's share, record
set -euo pipefail

module=$(go list -m)

packages() {
	go list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}' ./... | sed "s|^$module/||"
}

if [ "${1:-}" = packages ]; then
	packages
	exit 0
fi

: "${INDEX:?}" "${TOTAL:?}"

packages | bunpo split --index "$INDEX" --total "$TOTAL" >mine.txt

mine=()
while IFS= read -r dir; do mine+=("./$dir"); done <mine.txt

# Given no packages, go test would test the current directory instead.
if [ "${#mine[@]}" -eq 0 ]; then
	bunpo record --index "$INDEX"
	exit 0
fi

# -count=1, because a cached result reports 0 ms.
go test -race -count=1 -json "${mine[@]}" | tee go-test.json | jq -j 'select(.Action == "output") | .Output'

# One "<directory> <milliseconds>" line per package that passed.
jq -r --arg prefix "$module/" \
	'select(.Action == "pass" and .Test == null) | "\(.Package | ltrimstr($prefix)) \(.Elapsed * 1000 | round)"' \
	go-test.json >times.txt

bunpo record --index "$INDEX" --format lines times.txt
