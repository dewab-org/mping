# mping

![Build](https://github.com/dewab-org/mping/actions/workflows/build.yml/badge.svg)

`multiping` (binary `mping`) is a terminal UI for multi-host monitoring. It probes ICMP, TCP, HTTP, and HTTPS targets concurrently, shows status, success/failure counters, and RTTs, and lets you add/remove targets at runtime.

## Build

### Prerequisites

- Go 1.22+
- `pre-commit` for local commit hooks

### Manual build

```bash
go build -o mping ./cmd/mping
```

### Make

```bash
make                          # build ./mping
make validate                 # gofmt check, go vet, go test, go build
make install PREFIX=/usr/local  # install binary + themes + man page
```

`go install` can build the binary but will not install themes or the man page.

### Development checks

The repository includes local pre-commit hooks for Go formatting, vetting, tests, and build validation:

```bash
pre-commit install
pre-commit run --all-files
```

The same checks are available through `make validate`.

### Releases

- Tagged `v*` builds publish per-platform tarballs to <https://github.com/dewab-org/mping/releases>.
- Nightly builds from `main` publish `mping-<os>-<arch>.tar.gz` to the `nightly` release in <https://github.com/dewab-org/mping/releases>.
- Workflow runs and build artifacts are available at <https://github.com/dewab-org/mping/actions>.
- Targets: Linux amd64/arm64, macOS arm64 (statically linked).

### macOS GUI (Swift)

A SwiftUI desktop client that mirrors the core mping experience lives in `macos/MPingMac`. Build and run with:

```bash
cd macos/MPingMac
swift run MPingMac
```

The GUI uses the system `ping` binary and follows macOS UI conventions (toolbar actions, Settings window).

## Demo

[![asciicast](https://asciinema.org/a/wdOvXd28Acqnh8oZsn8z4bYOI.svg)](https://asciinema.org/a/wdOvXd28Acqnh8oZsn8z4bYOI)

## Run

```bash
./mping example.com 1.1.1.1
./mping --backend native example.com
./mping --protocol tcp --tcp-port 443 example.com
./mping example.com tcp:api.example.com:8443 icmp:router.local
./mping https://example.com/health http://localhost:8080/status
./mping -f hosts.txt
./mping --theme dracula --list-themes
```

You can add hosts as space-separated positional arguments or via `--file/-f` (one host per line). Host entries accept optional per-host protocol and TCP port overrides:

- `example.com`: use the global protocol and TCP port defaults
- `example.com:443`: monitor TCP/443
- `tcp:example.com:8443` or `tcp://example.com:8443`: monitor TCP/8443
- `icmp:example.com` or `icmp://example.com`: monitor ICMP
- `https://example.com/health` or `http://localhost:8080/status`: monitor HTTP(S) and show the returned status code

Key options:

- `--interval/-i <seconds>`: default ping interval (default 10)
- `--timeout/-t <seconds>`: default timeout (default 2)
- `--refresh/-R <seconds>`: screen refresh interval (default 1)
- `--protocol <icmp|tcp|http|https>`: probe protocol (default icmp)
- `--tcp-port <port>`: TCP port used for TCP probes (default 443)
- `--backend <system|native>`: choose ping backend
- `--max-concurrent-pings <n>`: worker pool size (default 64)
- `--ping-queue-capacity <n>`: queue size for pending ping jobs (default 256)
- `--config <path>`: YAML config path (see Configuration)
- `--theme/-T <name>`: select theme by name
- `--list-themes`: list themes discovered from theme files
- `--version`: show program version

## Configuration

### File locations (XDG)

Search order for config (first found wins):

1. `--config /path/to/config.yaml` (overrides search)
2. `$XDG_CONFIG_HOME/mping/config.yaml`
3. `$HOME/.config/mping/config.yaml`
4. Legacy fallback: `$HOME/.mping/config.yaml`

If none is found, built-in defaults are used.

### Config structure (YAML)

```yaml
interval_seconds: 10
timeout_seconds: 2
refresh_seconds: 1
theme: "default"

concurrency:
  max_concurrent_pings: 64
  max_hosts: 0
  ping_queue_capacity: 256

memory:
  max_hosts_tracked: 0

ping:
  protocol: "icmp"             # or "tcp", "http", "https"
  tcp_port: 443                # used when protocol is tcp
  backend: "system"            # or "native"
  system_command: "ping"
  system_args: ["-c", "1"]     # defaults chosen per OS if omitted

themes:
  custom-dark:
    title_background: "#000080"
    title_foreground: "#ffffff"
    status_background: "#000080"
    status_foreground: "#ffffff"
    header_background: "#000060"
    header_foreground: "#ffffff"
    row_foreground: "#ffffff"
    ok_text_success: "#00ff00"
    ok_text_failure: "#ff0000"
    modal_border_background: "#000080"
    modal_border_foreground: "#ffffff"
    button_ok_background: "#00aa00"
    button_ok_foreground: "#ffffff"
    button_cancel_background: "#aa0000"
    button_cancel_foreground: "#ffffff"
```

### Themes

Theme discovery order:

1. `./themes/*.theme`
2. Config directory (if `--config` is provided)
3. `<config_dir>/themes/*.theme`
4. `/usr/local/share/mping/themes`, `/usr/share/mping/themes`, `/opt/homebrew/share/mping/themes`

`.theme` files use key=value pairs with hex (`#RRGGBB`, `#GG`) or RGB triplets (`r g b`).

Keys: `title_background`, `title_foreground`, `status_background`, `status_foreground`, `header_background`, `header_foreground`, `row_foreground`, `ok_text_success`, `ok_text_failure`, `modal_border_background`, `modal_border_foreground`, `button_ok_background`, `button_ok_foreground`, `button_cancel_background`, `button_cancel_foreground`.

Examples provided in `themes/`: `default`, `dracula`, `solarized-dark`, `solarized-light`, `nord`.

You can define themes inline in YAML under `themes:` or via `.theme` files.

## Controls

- `a`: add hosts
- `o`: sort cycle
- `s`: settings (global protocol, TCP port, interval, timeout, refresh, sort, theme, backend, args)
- `r`: reverse sort
- `i`: set interval
- `t`: set timeout
- `d`: delete selected host
- `h` / `?`: help overlay
- `q`: quit
