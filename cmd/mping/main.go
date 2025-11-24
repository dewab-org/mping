package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"mping/internal/concurrency"
	"mping/internal/config"
	"mping/internal/ping"
	"mping/internal/state"
	"mping/internal/theme"
	uiPkg "mping/internal/ui"
)

func main() {
	var (
		intervalVal  int
		timeoutVal   int
		configFlag   = flag.String("config", "", "path to config file")
		fileFlag     = flag.String("file", "", "path to file with one host per line")
		backendFlag  = flag.String("backend", "", "ping backend (system|native)")
		workersFlag  = flag.Int("max-concurrent-pings", 0, "worker pool size")
		queueFlag    = flag.Int("ping-queue-capacity", 0, "ping queue capacity")
		maxHostsFlag = flag.Int("max-hosts", 0, "maximum hosts (0 = unlimited)")
		helpFlag     = flag.Bool("help", false, "show help")
		helpShort    = flag.Bool("h", false, "show help (shorthand)")
		refreshVal   int
		themeFlag    = flag.String("theme", "", "theme name")
		themeShort   = flag.String("T", "", "theme name (shorthand)")
		listThemes   = flag.Bool("list-themes", false, "list available themes")
	)

	flag.IntVar(&intervalVal, "interval", 0, "default interval seconds")
	flag.IntVar(&intervalVal, "i", 0, "default interval seconds (shorthand)")
	flag.IntVar(&timeoutVal, "timeout", 0, "default timeout seconds")
	flag.IntVar(&timeoutVal, "t", 0, "default timeout seconds (shorthand)")
	flag.StringVar(fileFlag, "f", "", "file with one host per line (shorthand)")
	flag.IntVar(&refreshVal, "refresh", 0, "screen refresh interval seconds")
	flag.IntVar(&refreshVal, "R", 0, "screen refresh interval seconds (shorthand)")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: mping [options] host1 host2 ...\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Provide hosts as space-separated positional arguments (domains or IPs).\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintln(flag.CommandLine.Output(), "\nExamples:")
		fmt.Fprintln(flag.CommandLine.Output(), "  mping example.com 1.1.1.1")
		fmt.Fprintln(flag.CommandLine.Output(), "  mping --backend native host1 host2")
	}
	versionFlag := flag.Bool("version", false, "show version")
	flag.Parse()

	if *helpFlag || *helpShort {
		flag.Usage()
		return
	}
	if *versionFlag {
		fmt.Println("mping v0.0.2")
		return
	}
	// themeDirs defined after config parsing; postpone listThemes handling

	fileHosts := []string{}
	if *fileFlag != "" {
		list, err := readHostsFile(*fileFlag)
		if err != nil {
			log.Fatalf("read hosts file: %v", err)
		}
		fileHosts = list
	}

	defaultCfg := config.Defaults()

	var cfgPath string
	var fileCfg config.Config
	if path, ok := config.FindConfigPath(*configFlag); ok {
		cfgPath = path
		parsed, err := config.LoadConfigFile(path)
		if err != nil {
			log.Fatalf("load config: %v", err)
		}
		fileCfg = parsed
	} else if *configFlag != "" {
		log.Fatalf("config file not found: %s", *configFlag)
	}

	themeDirs := []string{"themes"}
	if cfgPath != "" {
		cfgDir := filepath.Dir(cfgPath)
		themeDirs = append(themeDirs, cfgDir, filepath.Join(cfgDir, "themes"))
	}
	themeDirs = append(themeDirs,
		"/usr/local/share/mping/themes",
		"/usr/share/mping/themes",
		"/opt/homebrew/share/mping/themes",
	)
	fileThemes := theme.LoadThemeFiles(themeDirs)

	cli := config.CLIOverrides{
		IntervalSeconds:    intervalVal,
		TimeoutSeconds:     timeoutVal,
		RefreshSeconds:     refreshVal,
		MaxConcurrentPings: *workersFlag,
		PingQueueCapacity:  *queueFlag,
		MaxHosts:           *maxHostsFlag,
		Backend:            *backendFlag,
		ThemeName:          coalesce(*themeFlag, *themeShort),
	}

	settings, err := config.MergeSettings(defaultCfg, fileCfg, cli, cfgPath)
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	if settings.ThemeName == "" {
		settings.ThemeName = defaultCfg.ThemeName
	}

	allThemes := map[string]config.ThemeConfig{}
	for k, v := range fileThemes {
		allThemes[k] = v
	}
	for k, v := range fileCfg.Themes {
		allThemes[k] = v
	}
	if len(allThemes) == 0 {
		allThemes[defaultCfg.ThemeName] = defaultCfg.Theme
	}

	// populate default system args if missing
	if len(settings.SystemArgs) == 0 {
		settings.SystemArgs = defaultCfg.Ping.SystemArgs
	}

	if *listThemes {
		printThemes(allThemes)
		return
	}

	app := tview.NewApplication()

	th := theme.ResolveTheme(settings.ThemeName, settings.Theme, allThemes)
	st := state.NewSharedState(settings.MaxHosts)

	hosts := append(fileHosts, flag.Args()...)
	for _, h := range hosts {
		_ = st.AddHost(h, settings.Interval, settings.Timeout)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	backend := chooseBackend(settings)
	var ui *uiPkg.UI
	var dirty int32
	var refreshNs int64 = settings.RefreshInterval.Nanoseconds()
	markDirty := func() { atomic.StoreInt32(&dirty, 1) }

	pool := concurrency.NewWorkerPool(ctx, backend, st, settings.MaxConcurrentPings, settings.PingQueueCapacity, func() {
		markDirty()
	})
	schedulers := concurrency.NewSchedulerGroup(ctx, st, pool)

	for _, h := range hosts {
		schedulers.Start(h)
	}

	cb := uiPkg.Callbacks{
		AddHosts: func(text string) {
			parts := splitHosts(text)
			for _, p := range parts {
				if err := st.AddHost(p, settings.Interval, settings.Timeout); err == nil {
					schedulers.Start(p)
				}
			}
			if ui != nil {
				ui.Refresh()
			}
			markDirty()
		},
		DeleteHost: func(host string) {
			schedulers.Stop(host)
			st.DeleteHost(host)
			if ui != nil {
				ui.Refresh()
			}
			markDirty()
		},
		SetInterval: func(d time.Duration) {
			settings.Interval = d
			st.SetInterval(d)
			if ui != nil {
				ui.Config.Interval = d
				ui.Refresh()
			}
			markDirty()
		},
		SetTimeout: func(d time.Duration) {
			settings.Timeout = d
			st.SetTimeout(d)
			if ui != nil {
				ui.Config.Timeout = d
				ui.Refresh()
			}
			markDirty()
		},
		SetRefreshInterval: func(d time.Duration) {
			settings.RefreshInterval = d
			atomic.StoreInt64(&refreshNs, d.Nanoseconds())
			if ui != nil {
				ui.Config.RefreshInterval = d
			}
			markDirty()
		},
		SetSort: func(k state.SortKey, d state.SortDirection) {
			st.SetSort(k, d)
			if ui != nil {
				ui.Refresh()
			}
			markDirty()
		},
		ReverseSort: func() {
			key, dir := st.SortConfig()
			if dir == state.SortAsc {
				st.SetSort(key, state.SortDesc)
			} else {
				st.SetSort(key, state.SortAsc)
			}
			if ui != nil {
				ui.Refresh()
			}
			markDirty()
		},
		SetTheme: func(name string) {
			settings.ThemeName = name
			th = theme.ResolveTheme(name, settings.Theme, allThemes)
			if ui != nil {
				ui.UpdateTheme(th, name)
			}
			markDirty()
		},
		SetBackend: func(name, argsLinux, argsDarwin string) {
			if name != "" {
				settings.Backend = name
			}
			if strings.TrimSpace(argsLinux) != "" {
				settings.SystemArgs = strings.Fields(argsLinux)
			}
			if ui != nil {
				ui.Config.Backend = settings.Backend
				ui.Config.SystemArgs = settings.SystemArgs
			}
			markDirty()
		},
		Quit: func() {
			cancel()
			app.Stop()
		},
	}

	ui = uiPkg.NewUI(app, st, th, settings, cb, availableThemes(allThemes))

	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		ui.RefreshWithScreen(screen)
		return false
	})
	app.SetRoot(ui.Pages, true)

	// Periodic redraw decoupled from ping interval for smoothness.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(atomic.LoadInt64(&refreshNs))):
				_ = atomic.SwapInt32(&dirty, 0)
				app.QueueUpdateDraw(ui.Refresh)
			}
		}
	}()

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	schedulers.StopAll()
	pool.Close()
}

func chooseBackend(settings config.Settings) ping.PingBackend {
	switch strings.ToLower(settings.Backend) {
	case "native":
		return ping.NewNativeBackend()
	default:
		return ping.NewSystemBackend(settings.SystemCommand, settings.SystemArgs)
	}
}

func printThemes(themes map[string]config.ThemeConfig) {
	names := availableThemes(themes)
	for _, n := range names {
		fmt.Println(n)
	}
}

func availableThemes(custom map[string]config.ThemeConfig) []string {
	out := make([]string, 0, len(custom))
	for name := range custom {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func coalesce(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func splitHosts(text string) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == ';' || r == '\n' || r == '\r'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if trimmed := strings.TrimSpace(f); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func readHostsFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out, nil
}
