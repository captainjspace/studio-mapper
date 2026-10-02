#!/usr/bin/env zsh
#set up
#go mod init studio/engine

# build 
go build -o studio-map ./cmd/studio-map

#test all
go test -v ./...
go test ./...

