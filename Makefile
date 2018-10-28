# run pkg/zbus tests in a dockerized Linux container
test:
	docker run --rm -v `pwd`:/go/src/github.com/omSquare/zen-bus golang:1.11 \
		go test github.com/omSquare/zen-bus/pkg/zbus
