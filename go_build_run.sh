#!/usr/bin/env bash

set -euo pipefail

echo "==> Checking Go formatting..."
unformatted=$(gofmt -l .)

if [[ -n "$unformatted" ]]; then
    echo "The following files need formatting:"
    echo "$unformatted"
    exit 1
fi

echo "==> Verifying dependencies..."
go mod verify

echo "==> Running go vet..."
go vet ./...

echo "==> Running tests..."
go test -race -cover ./...

echo "==> Building Shrinkr..."
go build -v ./...

echo "==> All checks passed!"

echo "==> Starting Shrinkr..."
go run ./cmd/shrinkr