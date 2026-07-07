# MPing for macOS

SwiftUI desktop client for `mping`. It follows the core behavior of the TUI version—multi-host concurrent pinging with sortable columns—while adopting macOS-native UI conventions (toolbar buttons, Settings window, keyboard shortcuts).

## Requirements

- macOS 13+
- Swift 5.9+

## Build & Run

```bash
cd macos/MPingMac
swift run MPingMac           # debug build
swift build -c release       # release build in .build/release/MPingMac
```

The app uses the system `ping` binary (auto-discovered in standard paths) and runs pings concurrently according to the configured worker limit.

## Features

- Add hosts in bulk (space/comma/newline separators).
- Live table with host/IP, RTT, success %, counts, last OK, and latest error.
- Sortable columns and row selection with Delete shortcut.
- Settings for interval, timeout, backend selection placeholder, and max concurrent pings.
- Adheres to macOS patterns: toolbar actions, Settings window (`⌘,`), delete (`⌘⌫`), and accent-aware styling.
- Toolbar controls to nudge interval/timeout, status indicators, and RTT sparkline per host.

## App icon

Place your SVG at `Sources/MPingMac/Resources/app-icon.svg` and run:

```bash
cd macos/MPingMac
./scripts/build-icons.sh
```

This creates `Sources/MPingMac/Resources/AppIcon.icns` from the SVG using `iconutil`. Renderer priority: `rsvg-convert` (best SVG fidelity) → `inkscape` → ImageMagick (`magick`/`convert`). Provide an explicit SVG path as the first argument to override the default; set `ICON_RENDERER=rsvg|inkscape|magick` to force a renderer. The app loads `AppIcon.icns` from the bundle at launch; rebuild after regenerating the icon.
