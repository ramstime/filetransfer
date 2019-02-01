include Makefile.include

.PHONY:all
all: build 

.PHONY:build
build:
	@echo "building the metadatastoreclient"
	@echo "gopath being used is ${GOPATH}"
	${GOROOT}/bin/go install
	#${GOROOT}/bin/go build


.PHONY:clean
clean:
	@echo "Removing test coverage, rpm directories: ${TEST_PARENT_DIR} ${RPMPATH}/BUILD ${RPMPATH}/RPMS ..."
	@rm -vrf ${TEST_PARENT_DIR} ${RPMPATH}/BUILD ${RPMPATH}/RPMS


