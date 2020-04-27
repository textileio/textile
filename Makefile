textile:
	go install ./...

local-up:
	docker-compose -f docker-compose-dev.yml up

local-stop:
	docker-compose stop

local-clean:
	docker-compose down -v
