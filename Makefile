include .bingo/Variables.mk

.DEFAULT_GOAL=textile

textile:
	go install ./...

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
