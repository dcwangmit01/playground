.PHONY: *

all: build

install:
	glide install

build:
	mkdir -p bin/
	go build -o bin/create-ec2 create-ec2.go
	go build -o bin/destroy-ec2 destroy-ec2.go

clean:
	rm -rf vendor
	rm -rf bin/

test:
	go test

coverage:
	go test --coverprofile cover.out
	go tool cover -html=cover.out -o cover.html
