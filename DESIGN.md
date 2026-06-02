## Overview

`mping` is a terminal TUI for multi-host ping monitoring. It accepts hosts on the CLI and at runtime, pings them concurrently with ICMP, TCP, HTTP, or HTTPS probes, and renders a sortable table showing RTT, HTTP status, success/failure counts, last OK time, and errors. The UI remains responsive via a worker pool and per-host schedulers. Settings (protocol, TCP port, interval, timeout, refresh, sort, theme, backend, system args) can be changed on the fly.

## Core Behavior
- Hosts accepted via CLI (space/comma/newline), `--file`/`-f` (one per line), and runtime Add Hosts dialog. Entries can override global defaults with `tcp:host:port`, `tcp://host:port`, bare `host:port`, `icmp:host`, or HTTP(S) URLs.
- Delete selected host.
- Per-host scheduler submits jobs to a shared worker pool.
- Probe protocol: ICMP (default), TCP connect, HTTP, or HTTPS. ICMP can use system ping or native Go ping.
- Records success/failure counts, RTT, last error, last OK time, IP, resolved name.
- Shared state: map + ordered slice, protected by RWMutex.
- Sorting: hostname, IP, RTT, success, success%, failure, last OK, error; asc/desc.
- UI updates via QueueUpdateDraw; non-blocking ping/DNS.

## UI
- Layout: Title bar (top), table (center) with vertical scrollbar, status bar (bottom).
- Table columns: Hostname (resolved, `-n/a-` if absent), Mode, IP, RTT (2 decimals), Status, OK, Success%, Success, Fail, Last OK (elapsed), Error (clamped, no horizontal scroll).
- Title shows mode, sort, workers, interval, timeout, refresh, theme, config path.
- Status bar shows keybindings.
- Modals: Add hosts, interval, timeout, sort, help, settings.
- Settings modal with sections: Ping (protocol, TCP port, ICMP backend, interval, timeout, system args), Sort (key/dir), Display (theme, refresh). Height is computed from form items + buttons (no scrolling); tab order reaches all fields including sort direction.
- Keys: arrows/PageUp/PageDown for selection; a add; d delete; o sort; s settings; r reverse sort; i interval; t timeout; h/? help; q/Ctrl+C quit.

## Themes
- `.theme` files (key=value) using hex (`#RRGGBB`, `#GG`) or RGB triplets (`r g b`).
- Keys: `title_background`, `title_foreground`, `status_background`, `status_foreground`, `header_background`, `header_foreground`, `row_foreground`, `ok_text_success`, `ok_text_failure`, `modal_border_background`, `modal_border_foreground`, `button_ok_background`, `button_ok_foreground`, `button_cancel_background`, `button_cancel_foreground`.
- Search order for themes:
  1. `./themes/*.theme`
  2. Config directory (if `--config` is provided)
  3. `<config_dir>/themes/*.theme`
  4. `/usr/local/share/mping/themes`, `/usr/share/mping/themes`, `/opt/homebrew/share/mping/themes`
- Theme selection via `--theme/-T` or settings dialog; `--list-themes` lists discovered names.
- Inline themes may also be provided under `themes:` in YAML config.

## Configuration
- Search order:
  1. `--config /path/to/config.yaml` if provided (overrides search).
  2. `$XDG_CONFIG_HOME/mping/config.yaml` if set.
  3. `$HOME/.config/mping/config.yaml` (XDG default).
  4. Legacy fallback: `$HOME/.mping/config.yaml`.
- If not found, built-in defaults apply.
- Config keys (subset): `interval_seconds`, `timeout_seconds`, `refresh_seconds`, `ping.protocol`, `ping.tcp_port`, `ping.backend`, `concurrency.*`, `memory.*`, `ping.*`, `theme` (name), `themes` (inline definitions matching theme keys).
- System ping args are unified (`system_args`); OS defaults are applied if absent.

## Concurrency Model
- Main UI goroutine (tview) handles keypresses/modals; no blocking ping/DNS.
- Host scheduler goroutines (one per host) sleep for interval, enqueue ping jobs.
- Worker pool: size = `max_concurrent_pings`; reads jobs from buffered channel (`ping_queue_capacity`); applies results to shared state; triggers UI redraw.
- Backpressure via job queue; deletion stops scheduler; missing host skips updates.
- UI redraws are driven by a periodic refresh ticker (default 1s) and marked-dirty updates from workers and settings changes.

## Ping Backends
- Interface:
  ```go
  type PingBackend interface {
      Ping(ctx context.Context, target ping.Target, timeout time.Duration) (PingResult, error)
  }
  ```
  `PingResult` includes IP, resolved name, RTT, success, raw error.
- ICMP system backend: OS ping with context timeout; Linux `-w`, macOS `-W`.
- ICMP native backend: Go ping library; respects timeout; mirrors DNS behavior.
- TCP backend: opens a TCP connection to the configured port, or a host-specific `host:port` value when supplied.
- HTTP(S) backend: performs a GET request, does not follow redirects, and records the returned status code/text.

## Startup / Shutdown
- Parse flags; load/merge config; resolve theme; init state; start workers & schedulers; build UI; run tview app.
- On quit: cancel context, stop schedulers/workers, exit cleanly.
