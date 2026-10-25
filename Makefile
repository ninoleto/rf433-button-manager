GO ?= go
NODE ?= node
TEST_FLAGS ?= -race -count=1 -timeout=2m
TEST_PACKAGES ?= ./...

.PHONY: build test test-ui test-deploy vet check clean
build:
	mkdir -p bin
	GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -mod=readonly -trimpath -buildvcs=false -ldflags="-s -w -buildid=" -o bin/rf-button-manager ./cmd/rf-button-manager
test:
	GOTOOLCHAIN=local $(GO) test $(TEST_FLAGS) $(TEST_PACKAGES)
test-ui:
	$(NODE) --check internal/webui/static/app.js
	$(NODE) --test internal/webui/learning.test.cjs
test-deploy:
	for script in scripts/*.sh; do bash -n "$$script"; done
	bash scripts/test-deploy.sh
	bash scripts/test-history.sh
	python3 scripts/test-update-prebuilt.py
vet:
	GOTOOLCHAIN=local $(GO) vet ./...
check: test vet test-ui test-deploy
clean:
	rm -rf bin
