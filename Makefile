.PHONY: build_communicator build_standalone

export GO111MODULE=on

all: build_communicator build_standalone

build_communicator:
	@go build -o build/communicator.exe github.com/vaizmanai/clvisit/cmd/communicator

build_standalone:
	@go build -o build/standalone.exe -tags=webui github.com/vaizmanai/clvisit/cmd/communicator
