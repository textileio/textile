textile:
	go install ./...

hub-up:
	docker-compose -f cmd/hubd/docker-compose-dev.yml up --build

hub-stop:
	docker-compose -f cmd/hubd/docker-compose-dev.yml stop

hub-clean:
	docker-compose -f cmd/hubd/docker-compose-dev.yml down -v --remove-orphans

buckets-up:
	docker-compose -f cmd/buckd/docker-compose-dev.yml up --build

buckets-stop:
	docker-compose -f cmd/buckd/docker-compose-dev.yml stop

buckets-clean:
	docker-compose -f cmd/buckd/docker-compose-dev.yml down -v --remove-orphans
