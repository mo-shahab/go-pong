.PHONY: build watch start all clean

all: build start

build:
	tsc --project ./client/tsconfig.json

watch:
	tsc --project tsconfig.json --watch

start:
	cd ./server && go run main.go

run: build start

clean:
	rm -rf ./client/scripts/*.js
	rm -rf ./client/scripts/*.js.map
