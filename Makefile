
MAKEFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
CWD := $(shell dirname $(MAKEFILE_PATH))
export GOPATH=$(CWD)/build
export GO111MODULE=on

define goget
$(shell go get $(1) 2>&1 >/dev/null)
endef

define deref
$(shell go list -f "{{ .Dir }}" -m $(1))
endef

build: lib/partitions/partitions.pb.go lib/es/event.pb.go

start-stan: build/run/.nats-streaming-server
stop-stan:
ifneq (,$(wildcard ./build/run/.nats-streaming-server))
	@kill $(shell cat ./build/run/.nats-streaming-server)
	@-rm -f build/run/.nats-streaming-server
endif

start-nats: build/run/.nats-server
stop-nats: stop-stan
ifneq (,$(wildcard ./build/run/.nats-server))
	@kill $(shell cat build/run/.nats-server)
	@-rm -f build/run/.nats-server
endif

build/run/.nats-server: build/bin/nats-server
	@mkdir -p $(@D)
	@build/bin/nats-server -P $@ &
	@sleep 2

build/run/.nats-streaming-server: build/bin/nats-streaming-server build/run/.nats-server
	@mkdir -p $(@D)
	@build/bin/nats-streaming-server --cid nats -ns http://127.0.0.1:4222 & echo $$! > $@;

build/bin/nats-server:
	@mkdir -p $(@D)
	@go get github.com/nats-io/nats-server/v2

build/bin/nats-streaming-server: build/bin/nats-server
	@mkdir -p $(@D)
	@go get github.com/nats-io/nats-streaming-server 

clean: clean-proto

cleaner:
	@chmod -R u+rwx build/pkg/mod 2>/dev/null || true
	@rm -rf build

include grpc.mk

build/run/.doc:
	@mkdir -p $(@D)
	@godoc -http=localhost:6060 & echo $$! > $@;
	@open http://localhost:6060

stop-doc:
ifneq (,$(wildcard ./build/run/.doc))
	@kill $(shell cat build/run/.doc)
	@-rm -f build/run/.doc
endif

doc: build/run/.doc

.PHONY: start-nats stop-nats start-stan stop-stan clean cleaner doc
