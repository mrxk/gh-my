package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/docopt/docopt-go"
	"github.com/mrxk/gh-my/internal/github"
	"github.com/mrxk/gh-my/internal/model"
)

const (
	myUsage = `
My Github Plugin: my

Usage:
	my [(prs|requests|all)] [options]

Options:
	-d, --include-drafts  Include draft PRs
	-c, --include-closed  Include closed PRs
	`
)

type Options struct {
	startTab      model.TabIndex
	includeClosed bool
	includeDrafts bool
}

func parseArgs(usage string) (Options, error) {
	opts := Options{}
	docOpts, err := docopt.ParseDoc(usage)
	if err != nil {
		return opts, err
	}
	opts.includeDrafts, _ = docOpts.Bool("--include-drafts")
	opts.includeClosed, _ = docOpts.Bool("--include-closed")
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

func main() {
	opts, err := parseArgs(myUsage)
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	client := github.New(&github.Options{
		Ctx:           ctx,
		Ticker:        nil,
		IncludeDrafts: opts.includeDrafts,
		IncludeClosed: opts.includeClosed,
	})
	client.Run()
	p := tea.NewProgram(model.New(model.Options{
		Context:       ctx,
		Commands:      client.Commands,
		IncludeClosed: opts.includeClosed,
		IncludeDrafts: opts.includeDrafts,
		StartTab:      opts.startTab,
	}), tea.WithAltScreen())
	go pump(client, p)
	_, err = p.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	cancel()
}

func pump(client *github.Client, p *tea.Program) {
	for {
		response := <-client.Data
		p.Send(model.SearchResultsMsg{SearchResults: response})
	}
}
