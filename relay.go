package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

func runRelay(args []string) {
	fs := flag.NewFlagSet("relay", flag.ExitOnError)
	port := fs.String("port", "9000", "Listen port")
	listenHost := fs.String("listen-host", "127.0.0.1",
		"Address to bind. 127.0.0.1 (default) keeps the relay private; "+
			"pass 0.0.0.0 to accept registrations from the network")
	fs.Parse(args)

	// A published relay is unauthenticated: anyone who reaches it can
	// REGISTER a key and CONNECT through it, using this box as an open
	// proxy. Say so rather than letting it happen silently.
	if *listenHost != "127.0.0.1" && *listenHost != "localhost" {
		fmt.Fprintf(os.Stderr,
			"WARNING: relay bound to %s — anyone who can reach this port can "+
				"register a key and tunnel traffic through this machine.\n"+
				"Prefer an SSH tunnel (-L 9000:127.0.0.1:9000) over a public bind.\n",
			*listenHost)
	}

	r := &relayServer{
		serveConns:  make(map[string]net.Conn),
		pendingReqs: make(map[string]chan net.Conn),
	}

	addr := net.JoinHostPort(*listenHost, *port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("relay: %v", err)
	}
	fmt.Fprintf(os.Stderr, "relay listening on %s\n", addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("relay accept: %v", err)
			continue
		}
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			tcpConn.SetKeepAlive(true)
			tcpConn.SetKeepAlivePeriod(15 * time.Second)
		}
		go r.handle(conn)
	}
}

type relayServer struct {
	mu          sync.Mutex
	serveConns  map[string]net.Conn
	pendingReqs map[string]chan net.Conn
}

func (r *relayServer) handle(conn net.Conn) {
	transferred := false
	defer func() {
		if !transferred {
			conn.Close()
		}
	}()
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	conn.SetDeadline(time.Time{})

	cmd := parseLine(line)
	if len(cmd) < 2 {
		return
	}

	switch cmd[0] {
	case "REGISTER":
		if len(cmd) < 2 {
			return
		}
		key := cmd[1]
		r.mu.Lock()
		if old, ok := r.serveConns[key]; ok {
			old.Close()
		}
		r.serveConns[key] = conn
		r.mu.Unlock()

		log.Printf("serve registered with key: %s", key)
		// Pass the same bufio.Reader through so any bytes already buffered
		// from the socket (e.g. an early heartbeat) aren't silently dropped.
		r.serveLoop(key, conn, reader)

	case "CONNECT":
		if len(cmd) < 4 {
			conn.Write([]byte("NOKEY missing args\n"))
			return
		}
		key := cmd[1]
		reqID := cmd[2]
		dest := cmd[3]

		r.mu.Lock()
		serveConn, ok := r.serveConns[key]
		if !ok {
			r.mu.Unlock()
			conn.Write([]byte("NOKEY unknown key\n"))
			log.Printf("connect: unknown key %s", key)
			return
		}

		ch := make(chan net.Conn, 1)
		r.pendingReqs[reqID] = ch
		r.mu.Unlock()

		fmt.Fprintf(serveConn, "REQ %s %s\n", reqID, dest)
		log.Printf("REQ %s → %s", reqID, dest)

		select {
		case serveSide := <-ch:
			conn.Write([]byte("OK\n"))
			log.Printf("bridged %s ↔ %s", reqID, dest)
			bridge(conn, serveSide)
		case <-time.After(30 * time.Second):
			conn.Write([]byte("NOKEY timeout waiting for serve\n"))
			r.mu.Lock()
			delete(r.pendingReqs, reqID)
			r.mu.Unlock()
		}

	case "ACCEPT":
		if len(cmd) < 2 {
			return
		}
		reqID := cmd[1]
		r.mu.Lock()
		ch, ok := r.pendingReqs[reqID]
		if ok {
			delete(r.pendingReqs, reqID)
		}
		r.mu.Unlock()
		if ok {
			transferred = true
			ch <- conn
		}

	default:
		log.Printf("unknown cmd: %s", cmd[0])
	}
}

// serveLoop keeps the REGISTER connection open, resetting the read deadline
// on every line received (heartbeat PINGs included). It reads from the same
// bufio.Reader that handle() used for the REGISTER line, so nothing buffered
// during that read is lost.
func (r *relayServer) serveLoop(key string, conn net.Conn, reader *bufio.Reader) {
	defer func() {
		r.mu.Lock()
		if r.serveConns[key] == conn {
			delete(r.serveConns, key)
		}
		r.mu.Unlock()
		log.Printf("serve disconnected: %s", key)
	}()

	for {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, err := reader.ReadString('\n')
		if err != nil {
			return
		}
	}
}

func parseLine(line string) []string {
	var parts []string
	current := ""
	inWord := false
	for _, c := range line {
		if c == ' ' || c == '\n' || c == '\r' {
			if inWord {
				parts = append(parts, current)
				current = ""
				inWord = false
			}
		} else {
			current += string(c)
			inWord = true
		}
	}
	if inWord {
		parts = append(parts, current)
	}
	return parts
}

func bridge(a, b net.Conn) {
	done := make(chan struct{}, 2)
	go func() {
		io.Copy(a, b)
		a.Close()
		done <- struct{}{}
	}()
	go func() {
		io.Copy(b, a)
		b.Close()
		done <- struct{}{}
	}()
	<-done
	<-done
}
