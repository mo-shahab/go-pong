.PHONY: build watch start all clean

all: build start

build:
	tsc --project ./client/tsconfig.json

watch:
	tsc --project tsconfig.json --watch

start:
	cd ./server && go run main.go

go_fmt:
	cd ./server && go fmt .

run: build start

clean:
	rm -rf ./client/scripts/*.js
	rm -rf ./client/scripts/*.js.map
