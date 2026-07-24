package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

func runConnect(args []string) {
	if len(args) < 1 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintf(os.Stderr, "connect requires a key argument: proxy-relay connect <key>\n")
		os.Exit(1)
	}

	key := args[0]
	remaining := args[1:]

	fs := flag.NewFlagSet("connect", flag.ExitOnError)
	relayAddr := fs.String("relay", "localhost:9000", "Relay server address")
	proxyAddr := fs.String("proxy", ":8080", "Local HTTP proxy listen address")
	fs.Parse(remaining)

	log.Printf("relay %s  key %s", *relayAddr, key)
	log.Printf("proxy listening on %s", *proxyAddr)

	handler := &proxyHandler{
		relayAddr: *relayAddr,
		key:       key,
	}

	srv := &http.Server{
		Addr:         *proxyAddr,
		Handler:      handler,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("ready — HTTP_PROXY=http://localhost%s", *proxyAddr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("proxy: %v", err)
	}
}

type proxyHandler struct {
	relayAddr string
	key       string
}

func (h *proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		h.handleConnect(w, r)
		return
	}
	h.handleHTTP(w, r)
}

func (h *proxyHandler) handleConnect(w http.ResponseWriter, r *http.Request) {
	dest := r.Host
	if !strings.Contains(dest, ":") {
		dest += ":443"
	}

	relayConn, err := h.openTunnel(dest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		relayConn.Close()
		http.Error(w, "hijack not supported", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		relayConn.Close()
		return
	}

	bridge(clientConn, relayConn)
}

func (h *proxyHandler) handleHTTP(w http.ResponseWriter, r *http.Request) {
	dest := r.Host
	if !strings.Contains(dest, ":") {
		dest += ":80"
	}

	relayConn, err := h.openTunnel(dest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer relayConn.Close()

	relayReader := bufio.NewReader(relayConn)

	r.RequestURI = ""
	r.URL.Scheme = ""
	r.URL.Host = ""
	r.Header.Set("Connection", "close")

	err = r.Write(relayConn)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	resp, err := http.ReadResponse(relayReader, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (h *proxyHandler) openTunnel(dest string) (net.Conn, error) {
	reqID := newReqID()

	conn, err := net.DialTimeout("tcp", h.relayAddr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("relay unreachable: %w", err)
	}

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(15 * time.Second)
	}

	fmt.Fprintf(conn, "CONNECT %s %s %s\n", h.key, reqID, dest)

	okBytes := make([]byte, 3)
	_, err = io.ReadFull(conn, okBytes)
	if err != nil || string(okBytes) != "OK\n" {
		conn.Close()
		if err != nil {
			return nil, fmt.Errorf("relay error: %w", err)
		}
		return nil, fmt.Errorf("relay rejected")
	}

	log.Printf("tunnel %s → %s", reqID, dest)
	return conn, nil
}

func newReqID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
