# Usage

## Installation

Clone the repository for local development:

```bash
git clone https://github.com/gofurry/go-steam-core.git
cd go-steam-core
go mod download
```

The project is currently a scaffold. Runtime acceleration commands will be introduced in the v0.1.0 milestone.

## Basic Usage

Run the scaffold CLI:

```bash
go run ./cmd/steam-accelerator --version
```

Run the basic example:

```bash
go run ./examples/basic
```

## Configuration

Configuration loading is planned for v0.1.0. The first stable shape is expected to include:

- `mode`: `proxy_only`, `pac`, `system`, or `hosts`.
- `proxy.listen_addr`: default local proxy address.
- `resolver`: DNS / DoH mode, servers, cache, timeout, and IP policy.
- `upstream`: direct, HTTP proxy, or SOCKS5 proxy.
- `hosts`: hosts-mode HTTP and HTTPS listener settings.
- `cert`: local root CA options.
- `rules`: default Steam rules and custom domains.

## Common Examples

Current scaffold checks:

```bash
go test ./...
go vet ./...
go run ./cmd/steam-accelerator --version
go run ./examples/basic
```

Future v0.1.0 usage target:

```bash
steam-accelerator start --mode proxy-only
steam-accelerator status
steam-accelerator stop
```

Future restore target:

```bash
steam-accelerator restore
```
