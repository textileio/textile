include .bingo/Variables.mk

.DEFAULT_GOAL=build

TXTL_BUILD_FLAGS?=CGO_ENABLED=0
TXTL_VERSION?="git"
GOVVV_FLAGS=$(shell $(GOVVV) -flags -version $(TXTL_VERSION) -pkg $(shell go list ./buildinfo))

build: $(GOVVV)
	$(TXTL_BUILD_FLAGS) go build -ldflags="${GOVVV_FLAGS}" ./...
.PHONY: build

build-hub: $(GOVVV)
	$(TXTL_BUILD_FLAGS) go build -ldflags="${GOVVV_FLAGS}" ./cmd/hub
.PHONY: build-hub

build-hubd: $(GOVVV)
	$(TXTL_BUILD_FLAGS) go build -ldflags="${GOVVV_FLAGS}" ./cmd/hubd
.PHONY: build-hubd

build-buck: $(GOVVV)
	$(TXTL_BUILD_FLAGS) go build -ldflags="${GOVVV_FLAGS}" ./cmd/buck
.PHONY: build-buck

build-buckd: $(GOVVV)
	$(TXTL_BUILD_FLAGS) go build -ldflags="${GOVVV_FLAGS}" ./cmd/buckd
.PHONY: build-buckd

build-billingd: $(GOVVV)
	$(TXTL_BUILD_FLAGS) go build -ldflags="${GOVVV_FLAGS}" ./api/billingd
.PHONY: build-billingd

build-mindexd: $(GOVVV)
	$(TXTL_BUILD_FLAGS) go build -ldflags="${GOVVV_FLAGS}" ./api/mindexd
.PHONY: build-mindexd

install: $(GOVVV)
	$(TXTL_BUILD_FLAGS) go install -ldflags="${GOVVV_FLAGS}" ./...
.PHONY: install

install-hub: $(GOVVV)
	$(TXTL_BUILD_FLAGS) go install -ldflags="${GOVVV_FLAGS}" ./cmd/hub
.PHONY: install-hub

install-hubd: $(GOVVV)
	$(TXTL_BUILD_FLAGS) go install -ldflags="${GOVVV_FLAGS}" ./cmd/hubd
.PHONY: install-hubd

install-buck: $(GOVVV)
	$(TXTL_BUILD_FLAGS) go install -ldflags="${GOVVV_FLAGS}" ./cmd/buck
.PHONY: install-buck

install-buckd: $(GOVVV)
	$(TXTL_BUILD_FLAGS) go install -ldflags="${GOVVV_FLAGS}" ./cmd/buckd
.PHONY: install-buckd

install-billingd: $(GOVVV)
	$(TXTL_BUILD_FLAGS) go install -ldflags="${GOVVV_FLAGS}" ./api/billingd
.PHONY: install-billingd

install-mindexd: $(GOVVV)
	$(TXTL_BUILD_FLAGS) go install -ldflags="${GOVVV_FLAGS}" ./api/mindexd
.PHONY: install-mindexd



define gen_release_files
	$(GOX) -osarch=$(3) -output="build/$(2)/$(2)_${TXTL_VERSION}_{{.OS}}-{{.Arch}}/$(2)" -ldflags="${GOVVV_FLAGS}" $(1)
	mkdir -p build/dist; \
	cd build/$(2); \
	for release in *; do \
		cp ../../LICENSE ../../README.md $${release}/; \
		if [ $${release} != *"windows"* ]; then \
  		TXTL_FILE=$(2) $(GOMPLATE) -f ../../dist/install.tmpl -o "$${release}/install"; \
			tar -czvf ../dist/$${release}.tar.gz $${release}; \
		else \
			zip -r ../dist/$${release}.zip $${release}; \
		fi; \
	done
endef

build-hub-release: $(GOX) $(GOVVV) $(GOMPLATE)
	$(call gen_release_files,./cmd/hub,hub,"linux/amd64 linux/386 linux/arm darwin/amd64 darwin/arm64 windows/amd64")
.PHONY: build-hub-release

build-hubd-release: $(GOX) $(GOVVV) $(GOMPLATE)
	$(call gen_release_files,./cmd/hubd,hubd,"linux/amd64 linux/386 linux/arm darwin/amd64 darwin/arm64 windows/amd64")
