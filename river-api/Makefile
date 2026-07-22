.PHONY: build vet test tidy swagger run

build:
	go build ./...

vet:
	go vet ./...

test:
	go test ./...

tidy:
	go mod tidy

# Regenerate the OpenAPI spec under docs/ from the swag annotations on
# main.go + handlers. Requires `go install github.com/swaggo/swag/cmd/swag@latest`.
# `--parseDependency --parseInternal` so models referenced from internal
# packages (handlers, services, models) are inlined into the spec.
swagger:
	swag init \
		--generalInfo main.go \
		--dir cmd/server,internal/handlers,internal/services,internal/models \
		--output ./docs \
		--parseDependency \
		--parseInternal \
		--parseDepth 3

run:
	go run ./cmd/server
