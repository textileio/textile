textile:
	go install ./...

local-up:
	docker-compose -f docker-compose-dev.yml up --build

local-stop:
	docker-compose stop

local-clean:
	docker-compose down -v --remove-orphans

pow-up: local-clean
	rm -rf ~/.textile
	rm -rf .textile
	docker-compose -f docker-compose-pow.yml up --build

