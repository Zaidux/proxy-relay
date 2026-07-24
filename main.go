package main

import (
	"fmt"
	"os"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, `proxy-relay %s — lightweight IP relay

Usage:
  proxy-relay relay  [--port=9000]            Start the matchmaker on a public VPS
  proxy-relay serve  [--relay=host:port]       Share your residential IP (generates a key)
  proxy-relay connect <key> [--relay=host:port] [--proxy=:8080]
                                               Use another device's IP as your proxy

Examples:
  # On your home machine (User A):
  proxy-relay serve --relay=vps.example.com:9000
  # Prints: "Key: apple-tree-frog-lamp"

  # On the VPS where riciplay runs (User B):
  proxy-relay connect apple-tree-frog-lamp --relay=localhost:9000
  # Sets up local HTTP proxy on :8080

  # Then in riciplay config:
  # {"proxy": "http://localhost:8080"}
`, version)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "relay":
		runRelay(os.Args[2:])
	case "serve":
		runServe(os.Args[2:])
	case "connect":
		runConnect(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
