#!/usr/bin/env bash

set -euo pipefail

echo "==> formatting..."
go fmt ./...

echo "==> Verifying dependencies..."
go mod verify

echo "==> Running go vet..."
go vet ./...

echo "==> Running tests..."
go test -race -cover ./...

echo "==> Building Shrinkr..."
go build -v ./...

echo "==> All checks passed!"
