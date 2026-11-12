go fmt ./...
echo "refactored will now build"


go build ./...
echo "build will now test"

go test ./...
echo "tested will now run"

go run ./cmd/shrinkr