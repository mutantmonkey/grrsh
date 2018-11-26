#!/bin/sh
m4 config.m4 client/config.go.in > client/config.go
m4 config.m4 server/config.go.in > server/config.go

mkdir -p out
go build -ldflags="-s -w" -o out/client mutantmonkey.in/code/grrsh/client
go build -ldflags="-s -w" -o out/server mutantmonkey.in/code/grrsh/server
