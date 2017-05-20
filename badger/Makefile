.PHONY: *

all: release

install:
	glide install

clean:
	rm -rf vendor
	rm -rf build
	find . -name cover.out -o -name cover.html | xargs rm -rf

release:
	mkdir -p build
	env GOOS=linux GOARCH=amd64 go build -o build/tcp-server tcp-server/main.go

lint:
	golint tcp-server/

test:
	go test ./tcp-server/

coverage:
	go test ./tcp-server/ --coverprofile ./tcp-server/cover.out
	go tool cover -html=./tcp-server/cover.out -o ./tcp-server/cover.html
