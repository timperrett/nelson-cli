
TRAVIS_BUILD_NUMBER ?= dev
CLI_FEATURE_VERSION ?= 1.0
CLI_VERSION ?= ${CLI_FEATURE_VERSION}.${TRAVIS_BUILD_NUMBER}
# if not set, then we're doing local development
# as this will be set by the travis matrix for realz
TARGET_PLATFORM ?= darwin
TARGET_ARCH ?= amd64
TAR_NAME = nelson-${TARGET_PLATFORM}-${TARGET_ARCH}-${CLI_VERSION}.tar.gz

install:
	go get github.com/constabulary/gb/...

install-dev: install
	go get github.com/codeskyblue/fswatch

release: format test package

compile: format
	GOOS=${TARGET_PLATFORM} GOARCH=amd64 CGO_ENABLED=0 gb build -ldflags "-X main.globalBuildVersion=${CLI_VERSION}"

watch:
	fswatch

test: compile
	gb test -v

package: test
	mkdir -p target && \
	mv bin/nelson-${TARGET_PLATFORM}-amd64 ./nelson && \
	tar -zcvf ${TAR_NAME} nelson && \
	rm nelson && \
	sha1sum ${TAR_NAME} > ${TAR_NAME}.sha1 && \
	shasum -c ${TAR_NAME}.sha1 && \
	mv ${TAR_NAME} target/${TAR_NAME} && \
	mv ${TAR_NAME}.sha1 target/${TAR_NAME}.sha1

format:
	go fmt src/github.com/getnelson/nelson/*.go

clean:
	rm -rf bin && \
	rm -rf pkg

tar:
	echo ${TAR_NAME}
