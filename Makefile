BINNAME=curso
export VERSION := $(shell git describe --exact-match --tags $(git log -n1 --pretty='%h') || git rev-parse --verify --short HEAD || echo ${VERSION})
export COMMIT := $(shell git rev-parse --verify --short HEAD)
export DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

all: clean dependencies build

clean:
	rm -vf bin/*

build: build-windows build-linux build-macos

build-macos: build-macos-amd64 # build-macos-i386

build-linux: build-linux-amd64 # build-linux-i386

build-windows: build-windows-amd64 # build-windows-i386

build-macos-amd64:
	GOOS=darwin GOARCH=amd64    buffalo build --ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" -o bin/${BINNAME}-darwin-amd64 .

build-macos-i386:
	GOOS=darwin GOARCH=386     buffalo build --ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" -o bin/${BINNAME}-darwin-i386 .

build-linux-amd64:
	GOOS=linux GOARCH=amd64    buffalo build --ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" -o bin/${BINNAME}-linux-amd64 .

build-linux-i386:
	GOOS=linux GOARCH=386      buffalo build --ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" -o bin/${BINNAME}-linux-i386 .

build-windows-amd64:
	GOOS=windows GOARCH=amd64  buffalo build --ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" -o bin/${BINNAME}-windows-amd64.exe .

build-windows-i386:
	GOOS=windows GOARCH=386    buffalo build --ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" -o bin/${BINNAME}-windows-i386.exe .

dependencies:
	go mod verify
	go mod tidy

sign:
	gpg --detach-sign --digest-algo SHA512 --no-tty --batch --output bin/${BINNAME}-darwin-amd64.sig 				bin/${BINNAME}-darwin-amd64
	gpg --detach-sign --digest-algo SHA512 --no-tty --batch --output bin/${BINNAME}-darwin-i386.sig 				bin/${BINNAME}-darwin-i386
	gpg --detach-sign --digest-algo SHA512 --no-tty --batch --output bin/${BINNAME}-linux-amd64.sig 				bin/${BINNAME}-linux-amd64
	gpg --detach-sign --digest-algo SHA512 --no-tty --batch --output bin/${BINNAME}-linux-i386.sig 					bin/${BINNAME}-linux-i386
	gpg --detach-sign --digest-algo SHA512 --no-tty --batch --output bin/${BINNAME}-windows-amd64.exe.sig 	bin/${BINNAME}-windows-amd64.exe
	gpg --detach-sign --digest-algo SHA512 --no-tty --batch --output bin/${BINNAME}-windows-i386.exe.sig 		bin/${BINNAME}-windows-i386.exe

test:
	go test ./...

verify:
	gpg --verify bin/${BINNAME}-darwin-amd64.sig 				bin/${BINNAME}-darwin-amd64
	gpg --verify bin/${BINNAME}-darwin-i386.sig 				bin/${BINNAME}-darwin-i386
	gpg --verify bin/${BINNAME}-linux-amd64.sig 				bin/${BINNAME}-linux-amd64
	gpg --verify bin/${BINNAME}-linux-i386.sig 					bin/${BINNAME}-linux-i386
	gpg --verify bin/${BINNAME}-windows-amd64.exe.sig 	bin/${BINNAME}-windows-amd64.exe
	gpg --verify bin/${BINNAME}-windows-i386.exe.sig 		bin/${BINNAME}-windows-i386.exe