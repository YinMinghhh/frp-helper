# FRP STCP Visitor Helper

[中文说明](./README-zh.md)

`frp-helper` is a cross-platform Go CLI for managing the local `frpc visitor` side of FRP STCP access.

V1 focuses on one path:

1. Discover or import a JSON manifest.
2. Install or reuse `frpc`.
3. Generate `frpc.toml`.
4. Start `frpc` in the background by default.
5. Verify local listeners and print usable access commands.
6. Optionally register a login startup item.

## Current Scope

- Supports `windows/amd64`, `darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64`
- Generates a single TOML config with multiple `[[visitors]]`
- Starts `frpc` in the background with `run`
- Supports `start` for foreground debugging and log streaming
- Supports `run`, which auto-discovers `access.json` from the current directory or executable directory
- Supports login startup enable/disable
- Supports bundling the executable plus a real manifest for private distribution
- Persists manifest, generated config, runtime state, and logs
- Supports service enable/disable/remove and manifest merge/replace

Not included in V1:

- System service installation
- Full daemon/service management
- GUI
- `includes`-based config generation
- Auto-upgrade checks

## Build

```bash
go build -o ./bin/frp-helper ./cmd/frp-helper
```

## Quick Start

Use a project-local home directory while developing:

```bash
export FRP_HELPER_HOME="$(pwd)/.frp-helper-dev"
```

Copy the sample manifest and edit it:

```bash
cp ./examples/access.sample.json ./access.json
```

Then run:

```bash
./bin/frp-helper run -f ./access.json
```

`run` will apply the manifest first, wait until listeners are ready, print the access commands, then return to your shell.

If you want to keep `frpc` attached to the current terminal for debugging, use:

```bash
./bin/frp-helper start
```

or:

```bash
./bin/frp-helper run -f ./access.json --foreground
```

In another terminal:

```bash
export FRP_HELPER_HOME="$(pwd)/.frp-helper-dev"
./bin/frp-helper status
./bin/frp-helper endpoints
./bin/frp-helper startup status
./bin/frp-helper stop
```

If you want login startup, enable it once:

```bash
./bin/frp-helper startup enable
```

Or let `run` do both:

```bash
./bin/frp-helper run -f ./access.json --enable-startup
```

## Manifest Format

Top-level fields:

- `serverAddr`
- `serverPort`
- `authToken`
- `user`
- `services`

Each service supports:

- `name`
- `serverName`
- `secretKey`
- `bindPort`
- `serverUser`
- `protocolHint`
- `disabled`
- `accessUser`

Example:

```json
{
  "serverAddr": "frps.example.com",
  "serverPort": 7000,
  "authToken": "YOUR_AUTH_TOKEN",
  "user": "ops",
  "services": [
    {
      "name": "ubuntu ssh",
      "serverName": "ubuntu_ssh",
      "secretKey": "YOUR_STCP_SECRET",
      "bindPort": 6000,
      "protocolHint": "ssh",
      "accessUser": "alice"
    }
  ]
}
```

## Commands

```bash
./bin/frp-helper help
./bin/frp-helper apply -f ./access.json [--merge|--replace]
./bin/frp-helper install [--version v0.68.0] [--archive /path/to/frp.tar.gz|--base-url URL]
./bin/frp-helper run [-f ./access.json] [--merge|--replace] [--enable-startup|--disable-startup] [--foreground]
./bin/frp-helper start
./bin/frp-helper stop
./bin/frp-helper restart
./bin/frp-helper status
./bin/frp-helper endpoints
./bin/frp-helper startup status
./bin/frp-helper startup enable
./bin/frp-helper startup disable
./bin/frp-helper package -f ./access.json -o ./dist/bundle [--startup=true]
./bin/frp-helper purge [--with-bin]
./bin/frp-helper service list
./bin/frp-helper service enable <service-key>
./bin/frp-helper service disable <service-key>
./bin/frp-helper service remove <service-key>
```

## Local Files

By default, data is stored under the platform user config directory.

For development, set:

```bash
export FRP_HELPER_HOME="$(pwd)/.frp-helper-dev"
```

This creates:

- `config/manifest.json`
- `config/frpc.toml`
- `state/runtime.json`
- `logs/frpc.log`
- `bin/frpc/<version>/frpc`

## Real Environment Test

1. Build the CLI.
2. Copy `examples/access.sample.json` to `access.json`.
3. Replace `serverAddr`, `authToken`, `serverName`, and `secretKey` with real values.
4. Run `run`.
6. Confirm that the printed endpoint matches your expected access method, such as:

```bash
./bin/frp-helper run -f ./access.json --enable-startup
```

If you need an offline install or mirror source, use:

```bash
./bin/frp-helper install --archive /path/to/frp_0.68.0_darwin_arm64.tar.gz
```

or:

```bash
./bin/frp-helper install --base-url https://your-mirror.example.com/releases/download
```

## Automated Tests

Run everything:

```bash
go test ./... -count=1
```

The integration tests build and use a stub `frpc`, so they do not require a real `frps` server.

## Private Team Distribution

If you do not want end users to type CLI arguments or manually pass a manifest path, use `package`:

```bash
./bin/frp-helper package -f ./access.json -o ./dist/macos-arm64
```

This creates a private bundle containing:

- the `frp-helper` executable
- a bundled `access.json`
- platform-specific start/stop scripts
- a small bundle README

By default, the generated start script enables login startup on first run. It also starts `frpc` in the background and returns immediately, which makes the private bundle much easier to distribute inside a team than asking every user to run `apply -f`.

## Coexisting with Other FRP Setups

`frp-helper` keeps its own binary, generated config, runtime state, and logs under its own app directory or `FRP_HELPER_HOME`.

It does not edit other FRP configs on the machine. The main shared risk is local port collision: if another FRP process or another program already uses the same `bindPort`, `frp-helper` will fail to start instead of overwriting that setup.

## Adjusting the Default Visitor Template

The default generated visitor shape is defined in code:

- `internal/model/render.go`: TOML rendering for top-level fields and `[[visitors]]`
- `internal/model/model.go`: default constants such as `DefaultFRPCVersion` and `DefaultBindAddr`

If you want to change default `bindAddr`, emitted fields, or default rendering behavior, update those files.
