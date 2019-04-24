# UnpackIt

[![GoDoc](https://godoc.org/github.com/c4milo/unpackit?status.svg)](https://godoc.org/github.com/c4milo/unpackit)
[![Build Status](https://travis-ci.org/c4milo/unpackit.svg?branch=master)](https://travis-ci.org/c4milo/unpackit)

This Go library allows you to easily unpack the following files using magic numbers:

* tar.gz
* tar.bzip2
* tar.xz
* zip
* tar

## Usage

Unpack a file:

```go
    file, _ := os.Open(test.filepath)
    destPath, err := unpackit.Unpack(file, tempDir)
```

Unpack a stream (such as a http.Response):

```go
    res, err := http.Get(url)
    destPath, err := unpackit.Unpack(res.Body, tempDir)
```

