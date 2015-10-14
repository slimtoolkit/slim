#!/usr/bin/env bash

here="$(dirname "$BASH_SOURCE")"
cd $here

mvn -Dmaven.test.skip=true package



