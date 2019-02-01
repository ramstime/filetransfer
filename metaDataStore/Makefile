include Makefile.include

.PHONY:all
all: build 

.PHONY:build
build:
	@echo "building the metadatastore"
	@echo "gopath being used is ${GOPATH}"
	${GOROOT}/bin/go install
	#${GOROOT}/bin/go build

.PHONY:clean
clean:
	@echo "Removing test coverage, rpm directories: ${GOBIN}/BUILD ..."
	@rm -vrf ${GOBIN}/BUILD ${GOBIN}/RPMS


