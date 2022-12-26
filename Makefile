.PHONY: ci
ci: fmt-check revive errcheck

.PHONY: fmt
fmt:
	gofmt -w .

.PHONY: fmt-check
fmt-check:
	test -z "$$(gofmt -l $$(find . -name vendor -prune -false -o -name '*.go'))"

.PHONY: revive
revive:
	revive -config .revive.toml -formatter friendly -set_exit_status -exclude ./vendor/... ./...

.PHONY: errcheck
errcheck:
	errcheck ./...
