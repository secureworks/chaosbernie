# Usage:
# make        # compile all binary
# make clean  # remove ALL binaries and objects

.PHONY = all clean

mkfile_path := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
bin_path := ${mkfile_path}/bin
BUILD_CMD := GOOS=linux GOARCH=amd64 go build -ldflags="-s -w"

all: build

build: clean
	mkdir -p ${bin_path}
	(cd client && ${BUILD_CMD} -o ${bin_path}/client main.go)
	(cd server && ${BUILD_CMD} -o ${bin_path}/server main.go)

clean:
	rm -f ${bin_path}/server
	rm -f ${bin_path}/client

