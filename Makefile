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
