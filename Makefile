test: .test-buildtree
.test-buildtree:
	go test ./buildtree -v

install:
	go install

build: test
	goreleaser build --snapshot --rm-dist
