#
# Makefile
#

# ldflags variables to update --version
# short commit hash
COMMIT :=$(shell /usr/bin/git describe --always)
DATE :=$(shell /bin/date -u +"%Y-%m-%d-%H:%M")
BINARY :=goFlipFlop

all: clean build version

test:
	go test

clean:
	[ -f ${BINARY} ] && /bin/rm -rf ./${BINARY} || true

build:
	go build -ldflags "-X main.commit=${COMMIT} -X main.date=${DATE}" -o ./${BINARY} ./${BINARY}.go
#CGO_ENABLED=0 go build -ldflags "-X main.commit=${COMMIT} -X main.date=${DATE}" -o ./${BINARY} ./${BINARY}.go

# https://words.filippo.io/shrink-your-go-binaries-with-this-one-weird-trick/
buildsmall:
	go build -ldflags "-s -w -X main.commit=${COMMIT} -X main.date=${DATE}" -o ./${BINARY} ./${BINARY}.go

shrink:
	upx --brute ./${BINARY}

version:
	./${BINARY} --version

curl:
	curl --silent http://localhost:9901/metrics

run:
	./${BINARY} --intf enp1s0.10 --frequency 10s

runlocal:
	./${BINARY} --intf virbr0 --frequency 10s
#