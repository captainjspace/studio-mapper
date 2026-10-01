#!/usr/bin/env zsh
set -e

echo "Initializing Go module if missing..."
if [ ! -f "go.mod" ]; then
    go mod init studio/engine
fi

echo "Running full test suite..."
go test -v ./...

echo "Compiling binary..."
go build -o studio-map .

echo "Installing binary to local user path (~/.local/bin)..."
mkdir -p ~/.local/bin
cp studio-map ~/.local/bin/studio-map

echo "Build and deployment complete. Run 'studio-map' from anywhere."

