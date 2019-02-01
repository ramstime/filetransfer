include Makefile.include

.PHONY: all
all:MetaDataStore

.PHONY:MetaDataStore
MetaDataStore:
	@echo "building protofile for metadatastore"
	protoc -I=${PROJECT} --proto_path=${PROJECT} --go_out=plugins=grpc:${PROJECT}/ ${PROJECT}/MetaDataStore/MetaDataStore.proto
