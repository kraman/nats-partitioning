
MAKEFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
CWD := $(shell dirname $(MAKEFILE_PATH))
export GOPATH=$(CWD)/build
export GO111MODULE=on

start-nats: build/run/.nats-server
stop-nats:
ifneq (,$(wildcard ./build/run/.nats-server))
	@kill $(shell cat build/run/.nats-server)
	@-rm -f build/run/.nats-server
endif

build/run/.nats-server: build/bin/nats-server
	@mkdir -p $(@D)
	@build/bin/nats-server -P $@ &

build/bin/nats-server:
	@mkdir -p $(@D)
	@go get github.com/nats-io/nats-server/v2 2>&1

.PHONY: start-nats stop-nats