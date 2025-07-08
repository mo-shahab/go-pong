.PHONY: build watch start all clean proto proto-go proto-ts install-proto-deps
all: proto build start

# Install protobuf dependencies
install-proto-deps:
	@echo "Installing protobuf dependencies..."
	@which protoc > /dev/null || (echo "Please install protoc first: https://grpc.io/docs/protoc-installation/" && exit 1)
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	cd ./client && pnpm install --save-dev ts-proto
	@echo "Protobuf dependencies installed!"

# Generate protobuf files for both Go and TypeScript
proto: proto-go proto-ts

# Generate Go protobuf files
proto-go:
	@echo "Generating Go protobuf files..."
	@mkdir -p ./server/proto
	protoc --plugin=$(HOME)/go/bin/protoc-gen-go \
		--go_out=./server/ \
		--go_opt=paths=source_relative \
		./proto/gopong.proto
	@echo "Go protobuf files generated!"

# Generate TypeScript protobuf files
proto-ts:
	@echo "Generating TypeScript protobuf files..."
	@mkdir -p ./client/src/proto
	protoc \
		--plugin=./client/node_modules/.bin/protoc-gen-ts_proto \
		--ts_proto_out=./client/src/proto \
		--ts_proto_opt=esModuleInterop=true \
		--ts_proto_opt=forceLong=string \
		--ts_proto_opt=useOptionals=messages \
		--ts_proto_opt=outputJsonMethods=true \
		--ts_proto_opt=useESModules=true \
		--ts_proto_opt=useBufbuild=true \
		--proto_path=./proto \
		./proto/gopong.proto
	@echo "TypeScript protobuf files generated!"

# Build TypeScript
build: proto-ts
	cd ./client && pnpm run build

# Start Go server
start-go:
	cd ./server && go run main.go

start-ts:
	cd ./client && pnpm run dev

start:
	npx concurrently "cd server && air" "cd client && pnpm run dev"

# Format Go code
go_fmt:
	cd ./server && go fmt ./...

# Run everything
run: proto build start

# Clean generated files
clean:
	rm -rf ./client/scripts/*.js
	rm -rf ./client/scripts/*.js.map
	rm -rf ./server/proto/*
	rm -rf ./client/src/proto/*
	@echo "Cleaned generated files!"

# Clean everything including dependencies
clean-all: clean
	cd ./client && rm -rf node_modules
	@echo "Cleaned everything!"

# Help
help:
	@echo "Available targets:"
	@echo "	all			 - Generate protobuf, build TypeScript, and start server"
	@echo "	install-proto-deps - Install protobuf compilation dependencies"
	@echo "	proto			 - Generate protobuf files for both Go and TypeScript"
	@echo "	proto-go		 - Generate Go protobuf files only"
	@echo "	proto-ts		 - Generate TypeScript protobuf files only"
	@echo "	build			 - Build TypeScript files"
	@echo "	watch			 - Watch and rebuild TypeScript files"
	@echo "	start			 - Start Go server"
	@echo "	run			 - Generate protobuf, build, and start"
	@echo "	clean			 - Clean generated files"
	@echo "	clean-all		 - Clean everything including dependencies"
	@echo "	go_fmt			 - Format Go code"
