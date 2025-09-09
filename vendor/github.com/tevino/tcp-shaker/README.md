# TCP Checker :heartbeat:

[![Go Report Card](https://goreportcard.com/badge/github.com/tevino/tcp-shaker)](https://goreportcard.com/report/github.com/tevino/tcp-shaker)
[![GoDoc](https://godoc.org/github.com/tevino/tcp-shaker?status.svg)](https://godoc.org/github.com/tevino/tcp-shaker)
[![Build Status](https://travis-ci.org/tevino/tcp-shaker.svg?branch=master)](https://travis-ci.org/tevino/tcp-shaker)

This package is used to perform TCP handshake without ACK, which useful for TCP health checking.

HAProxy does this exactly the same, which is:

1. SYN
2. SYN-ACK
3. RST

This implementation has been running on tens of thousands of production servers for years.

## Why do I have to do this

In most cases when you establish a TCP connection(e.g. via `net.Dial`), these are the first three packets between the client and server([TCP three-way handshake][tcp-handshake]):

1. Client -> Server: SYN
2. Server -> Client: SYN-ACK
3. Client -> Server: ACK

**This package tries to avoid the last ACK when doing handshakes.**

By sending the last ACK, the connection is considered established.

However, as for TCP health checking the server could be considered alive right after it sends back SYN-ACK,

that renders the last ACK unnecessary or even harmful in some cases.

### Benefits

By avoiding the last ACK

1. Less packets better efficiency
2. The health checking is less obvious

The second one is essential because it bothers the server less.

This means the application level server will not notice the health checking traffic at all, **thus the act of health checking will not be
considered as some misbehavior of client.**

## Requirements

- Linux 2.4 or newer

There is a **fake implementation** for **non-Linux** platform which is equivalent to:

```go
conn, err := net.DialTimeout("tcp", addr, timeout)
conn.Close()
```

The reason for a fake implementation is that there is currently no way to perform an equivalent operation on certain platforms, specifically macOS. A fake implementation is a degradation on those platforms and ensures a successful compilation.


## Usage

### Quick start (recommended)

```go
import (
	tcpshaker "github.com/tevino/tcp-shaker"
)

// Get the global singleton Checker instance
checker := tcpshaker.DefaultChecker()


// Checking example.com
err := checker.CheckAddr("example.com:80", time.Second)
switch err {
case ErrTimeout:
	fmt.Println("Connect timed out after 1s")
case nil:
	fmt.Println("Connect succeeded")
default:
	fmt.Println("Error occurred while connecting: ", err)
}
```

### Manual initialization

For fine-grained control of the lifecycle of the `Checker`.

```
// Initializing the checker
// Only a single instance is needed and it's safe to use among goroutines.
checker := NewChecker()

ctx, stopChecker := context.WithCancel(context.Background())
defer stopChecker()
go func() {
	if err := checker.CheckingLoop(ctx); err != nil {
		fmt.Println("checking loop stopped due to fatal error: ", err)
	}
}()

<-checker.WaitReady()

// now the checker could be used as shown in the previous example.
```

### Command-line tool

A `tcp-checker` command-line tool is also available. It can be built with:

```bash
go build -o tcp-checker ./cmd/tcp-checker
```

Or installed directly using `go install`:

```bash
go install github.com/tevino/tcp-shaker/cmd/tcp-checker
```

Example usage:
```bash
# Check example.com:443 with a 2 seconds timeout
tcp-checker -a example.com:443 -t 2000
```

## Development & Contributing
See [CONTRIBUTING.md](./CONTRIBUTING.md) to learn how to contribute to the project.

## TODO

- [x] IPv6 support (Test environment needed, PRs are welcome)

## Special thanks to contributors

- @lujjjh Added zero linger support for non-Linux platform
- @jakubgs Fixed compatibility on Android
- @kirk91 Added support for IPv6
- @eos175 Added a global singleton for `Checker`

[tcp-handshake]: https://en.wikipedia.org/wiki/Handshaking#TCP_three-way_handshake
