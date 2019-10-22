#!/usr/bin/env bash

if ! which govendor > /dev/null; then
    echo "Tools: No govendor. Installing..."
    go get -u github.com/kardianos/govendor
fi

if ! which golint > /dev/null; then
    echo "Tools: No golint. Installing...."
    go get -u github.com/golang/lint/golint
fi


