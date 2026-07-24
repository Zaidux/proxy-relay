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
	fs.Parse(args)

	r := &relayServer{
		serveConns:  make(map[string]net.Conn),
		pendingReqs: make(map[string]chan net.Conn),
	}

	ln, err := net.Listen("tcp", ":"+*port)
	if err != nil {
		log.Fatalf("relay: %v", err)
	}
	fmt.Fprintf(os.Stderr, "relay listening on :%s\n", *port)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("relay accept: %v", err)
			continue
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
		r.serveLoop(key, conn)

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

func (r *relayServer) serveLoop(key string, conn net.Conn) {
	defer func() {
		r.mu.Lock()
		if r.serveConns[key] == conn {
			delete(r.serveConns, key)
		}
		r.mu.Unlock()
		log.Printf("serve disconnected: %s", key)
	}()

	buf := make([]byte, 1)
	for {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, err := conn.Read(buf)
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
