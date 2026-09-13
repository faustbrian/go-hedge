#!/usr/bin/env bash
set -euo pipefail

current=$(mktemp)
baseline=compat/public-api.txt
trap 'rm -f "$current"' EXIT
while IFS= read -r package; do
	printf 'PACKAGE %s\n' "$package" >> "$current"
	go doc -all "$package" >> "$current"
done < <(go list ./...)
perl -0pi -e 's/\n+\z/\n/' "$current"
if [[ "${1:-}" == "--update" ]]; then
	mkdir -p "$(dirname "$baseline")"
	cp "$current" "$baseline"
	exit 0
fi
if [[ ! -f "$baseline" ]]; then
	echo "$baseline is missing; run make api-update" >&2
	exit 1
fi
diff -u "$baseline" "$current"
