## Overview

`mping` is a terminal TUI for multi-host ping monitoring. It accepts hosts on the CLI and at runtime, pings them concurrently with a pluggable backend, and renders a sortable table showing RTT, success/failure counts, last OK time, and errors. The UI remains responsive via a worker pool and per-host schedulers. Settings can be changed on the fly (interval, timeout, refresh, sort, theme).

## Core Behavior
- Hosts accepted via CLI (space/comma/newline), `--file`/`-f` (one per line), and runtime Add Hosts dialog.
- Delete selected host.
- Per-host scheduler submits jobs to a shared worker pool.
- Pluggable ping backend: system ping (default) or native Go ping.
- Records success/failure counts, RTT, last error, last OK time, IP, resolved name.
- Shared state: map + ordered slice, protected by RWMutex.
- Sorting: hostname, IP, RTT, success, success%, failure, last OK, error; asc/desc.
- UI updates via QueueUpdateDraw; non-blocking ping/DNS.

## UI
- Layout: Title bar (top), table (center) with vertical scrollbar, status bar (bottom).
- Table columns: Hostname (resolved, `-n/a-` if absent), IP, RTT (2 decimals), OK, Success%, Success, Fail, Last OK (elapsed), Error (clamped, no horizontal scroll).
- Title shows backend, sort, workers, interval, timeout, refresh, theme, config path.
- Status bar shows keybindings.
- Modals: Add hosts, interval, timeout, sort, help, settings.
- Settings modal with sections: Ping (backend, interval, timeout, system args), Sort (key/dir), Display (theme, refresh). Height is dynamic to fit items and buttons without scrolling.
- Keys: arrows/PageUp/PageDown for selection; a add; d delete; o sort; s settings; r reverse sort; i interval; t timeout; h/? help; q/Ctrl+C quit.

## Themes
- `.theme` files (key=value) using hex (`#RRGGBB`, `#GG`) or RGB triplets (`r g b`).
- Keys: `title_background`, `title_foreground`, `status_background`, `status_foreground`, `header_background`, `header_foreground`, `row_foreground`, `ok_text_success`, `ok_text_failure`, `modal_border_background`, `modal_border_foreground`.
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
- Config keys (subset): `interval_seconds`, `timeout_seconds`, `refresh_seconds`, `ping.backend`, `concurrency.*`, `memory.*`, `ping.*`, `theme` (name), `themes` (inline definitions matching theme keys).

## Concurrency Model
- Main UI goroutine (tview) handles keypresses/modals; no blocking ping/DNS.
- Host scheduler goroutines (one per host) sleep for interval, enqueue ping jobs.
- Worker pool: size = `max_concurrent_pings`; reads jobs from buffered channel (`ping_queue_capacity`); applies results to shared state; triggers UI redraw.
- Backpressure via job queue; deletion stops scheduler; missing host skips updates.

## Ping Backends
- Interface:
  ```go
  type PingBackend interface {
      Ping(ctx context.Context, hostName string, timeout time.Duration) (PingResult, error)
  }
  ```
  `PingResult` includes IP, resolved name, RTT, success, raw error.
- System backend: OS ping with context timeout; Linux `-w`, macOS `-W`.
- Native backend: Go ping library; respects timeout; mirrors DNS behavior.

## Startup / Shutdown
- Parse flags; load/merge config; resolve theme; init state; start workers & schedulers; build UI; run tview app.
- On quit: cancel context, stop schedulers/workers, exit cleanly.
