package main

import (
	"bufio"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

var wordList = []string{
	"alpha", "bravo", "delta", "echo", "foxtrot", "golf", "hotel",
	"juliet", "kilo", "lima", "mike", "november", "oscar", "papa",
	"quebec", "romeo", "sierra", "tango", "uniform", "victor",
	"whiskey", "xray", "yankee", "zulu", "amber", "azure", "blaze",
	"cobalt", "coral", "crimson", "ember", "frost", "hazel", "indigo",
	"ivory", "jade", "jasper", "magenta", "maroon", "moss", "olive",
	"onyx", "opal", "pearl", "plum", "ruby", "rust", "sage", "scarlet",
	"shadow", "silver", "slate", "steel", "storm", "teal", "topaz",
	"violet", "willow", "apple", "birch", "cedar", "cherry", "clover",
	"fern", "flame", "forest", "hawk", "iris", "ivy", "lotus", "maple",
	"oak", "pine", "raven", "river", "robin", "sky", "sparrow", "stone",
	"sunset", "thunder", "wolf", "wren", "arrow", "bolt", "bridge",
	"cloud", "coast", "cove", "dawn", "drift", "field", "flame",
	"glacier", "glen", "grove", "harbor", "haven", "hollow", "horizon",
	"island", "lagoon", "lake", "leaf", "light", "meadow", "mist",
	"moon", "morning", "mountain", "night", "ocean", "path", "peak",
	"pine", "pond", "prairie", "rain", "reef", "ridge", "shore",
	"spring", "star", "stream", "summit", "surf", "tide", "trail",
	"valley", "wave", "wind", "wing",
}

func generateKey() string {
	b := make([]byte, 4)
	rand.Read(b)
	w1 := wordList[int(b[0])%len(wordList)]
	w2 := wordList[int(b[1])%len(wordList)]
	w3 := wordList[int(b[2])%len(wordList)]
	w4 := wordList[int(b[3])%len(wordList)]
	return fmt.Sprintf("%s-%s-%s-%s", w1, w2, w3, w4)
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	relayAddr := fs.String("relay", "localhost:9000", "Relay server address")
	key := fs.String("key", "", "Custom key (auto-generated if empty)")
	fs.Parse(args)

	if *key == "" {
		*key = generateKey()
	}

	log.Printf("Connecting to relay: %s", *relayAddr)
	log.Printf("Key: %s", *key)
	log.Printf("")
	log.Printf("═══════════════════════════════════════════")
	log.Printf("  Share this key with User B:")
	log.Printf("  %s", *key)
	log.Printf("═══════════════════════════════════════════")
	log.Printf("")

	for {
		err := serveOnce(*relayAddr, *key)
		if err != nil {
			log.Printf("serve: %v — reconnecting in 5s", err)
			time.Sleep(5 * time.Second)
		}
	}
}

func serveOnce(relayAddr, key string) error {
	conn, err := net.DialTimeout("tcp", relayAddr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("dial relay: %w", err)
	}
	defer conn.Close()

	fmt.Fprintf(conn, "REGISTER %s\n", key)
	log.Printf("registered with relay")

	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("relay connection lost: %w", err)
		}
		line = strings.TrimSpace(line)

		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 3 || parts[0] != "REQ" {
			continue
		}
		reqID := parts[1]
		dest := parts[2]

		log.Printf("incoming request %s → %s", reqID, dest)
		go handleRequest(relayAddr, reqID, dest)
	}
}

func handleRequest(relayAddr, reqID, dest string) {
	bridgeConn, err := net.DialTimeout("tcp", relayAddr, 10*time.Second)
	if err != nil {
		log.Printf("req %s: dial relay for ACCEPT: %v", reqID, err)
		return
	}
	defer bridgeConn.Close()

	fmt.Fprintf(bridgeConn, "ACCEPT %s\n", reqID)

	target, err := net.DialTimeout("tcp", dest, 15*time.Second)
	if err != nil {
		log.Printf("req %s: dial target %s: %v", reqID, dest, err)
		return
	}
	defer target.Close()

	log.Printf("req %s: connected to %s", reqID, dest)

	done := make(chan struct{}, 2)
	go func() {
		io.Copy(bridgeConn, target)
		bridgeConn.Close()
		done <- struct{}{}
	}()
	go func() {
		io.Copy(target, bridgeConn)
		target.Close()
		done <- struct{}{}
	}()
	<-done
	<-done
	log.Printf("req %s: completed", reqID)
}

var _ = os.Stderr
