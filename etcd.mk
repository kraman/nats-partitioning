
ETCD_VER=v3.3.15

# choose either URL
GOOGLE_URL=https://storage.googleapis.com/etcd
GITHUB_URL=https://github.com/etcd-io/etcd/releases/download
DOWNLOAD_URL=${GOOGLE_URL}

build/bin/etcd:
	@mkdir -p $(@D)
ifeq ($(shell uname -s),Darwin)
	@mkdir -p build/tmp
	@curl -sL ${DOWNLOAD_URL}/${ETCD_VER}/etcd-${ETCD_VER}-darwin-amd64.zip -o build/tmp/etcd-${ETCD_VER}-darwin-amd64.zip
	@unzip build/tmp/etcd-${ETCD_VER}-darwin-amd64.zip -d build/tmp > /dev/null && rm -f build/tmp/etcd-${ETCD_VER}-darwin-amd64.zip
	@mv build/tmp/etcd-${ETCD_VER}-darwin-amd64/etcd* $(@D) && rm -rf mv build/tmp/etcd-${ETCD_VER}-darwin-amd64
else
	@rm -f build/tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz
	@rm -rf build/tmp/etcd-download && mkdir -p build/tmp/etcd-download
	@curl -sL ${DOWNLOAD_URL}/${ETCD_VER}/etcd-${ETCD_VER}-$(OS)-amd64.tar.gz -o build/tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz
	@tar xzf build/tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz -C build/tmp/etcd-download --strip-components=1
	@rm -f build/tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz
	@cp build/tmp/etcd-download/etcd build/tmp/etcd-download/etcdctl $(@D)/
endif
	@$@ --version
	@ETCDCTL_API=3 build/bin/etcdctl version

start-etcd: build/run/.etcd-server
build/run/.etcd-server: build/bin/etcd
	@mkdir -p build/run/default.etcd
	@build/bin/etcd --data-dir build/run/default.etcd & echo $$! > $@;

stop-etcd:
ifneq (,$(wildcard ./build/run/.etcd-server))
	@kill $(shell cat build/run/.etcd-server)
	@-rm -rf build/run/.etcd-server build/run/default.etcd
endif