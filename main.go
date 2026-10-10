// escowx is a terminal dashboard for National Weather Service conditions,
// forecasts and alerts around Escondido, CA.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/j4mes/escowx/api"
	"github.com/j4mes/escowx/config"
	"github.com/j4mes/escowx/tracker"
	"github.com/j4mes/escowx/ui"
)

func main() {
	cfgPath := flag.String("config", "", "path to config file, created with defaults if missing (default ~/.config/escowx/config.yaml)")
	contact := flag.String("contact", "", "contact for the API User-Agent (overrides config and ESCOWX_CONTACT)")
	demoMode := flag.Bool("demo", false, "inject fake alerts to preview every threat mode (m / M cycle modes)")
	once := flag.Bool("once", false, "print a plain-text summary and exit")
	flag.Parse()

	if *cfgPath == "" {
		p, err := config.DefaultPath()
		if err != nil {
			fmt.Fprintln(os.Stderr, "escowx: finding config dir:", err, "(use --config)")
			os.Exit(2)
		}
		*cfgPath = p
	}
	cfg, created, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "escowx:", err)
		os.Exit(2)
	}
	if created {
		fmt.Fprintf(os.Stderr, "escowx: wrote default config to %s\n", *cfgPath)
	}
	if env := os.Getenv("ESCOWX_CONTACT"); env != "" {
		cfg.Contact = env
	}
	if *contact != "" {
		cfg.Contact = *contact
	}
	client := api.NewClient(cfg.Contact)

	if *once {
		os.Exit(runOnce(cfg, client, *demoMode, os.Stdout))
	}

	tr, err := tracker.New(cfg.LogPath())
	if err != nil {
		fmt.Fprintln(os.Stderr, "escowx: opening log:", err)
		os.Exit(2)
	}
	defer tr.Close()
	tr.Bell, tr.Notify, tr.Demo = cfg.Bell, cfg.Notify, *demoMode

	m := ui.New(cfg, client, tr, *demoMode)
	defer m.Shutdown()
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "escowx:", err)
		os.Exit(1)
	}
}
