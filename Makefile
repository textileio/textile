textile:
	go install ./...

docker:
	$(eval VERSION := $(shell git --no-pager describe --abbrev=0 --tags --always))
	docker build -t textiled:$(VERSION:v%=%) .