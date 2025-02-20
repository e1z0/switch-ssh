APP_NAME := switch-ssh
SRC := $(wildcard src/*.go)

all: build

build:
	go build -o ${APP_NAME} ${SRC}
format:
	go fmt $(SRC)
release:
	./release.sh
