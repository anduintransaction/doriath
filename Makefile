DIST=./dist
BIN=doriath
OS_MAC=darwin
ARCH_MAC=amd64
OS_LINUX=linux
ARCH_LINUX=amd64

build:
	go build

install:
	go install

test: .test-buildtree

clean:
	rm -rf doriath ${DIST} ${GOPATH}/bin/doriath

dist: clean build .dist-prepare .dist-mac .dist-linux

.dist-prepare:
	rm -rf ${DIST}
	mkdir -p ${DIST}

.dist-mac:
	GOOS=${OS_MAC} GOARCH=${ARCH_MAC} go build -o ${DIST}/${BIN} && \
	cd ${DIST} && \
	tar czf ${BIN}-`../doriath version`-${OS_MAC}-${ARCH_MAC}.tar.gz ${BIN} && \
	rm ${BIN} && \
	cd ..

.dist-linux:
	GOOS=${OS_LINUX} GOARCH=${ARCH_LINUX} go build -o ${DIST}/${BIN} && \
	cd ${DIST} && \
	tar czf ${BIN}-`../doriath version`-${OS_LINUX}-${ARCH_LINUX}.tar.gz ${BIN} && \
	rm ${BIN} && \
	cd ..

.test-buildtree:
	go test ./buildtree -v
