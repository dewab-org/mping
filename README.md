`multiping` (binary `mping`) is a terminal UI for multi-host ping monitoring. It pings hosts concurrently, shows success/failure counters and RTTs, and lets you add/remove hosts at runtime.

![Build](https://github.com/your-repo/mping/actions/workflows/build.yml/badge.svg)

## Build

```bash
make
```

Requires Go 1.22+. The default target builds `./cmd/mping` into `./mping`.

## Run

```bash
./mping example.com 1.1.1.1
./mping --backend native example.com
./mping -f hosts.txt
./mping --theme dracula --list-themes
```

You can add hosts as space-separated positional arguments (domains or IPs) or via `--file/-f` (one host per line).

Key options:
- `--interval/-i <seconds>`: default ping interval (default 10)
- `--timeout/-t <seconds>`: default timeout (default 2)
- `--refresh/-R <seconds>`: screen refresh interval (default 1)
- `--backend <system|native>`: choose ping backend
- `--max-concurrent-pings <n>`: worker pool size (default 64)
- `--ping-queue-capacity <n>`: queue size for pending ping jobs (default 256)
- `--config <path>`: optional YAML config (see example below)
- `--theme/-T <name>`: select theme by name
- `--list-themes`: list themes discovered from theme files

## Themes

Themes are loaded from `.theme` files using hex or `r g b` values.

Search order:
1. `./themes/*.theme`
2. Config directory (if `--config` is provided)
3. `<config_dir>/themes/*.theme`

See `themes/default.theme`, `themes/dracula.theme`, `themes/solarized-dark.theme`, `themes/solarized-light.theme`, and `themes/nord.theme` for examples. You can add more in the same format.

## Example config

See `examples/config.yaml` for a starting point. Themes can be defined inline under `themes:` and/or provided as `.theme` files in the config directory’s `themes/` subfolder.

## Controls

- `a`: add hosts
- `o`: sort options
- `s`: settings (interval, timeout, refresh, sort, theme)
- `r`: reverse sort
- `i`: set interval
- `t`: set timeout
- `d`: delete selected host
- `h` / `?`: help overlay
- `q`: quit
