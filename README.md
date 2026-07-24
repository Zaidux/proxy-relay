# proxy-relay

Lightweight peer-to-peer IP relay. Share your residential IP with another device so its traffic exits from your network. Built in Go — zero dependencies, single binary, works on Termux/Android.

## How it works

```
User A (home, residential IP)          Relay (public VPS)            User B (CLI tools)
  proxy-relay serve                      proxy-relay relay            proxy-relay connect <key>
```

1. User A runs `serve` → generates a 4-word key → shares it
2. User B runs `connect <key>` → gets a local HTTP proxy on `:8080`
3. All traffic from User B now exits through User A's residential IP

No root, no kernel modules, no inbound ports — just outbound TCP.

## Install

```bash
# Via pip (recommended)
pip install proxy-relay

# Via Go
go install github.com/Zaidux/proxy-relay@latest

# Prebuilt binary
curl -L https://github.com/Zaidux/proxy-relay/releases/latest/download/proxy-relay-linux-amd64 -o /usr/local/bin/proxy-relay
chmod +x /usr/local/bin/proxy-relay
```

## Usage

### 1. Start the relay (on a publicly reachable VPS)
```bash
proxy-relay relay --port=9000
```

### 2. Share your IP (on your home device)
```bash
proxy-relay serve --relay=<vps-ip>:9000
# Prints: Key: coral-echo-pine-dawn
```

### 3. Use someone's IP (on any machine)
```bash
proxy-relay connect coral-echo-pine-dawn --relay=<vps-ip>:9000
# Local HTTP proxy running on :8080
export HTTP_PROXY=http://localhost:8080
```

### Verify
```bash
curl --proxy http://localhost:8080 http://ipinfo.io/ip
# Should show User A's IP, not the VPS IP
```

## Riciplay integration

Set the proxy in `~/.config/riciplay/config.json`:
```json
{"proxy": "http://localhost:8080"}
```

All riciplay tools (browse, nuclei, dalfox, sqlmap, etc.) route through the residential IP.

## Building from source

```bash
git clone https://github.com/Zaidux/proxy-relay.git
cd proxy-relay
go build -o proxy-relay .
```

Or cross-compile:
```bash
make cross   # builds linux/darwin/windows for amd64/arm64
```

## License

MIT
