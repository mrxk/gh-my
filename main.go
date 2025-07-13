package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/docopt/docopt-go"
	"github.com/mrxk/gh-my/internal/model"
	"github.com/mrxk/gh-my/internal/prtable"
	"github.com/sassoftware/sas-ggdk/pkg/jsonutils"
)

const (
	myUsage = `
My Github Plugin: my

Usage:
	my [(prs|requests|all)] [options]

Options:
	-d, --include-drafts               Include draft PRs
	-c, --include-closed               Include closed PRs
	-w <interval>, --watch=<interval>  Poll every <interval>
	-f <path>, --config=<path>         Path to config file [default: ${XDG_CONFIG_HOME}/gh-my/config.json]
	`
)

type Options struct {
	startTab      model.TabIndex
	IncludeClosed bool             `json:"includeClosed,omitempty"`
	IncludeDrafts bool             `json:"includeDrafts,omitempty"`
	Interval      time.Duration    `json:"interval,omitempty"`
	Repositories  []string         `json:"repositories,omitempty"`
	DefaultView   []prtable.Column `json:"defaultView,omitempty"`
	WideView      []prtable.Column `json:"wideView,omitempty"`
}

func parseArgs(usage string) (Options, error) {
	opts := Options{}
	docOpts, err := docopt.ParseDoc(usage)
	if err != nil {
		return opts, err
	}
	path, _ := docOpts.String("--config")
	if path != "" {
		opts = loadConfig(path)
	}
	includeDrafts, _ := docOpts.Bool("--include-drafts")
	if includeDrafts {
		opts.IncludeDrafts = true
	}
	includeClosed, _ := docOpts.Bool("--include-closed")
	if includeClosed {
		opts.IncludeClosed = true
	}
	interval, _ := docOpts.String("--watch")
	if interval != "" {
		duration, err := time.ParseDuration(interval)
		if err != nil {
			return opts, err
		}
		opts.Interval = duration
	}
	prs, _ := docOpts.Bool("prs")
	requests, _ := docOpts.Bool("requests")
	all, _ := docOpts.Bool("all")
	switch {
	case prs:
		opts.startTab = model.MyPRsTab
	case requests:
		opts.startTab = model.MyRequestsTab
	case all:
		opts.startTab = model.AllPRsTab
	}
	return opts, nil
}

func loadConfig(rawPath string) Options {
	_, present := os.LookupEnv("XDG_CONFIG_HOME")
	if !present {
		home, err := os.UserHomeDir()
		if err == nil {
			os.Setenv("XDG_CONFIG_HOME", path.Join(home, ".config"))
		}
	}
	path := os.ExpandEnv(rawPath)
	optionsResult := jsonutils.LoadAs[Options](path)
	if optionsResult.IsError() {
		fmt.Printf("ERROR: failed to load options: %s\n", optionsResult.Error().Error())
		os.Exit(2)
		return Options{}
	}
	return optionsResult.MustGet()
}

func main() {
	opts, err := parseArgs(myUsage)
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	p := tea.NewProgram(model.New(model.Options{
		Context:       ctx,
		IncludeClosed: opts.IncludeClosed,
		IncludeDrafts: opts.IncludeDrafts,
		Interval:      opts.Interval,
		StartTab:      opts.startTab,
		Repositories:  opts.Repositories,
		DefaultView:   opts.DefaultView,
		WideView:      opts.WideView,
	}), tea.WithAltScreen())
	_, err = p.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	cancel()
}
