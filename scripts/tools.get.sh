#!/usr/bin/env bash

#tmp until Go has better support for installing tools with modules
export GO111MODULE=off

go get -u golang.org/x/tools/go/analysis/passes/shadow/cmd/shadow

if ! which golint > /dev/null; then
    echo "Tools - Installing golint..."
    go get -u golang.org/x/lint/golint
fi

if ! which license-bill-of-materials > /dev/null; then
    echo "Tools - Installing bom tool..."
	go get -v -u github.com/cloudimmunity/license-bill-of-materials
fi
