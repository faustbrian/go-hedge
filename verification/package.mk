GO ?= go

.PHONY: api architecture clean-consumer dependencies docs

api:
	./scripts/check-api-compat.sh

architecture:
	./scripts/check-architecture.sh

clean-consumer:
	./scripts/check-clean-consumer.sh

dependencies:
	$(GO) mod verify
	$(GO) list -mod=readonly -deps ./... >/dev/null
	grep -Eq 'Failsafe-Go v0\.9\.6.*MIT License' THIRD_PARTY_LICENSES.md
	grep -Eq 'bitset v1\.24\.4.*MIT License' THIRD_PARTY_LICENSES.md

docs:
	./scripts/check-docs.sh