.PHONY: build-hubd-release

build-buck-release: $(GOX) $(GOVVV) $(GOMPLATE)
	$(call gen_release_files,./cmd/buck,buck,"linux/amd64 linux/386 linux/arm darwin/amd64 darwin/arm64 windows/amd64")
.PHONY: build-buck-release

build-buckd-release: $(GOX) $(GOVVV) $(GOMPLATE)
	$(call gen_release_files,./cmd/buckd,buckd,"linux/amd64 linux/386 linux/arm darwin/amd64 darwin/arm64 windows/amd64")
.PHONY: build-buckd-release

build-billingd-release: $(GOX) $(GOVVV) $(GOMPLATE)
	$(call gen_release_files,./api/billingd,billingd,"linux/amd64 linux/386 linux/arm darwin/amd64 darwin/arm64 windows/amd64")
.PHONY: build-billingd-release

build-releases: build-hub-release build-hubd-release build-buck-release build-buckd-release build-billingd-release
.PHONY: build-releases

hub-up:
	docker-compose -f cmd/hubd/docker-compose-dev.yml up --build

hub-stop:
	docker-compose -f cmd/hubd/docker-compose-dev.yml stop

hub-clean:
	docker-compose -f cmd/hubd/docker-compose-dev.yml down -v --remove-orphans

buck-up:
	docker-compose -f cmd/buckd/docker-compose-dev.yml up --build

buck-stop:
	docker-compose -f cmd/buckd/docker-compose-dev.yml stop

buck-clean:
	docker-compose -f cmd/buckd/docker-compose-dev.yml down -v --remove-orphans

test:
	go test -race -timeout 45m ./...
.PHONY: test

clean-protos:
	find . -type f -name '*.pb.go' -delete
	find . -type f -name '*pb_test.go' -delete
.PHONY: clean-protos

clean-js-protos:
	find . -type f -name '*pb.js' ! -path "*/node_modules/*" -delete
	find . -type f -name '*pb.d.ts' ! -path "*/node_modules/*" -delete
	find . -type f -name '*pb_service.js' ! -path "*/node_modules/*" -delete
	find . -type f -name '*pb_service.d.ts' ! -path "*/node_modules/*" -delete
.PHONY: clean-js-protos

install-protoc:
	cd buildtools && ./install_protoc.bash

PROTOCGENGO=$(shell pwd)/buildtools/protoc-gen-go
protos: install-protoc clean-protos
	PATH=$(PROTOCGENGO):$(PATH) ./scripts/protoc_gen_plugin.bash \
	--proto_path=. \
	--plugin_name=go \
	--plugin_out=. \
	--plugin_opt=plugins=grpc,paths=source_relative
.PHONY: protos

js-protos: install-protoc clean-js-protos
	./scripts/gen_js_protos.bash

# local is what we run when testing locally.
# This does breaking change detection against our local git repository.
.PHONY: buf-local
buf-local: $(BUF)
	$(BUF) check lint
	# $(BUF) check breaking --against-input '.git#branch=master'

# https is what we run when testing in most CI providers.
# This does breaking change detection against our remote HTTPS git repository.
.PHONY: buf-https
buf-https: $(BUF)
	$(BUF) check lint
	# $(BUF) check breaking --against-input "$(HTTPS_GIT)#branch=master"

# ssh is what we run when testing in CI providers that provide ssh public key authentication.
# This does breaking change detection against our remote HTTPS ssh repository.
# This is especially useful for private repositories.
.PHONY: buf-ssh
buf-ssh: $(BUF)
	$(BUF) check lint
	# $(BUF) check breaking --against-input "$(SSH_GIT)#branch=master"


MINDEXDPB=$(shell pwd)/api/mindexd
PROTOC=$(shell pwd)/buildtools/protoc/bin
.PHONY: mindex-rest
mindex-rest:
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest
	PATH=$(PROTOC):$(PATH) protoc -I . --grpc-gateway_out . --grpc-gateway_opt logtostderr=true --grpc-gateway_opt paths=source_relative --grpc-gateway_opt generate_unbound_methods=true --openapiv2_out . --openapiv2_opt generate_unbound_methods=true  --openapiv2_opt logtostderr=true  api/mindexd/pb/mindexd.proto
