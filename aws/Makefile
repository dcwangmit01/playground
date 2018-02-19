.PHONY: *

all: build

dep:
	dep ensure

build:
	mkdir -p build/
	go build -o build/create-ec2 create-ec2.go
	go build -o build/destroy-ec2 destroy-ec2.go

clean:
	rm -rf vendor
	rm -rf build/

test:
	go test

coverage:
	go test --coverprofile cover.out
	go tool cover -html=cover.out -o cover.html
