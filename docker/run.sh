#! /bin/sh
set -e

OLDPWD=`pwd`
export GOARCH="amd64"
export GOOS="linux"
export CGO_ENABLED=0

cd ..

go build -v -o docker/dist/go-url-shortener

cd docker

docker build --no-cache -t go-url-shortener .

cd "$OLDDIR"
