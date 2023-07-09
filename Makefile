.PHONY: windows linux

GO111MODULE = on
GOARCH = amd64

all: windows linux

windows: windows_standalone windows_communicator

linux: linux_standalone linux_communicator build_deb

windows_standalone:
	set GOOS = windows
	@go build -o build/standalone.exe -tags=webui github.com/vaizmanai/clvisit/cmd/communicator

windows_communicator:
	set GOOS = windows
	@go build -o build/communicator.exe github.com/vaizmanai/clvisit/cmd/communicator

linux_standalone:
	set GOOS = linux
	@go build -o build/standalone -tags=webui github.com/vaizmanai/clvisit/cmd/communicator

linux_communicator:
	set GOOS = linux
	@go build -o build/communicator github.com/vaizmanai/clvisit/cmd/communicator

build_deb: linux_standalone
	@cp build/standalone init/deb/opt/remote-admin/admin
	fakeroot dpkg-deb --build init/deb build
