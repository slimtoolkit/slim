## mangos <img src=mangos.jpg align=right>


[![Linux Status](https://img.shields.io/travis/go-mangos/mangos.svg?label=linux)](https://travis-ci.org/go-mangos/mangos)
[![Windows Status](https://img.shields.io/appveyor/ci/gdamore/mangos.svg?label=windows)](https://ci.appveyor.com/project/gdamore/mangos)
[![Apache License](https://img.shields.io/badge/license-APACHE2-blue.svg)](https://github.com/go-mangos/mangos/blob/master/LICENSE)
[![Gitter](https://img.shields.io/badge/gitter-join-brightgreen.svg)](https://gitter.im/gdamore/mangos)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://godoc.org/github.com/go-mangos/mangos)
[![Go Report Card](https://goreportcard.com/badge/github.com/go-mangos/mangos)](https://goreportcard.com/report/github.com/go-mangos/mangos)

package mangos is an implementation in pure Go of the SP ("Scalable Protocols")
protocols.  This makes heavy use of go channels, internally, but it can operate
on systems that lack support for cgo.  It has no external dependencies.

The reference implementation of the SP protocols is available as
[nanomsg](http://www.nanomsg.org)
 
The design is intended to make it easy to add new transports with almost trivial
effort, as well as new topologies ("protocols" in SP terminology.)

At present, all of the Req/Rep, Pub/Sub, Pair, Bus, Push/Pull, and
Surveyor/Respondent patterns are supported.

Additionally, there is an experimental new pattern called STAR available.  This
pattern is like Bus, except that the messages are delivered not just to
immediate peers, but to all members of the topology.  Developers must be careful
not to create cycles in their network when using this pattern, otherwise
infinite loops can occur.

Supported transports include TCP, inproc, IPC, Websocket, Websocket/TLS and TLS.
Use addresses of the form "tls+tcp://<host>:<port>" to access TLS.
Note that ipc:// is not supported on Windows (by either this or the reference
implementation.)  Forcing the local TCP port in Dial is not supported yet (this
is rarely useful).

Basic interoperability with nanomsg has been verified (you can do so yourself
with nanocat and macat) for all protocols nanomsg supports.  Additionally there
are a number of projects that use the two products together.

If you find this useful, I would appreciate knowing about it.  I can be reached
via my email address, garrett -at- damore -dot- org

## Installing

### Using *go get*

    $ go get -u github.com/go-mangos/mangos

After this command *mangos* is ready to use. Its source will be in:

    $GOPATH/src/pkg/github.com/go-mangos/mangos

You can use `go get -u -a` to update all installed packages.

## Documentation

For docs, see http://godoc.org/github.com/go-mangos/mangos or run:

    $ godoc github.com/go-mangos/mangos

## Testing

This package supports internal self tests, which can be run in
the idiomatic Go way, although it uses a separate test sub-package:

    $ go test github.com/go-mangos/mangos/test

There are also internal benchmarks available:

    $ go test -bench=. github.com/go-mangos/mangos/test

## Examples

Some examples are posted in the directories under examples/
The examples are rewrites (in Go) of Tim Dysinger's libnanomsg examples,
which are located at

http://tim.dysinger.net/posts/2013-09-16-getting-started-with-nanomsg.html

godoc in the example directories will yield information about how to run
each example program.

Enjoy!

Copyright 2015 The Mangos Authors
