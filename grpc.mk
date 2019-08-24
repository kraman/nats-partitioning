MAKEFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
CWD := $(shell dirname $(MAKEFILE_PATH))
GOPATH := $(CWD)/build
GO111MODULE := on
PROTOC_VERSION := 3.8.0

export GOPATH
export GO111MODULE

define deref
$(shell GOPATH=$(GOPATH) go list -f "{{ .Dir }}" -m $(1))
endef

build/bin/protoc-gen-grpc-gateway:
	@go get -u github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway

build/bin/protoc-gen-swagger:
	@go get -u github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger

build/bin/protoc-gen-gogoslick:
	@go get -u github.com/gogo/protobuf/protoc-gen-gogoslick

build/protoc/bin/protoc:
	@mkdir -p build/tmp build/protoc
	$(eval OS := $(shell if uname -s | grep Darwin > /dev/null; then echo osx ; else echo linux ; fi))
	@curl -s -o build/tmp/protoc.zip -L https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/protoc-$(PROTOC_VERSION)-$(OS)-$(shell uname -m).zip
	@(cd build/protoc; unzip -qq ../tmp/protoc.zip)

%.pb.go %.pb.gw.go %.swagger.json: %.proto build/protoc/bin/protoc build/bin/protoc-gen-gogoslick build/bin/protoc-gen-swagger build/bin/protoc-gen-grpc-gateway
	PATH=$(PATH):build/bin:build/protoc/bin DEBUG=1 protoc \
		-I $(call deref, github.com/grpc-ecosystem/grpc-gateway)/third_party/googleapis \
		-I $(call deref, github.com/gogo/protobuf) \
		-I build/protoc/include \
		-I $(@D) \
		--gogoslick_out=plugins=grpc,\
Mgoogle/protobuf/any.proto=github.com/gogo/protobuf/types,\
Mgoogle/protobuf/duration.proto=github.com/gogo/protobuf/types,\
Mgoogle/protobuf/struct.proto=github.com/gogo/protobuf/types,\
Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types,\
Mgoogle/protobuf/wrappers.proto=github.com/gogo/protobuf/types:$(@D) \
		--grpc-gateway_out=logtostderr=true:$(@D) \
		--swagger_out=logtostderr=true:$(@D) \
		$<

clean-proto:
	@find . -name *.pb.go | grep -v ./build | xargs rm
	@find . -name *.pb.gw.go | grep -v ./build | xargs rm
	@find . -name *.swagger.json | grep -v ./build | xargs rm