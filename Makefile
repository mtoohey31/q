.PHONY: all
all: q

q: go.mod go.sum vendor/modules.txt $(shell find . -type d -o -name '*.go' -o -name '*.tmpl')
	go build -o $@

.PHONY: ci
ci: all fmt-check vet

.PHONY: fmt
fmt:
	gofmt -w $$(find . -name vendor -prune -false -o -name '*.go')

.PHONY: fmt-check
fmt-check:
	test -z "$$(gofmt -l $$(find . -name vendor -prune -false -o -name '*.go'))"

.PHONY: vet
vet:
	go vet ./...

.PHONY: clean
clean:
	rm -f q result

.PHONY: clean-all
clean-all: clean
	go clean -cache -modcache
