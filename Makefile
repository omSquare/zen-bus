# Copyright (c) 2018 omSquare s.r.o.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

.PHONY: all test build docker-test

all: test

test:
	go test -v ./...

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm go build -o build/zbus.arm github.com/omSquare/zen-bus/cmd/zbus
	CGO_ENABLED=0 GOOS=linux GOARCH=mipsle go build -o build/zbus.mipsle github.com/omSquare/zen-bus/cmd/zbus

# run pkg/zbus tests in a dockerized Linux container
docker-test:
	docker run --rm -v `pwd`:/go/src/github.com/omSquare/zen-bus golang:1.11 \
		go test github.com/omSquare/zen-bus/...
