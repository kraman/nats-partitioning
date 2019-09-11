
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

build: service/service.pb.go #lib/manager/nats/partitions.pb.go lib/manager/nats/es/event.pb.go

clean: clean-proto

cleaner:
	@chmod -R u+rwx build/pkg/mod 2>/dev/null || true
	@rm -rf build

include grpc.mk
include nats.mk
include etcd.mk

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
