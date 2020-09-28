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
	$(call gen_release_files,./cmd/hub,hub,"linux/amd64 linux/386 linux/arm darwin/amd64 windows/amd64")
.PHONY: build-hub-release

build-hubd-release: $(GOX) $(GOVVV) $(GOMPLATE)
	$(call gen_release_files,./cmd/hubd,hubd,"linux/amd64 linux/386 linux/arm darwin/amd64 windows/amd64")
.PHONY: build-hubd-release

build-buck-release: $(GOX) $(GOVVV) $(GOMPLATE)
	$(call gen_release_files,./cmd/buck,buck,"linux/amd64 linux/386 linux/arm darwin/amd64 windows/amd64")
.PHONY: build-buck-release

build-buckd-release: $(GOX) $(GOVVV) $(GOMPLATE)
	$(call gen_release_files,./cmd/buckd,buckd,"linux/amd64 linux/386 linux/arm darwin/amd64 windows/amd64")
.PHONY: build-buckd-release

build-releases: build-hub-release build-hubd-release build-buck-release build-buckd-release
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
