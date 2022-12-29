.PHONY: all
all: q

q: go.mod go.sum internal/version/version.txt vendor/modules.txt $(shell find . -type d -o -name '*.go')
	go build -o $@

.PHONY: ci
ci: all fmt-check vet revive errcheck

.PHONY: fmt
fmt:
	gofmt -w $$(find . -name vendor -prune -false -o -name '*.go')

.PHONY: fmt-check
fmt-check:
	test -z "$$(gofmt -l $$(find . -name vendor -prune -false -o -name '*.go'))"

.PHONY: vet
vet:
	go vet ./...

.PHONY: revive
revive:
	revive -config .revive.toml -formatter friendly -set_exit_status -exclude ./vendor/... ./...

.PHONY: errcheck
errcheck:
	errcheck ./...
