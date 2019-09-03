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

.PHONY: start-nats stop-nats start-stan stop-stan