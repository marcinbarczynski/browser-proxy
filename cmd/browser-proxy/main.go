package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/max/browser-proxy/internal/config"
	"github.com/max/browser-proxy/internal/opener"
	"github.com/max/browser-proxy/internal/platform"
)

const Version = "0.1.0"

const usage = `browser-proxy — route URLs to the right browser

Usage:
  browser-proxy <command> [args]

Commands:
  init                 Write an example config to ~/.config/browser-proxy/config.toml
  install              Register as the system default browser
  uninstall            Unregister
  open <url>           Open <url> in the configured browser (called by the OS)
  test <url>           Print which browser would be used (does not open anything)
  daemon               Run the URL-event listener (macOS-internal; called from .app)
  config               Show the active config path and contents
  version              Print version
`

func main() {
	// macOS: when launched from inside the .app bundle (no args), enter
	// daemon mode automatically. Linux's .desktop always passes "open <url>".
	if platform.IsBundleStart() && (len(os.Args) < 2 || os.Args[1] == "") {
		runDaemon()
		return
	}

	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]

	switch cmd {
	case "init":
		must(cmdInit())
	case "install":
		must(platform.Install())
	case "uninstall":
		must(platform.Uninstall())
	case "open":
		must(cmdOpen(args))
	case "test":
		must(cmdTest(args))
	case "daemon":
		runDaemon()
	case "config":
		must(cmdShowConfig())
	case "version", "--version", "-v":
		fmt.Println(Version)
	case "help", "--help", "-h":
		fmt.Print(usage)
	default:
		fmt.Fprintln(os.Stderr, "unknown command:", cmd)
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}

func must(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func cmdInit() error {
	p := config.DefaultPath()
	if _, err := os.Stat(p); err == nil {
		return fmt.Errorf("config already exists at %s (delete it to start over)", p)
	}
	if err := config.WriteExample(p); err != nil {
		return err
	}
	fmt.Printf("Wrote example config to %s\n", p)
	fmt.Println("Edit it to taste, then run `browser-proxy install`.")
	return nil
}

func cmdShowConfig() error {
	p := config.DefaultPath()
	fmt.Println("Path:", p)
	data, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	fmt.Println("---")
	fmt.Print(string(data))
	return nil
}

func cmdTest(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: browser-proxy test <url>")
	}
	r, err := config.Load(config.DefaultPath())
	if err != nil {
		return err
	}
	browser, idx, err := r.Resolve(args[0])
	if err != nil {
		return err
	}
	if idx >= 0 {
		fmt.Printf("%s   (matched rule %d: %s)\n", browser, idx, r.Rules[idx].Match)
	} else {
		fmt.Printf("%s   (default — no rule matched)\n", browser)
	}
	return nil
}

func cmdOpen(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: browser-proxy open <url>")
	}
	return route(args[0])
}

func route(rawURL string) error {
	r, err := config.Load(config.DefaultPath())
	if err != nil {
		return err
	}
	browser, _, err := r.Resolve(rawURL)
	if err != nil {
		return err
	}
	return opener.Open(browser, rawURL)
}

func runDaemon() {
	platform.RunDaemon(func(rawURL string) {
		if err := route(rawURL); err != nil {
			fmt.Fprintf(os.Stderr, "route %q: %v\n", rawURL, err)
		}
	})
}
