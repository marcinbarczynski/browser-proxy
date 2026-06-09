package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/maxischmaxi/browser-proxy/internal/browsers"
	"github.com/maxischmaxi/browser-proxy/internal/config"
	"github.com/maxischmaxi/browser-proxy/internal/opener"
	"github.com/maxischmaxi/browser-proxy/internal/platform"
	"github.com/maxischmaxi/browser-proxy/internal/source"
)

// Version is overridden at build time via -ldflags "-X main.Version=...".
// Local builds report this default; CI release builds report the git tag.
var Version = "1.2.0"

const usage = `browser-proxy — route URLs to the right browser

Usage:
  browser-proxy <command> [args]

Commands:
  init                 Write an example config to ~/.config/browser-proxy/config.toml
  install              Register as the system default browser
  uninstall            Unregister
  open <url>           Open <url> in the configured browser (called by the OS)
  test [-source NAME] <url>
                       Print which browser/profile would be used (does not open anything)
                       -source overrides the auto-detected source app
  profiles <browser>   List the profile names known by a Chromium- or
                       Firefox-family browser (use these as 'profile' in rules)
  daemon               Run the URL-event listener (macOS-internal; called from .app)
  config               Show the active config path and contents
  version              Print version
`

func main() {
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
	case "profiles":
		must(cmdProfiles(args))
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
	fs := flag.NewFlagSet("test", flag.ExitOnError)
	srcOverride := fs.String("source", "", "simulate the source app (name or macOS bundle id)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) < 1 {
		return errors.New("usage: browser-proxy test [-source NAME] <url>")
	}

	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		return err
	}
	defer cfg.Close()

	src := source.Detect()
	if *srcOverride != "" {
		if dotNoSpace(*srcOverride) {
			src = source.Info{BundleID: *srcOverride}
		} else {
			src = source.Info{Name: *srcOverride}
		}
	}

	originalURL := rest[0]
	finalURL := cfg.Rewriter.Apply(originalURL)
	if finalURL != originalURL {
		fmt.Printf("URL rewritten:\n  from: %s\n  to:   %s\n", originalURL, finalURL)
	}

	d := cfg.Router.Resolve(finalURL, src)
	target := d.Browser
	if d.Profile != "" {
		target = fmt.Sprintf("%s [profile: %s]", d.Browser, d.Profile)
	}

	srcLabel := "(no source detected)"
	if !src.Empty() {
		switch {
		case src.Name != "" && src.BundleID != "":
			srcLabel = fmt.Sprintf("(source: %s [%s])", src.Name, src.BundleID)
		case src.Name != "":
			srcLabel = fmt.Sprintf("(source: %s)", src.Name)
		default:
			srcLabel = fmt.Sprintf("(source bundle: %s)", src.BundleID)
		}
	}

	if d.MatchedRule() {
		fmt.Printf("%s   matched rule %d: %s   %s\n", target, d.RuleIndex, cfg.Router.Rules[d.RuleIndex].Describe(), srcLabel)
	} else {
		fmt.Printf("%s   default — no rule matched   %s\n", target, srcLabel)
	}
	return nil
}

func dotNoSpace(s string) bool {
	hasDot := false
	for _, r := range s {
		switch r {
		case '.':
			hasDot = true
		case ' ', '\t', '/':
			return false
		}
	}
	return hasDot
}

func cmdProfiles(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: browser-proxy profiles <browser>")
	}
	browser := args[0]
	switch browsers.DetectFamily(browser) {
	case browsers.Chromium:
		return printChromiumProfiles(browser)
	case browsers.Firefox:
		return printFirefoxProfiles(browser)
	default:
		return fmt.Errorf("%q is not a Chromium- or Firefox-family browser; profile listing is only supported for those", browser)
	}
}

func printChromiumProfiles(browser string) error {
	source, profiles, err := browsers.ListChromiumProfiles(browser)
	if err != nil {
		return err
	}
	fmt.Printf("Browser: %s (Chromium family)\nSource:  %s\n\n", browser, source)

	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	fmt.Fprintln(w, "DIRECTORY\tDISPLAY NAME\tEMAIL")
	for _, p := range profiles {
		email := p.Email
		if email == "" {
			email = "—"
		}
		name := p.Name
		if name == "" {
			name = "—"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", p.Directory, name, email)
	}
	w.Flush()

	fmt.Println()
	fmt.Println("In a rule's 'profile' field you may use either form:")
	if len(profiles) > 0 && profiles[0].Name != "" {
		fmt.Printf("  profile = \"%s\"   # display-name lookup\n", profiles[0].Name)
	}
	if len(profiles) > 0 {
		fmt.Printf("  profile = \"%s\"   # direct directory match\n", profiles[0].Directory)
	}
	return nil
}

func printFirefoxProfiles(browser string) error {
	source, profiles, err := browsers.ListFirefoxProfiles(browser)
	if err != nil {
		return err
	}
	fmt.Printf("Browser: %s (Firefox family)\nSource:  %s\n\n", browser, source)

	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tPATH\tDEFAULT")
	for _, p := range profiles {
		def := ""
		if p.IsDefault {
			def = "*"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", p.Name, p.Path, def)
	}
	w.Flush()

	fmt.Println()
	fmt.Println("In a rule's 'profile' field, use the NAME column:")
	if len(profiles) > 0 {
		fmt.Printf("  profile = \"%s\"\n", profiles[0].Name)
	}
	return nil
}

func cmdOpen(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: browser-proxy open <url>")
	}
	return route(args[0], source.Detect())
}

func route(rawURL string, src source.Info) error {
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		return err
	}
	defer cfg.Close()

	finalURL := cfg.Rewriter.Apply(rawURL)
	if finalURL != rawURL {
		cfg.Log.Rewritten(rawURL, finalURL)
	}

	d := cfg.Router.Resolve(finalURL, src)

	srcLabel := src.Name
	if srcLabel == "" {
		srcLabel = src.BundleID
	}
	ruleDesc := ""
	if d.MatchedRule() {
		ruleDesc = cfg.Router.Rules[d.RuleIndex].Describe()
	}
	cfg.Log.Routed(finalURL, d.Browser, d.Profile, ruleDesc, srcLabel, d.RuleIndex)

	if err := opener.Open(d.Browser, d.Profile, finalURL); err != nil {
		cfg.Log.Error("open %q: %v", d.Browser, err)
		return err
	}
	return nil
}

func runDaemon() {
	platform.RunDaemon(func(rawURL string, src source.Info) {
		if err := route(rawURL, src); err != nil {
			fmt.Fprintf(os.Stderr, "route %q: %v\n", rawURL, err)
		}
	})
}
