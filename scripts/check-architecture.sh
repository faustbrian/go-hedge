#!/usr/bin/env bash
set -euo pipefail

production_go_files() {
	files=$(git ls-files --cached --others --exclude-standard -- '*.go')
	if [ -z "$files" ]; then
		return
	fi
	printf '%s\n' "$files" | while IFS= read -r file; do
		case "$file" in
		*_test.go | .golib-tooling/* | .verification/*) continue ;;
		esac
		printf '%s\n' "$file"
	done
}

files=$(production_go_files)
matches=$(printf '%s\n' "$files" | while IFS= read -r file; do
	[ -n "$file" ] || continue
	grep -nEH '"unsafe"|//go:linkname|import[[:space:]]+"C"|func[[:space:]]+init[[:space:]]*\(' "$file" || true
done)
if [ -n "$matches" ]; then
	printf '%s\n' "$matches"
	echo 'forbidden production runtime mechanism found' >&2
	exit 1
fi
dependencies=$(go list -deps -f '{{.ImportPath}}' .)
if printf '%s\n' "$dependencies" | grep -E '^github\.com/faustbrian/golib/pkg/(retry|circuit-breaker|bulkhead|rate-limit)(/|$)'; then
	echo 'an owning resilience sibling entered the core dependency graph' >&2
	exit 1
fi
CGO_ENABLED=0 go test ./...
echo 'architecture constraints pass'
