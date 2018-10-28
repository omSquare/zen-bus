all: test

test:
	go test -v ./...

# run pkg/zbus tests in a dockerized Linux container
docker-test:
	docker run --rm -v `pwd`:/go/src/github.com/omSquare/zen-bus golang:1.11 \
		go test github.com/omSquare/zen-bus/...
