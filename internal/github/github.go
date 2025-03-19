package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/sassoftware/sas-ggdk/pkg/result"
	"github.com/sassoftware/sas-ggdk/pkg/sliceutils"
)

type PullRequestSearchResults struct {
	Data struct {
		Search struct {
			IssueCount int `json:"issueCount"`
			Edges      []struct {
				Node struct {
					Additions int `json:"additions"`
					Author    struct {
						Login string `json:"login"`
					} `json:"author"`
					ChangedFiles int    `json:"changedFiles"`
					CreatedAt    string `json:"string"`
					Deletions    int    `json:"deletions"`
					Number       int    `json:"number"`
					Repository   struct {
						NameWithOwner string `json:"nameWithOwner"`
					} `json:"repository"`
					ReviewDecision    string `json:"reviewDecision"`
					StatusCheckRollup struct {
						State string `json:"state"`
					} `json:"statusCheckRollup"`
					Title              string `json:"title"`
					URL                string `json:"url"`
					Mergeable          string `json:"mergeable"`
					IsDraft            bool   `json:"isDraft"`
					State              string `json:"state"`
					UpdatedAt          string `json:"updatedAt"`
					TotalCommentsCount int    `json:"totalCommentsCount"`
				} `json:"node"`
			} `json:"edges"`
		} `json:"search"`
	} `json:"data"`
}

// SearchResults is a sealed interface that is used to indicate which structs
// are returned as search results from this github client
type SearchResults interface {
	isSearchResults()
}

type MyPRs struct {
	SearchResults result.Result[PullRequestSearchResults]
}

func (*MyPRs) isSearchResults() {}

type MyRequets struct {
	SearchResults result.Result[PullRequestSearchResults]
}

func (*MyRequets) isSearchResults() {}

type AllPRs struct {
	SearchResults result.Result[PullRequestSearchResults]
}

func (*AllPRs) isSearchResults() {}

// Command is a sealed interface that is used to indicate which structs are
// commands for this github client
type Command interface {
	isCommand()
}

type FetchMyPRs struct{}

func (FetchMyPRs) isCommand() {}

type FetchMyRequests struct{}

func (FetchMyRequests) isCommand() {}

type FetchAllPRs struct{}

func (FetchAllPRs) isCommand() {}

type IncludeDrafts bool

func (IncludeDrafts) isCommand() {}

type IncludeClosed bool

func (IncludeClosed) isCommand() {}

type Client struct {
	Data     <-chan SearchResults
	Commands chan<- Command

	data          chan<- SearchResults
	ctx           context.Context
	ticker        *time.Ticker
	commands      <-chan Command
	includeDrafts bool
	includeClosed bool
	repositories  []string
}

type Options struct {
	Ctx           context.Context
	Ticker        *time.Ticker
	IncludeDrafts bool
	IncludeClosed bool
}

func New(config *Options) *Client {
	data := make(chan SearchResults)
	commands := make(chan Command)
	c := &Client{
		Data:     data,
		Commands: commands,
		data:     data,
		commands: commands,
	}
	// Load defaults from file
	_ = c.loadConfig()

	// Override with explicit args
	c.ctx = config.Ctx
	c.ticker = config.Ticker
	c.includeDrafts = config.IncludeDrafts
	c.includeClosed = config.IncludeClosed
	return c
}

func (c *Client) Run() {
	go c.run()
}

func (c *Client) run() {
	var ticker <-chan time.Time
	if c.ticker != nil {
		ticker = c.ticker.C
	}
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker:
			c.fetchMyPRs()
		case cmd := <-c.commands:
			switch typedCmd := cmd.(type) {
			case FetchMyPRs:
				c.fetchMyPRs()
			case FetchMyRequests:
				c.fetchMyRequests()
			case FetchAllPRs:
				c.fetchAllPRs()
			case IncludeDrafts:
				c.includeDrafts = bool(typedCmd)
			case IncludeClosed:
				c.includeClosed = bool(typedCmd)
			}
		}
	}
}

const template = `
{
  search(query: "%s", type: ISSUE, first: 100) {
    issueCount edges {
	  node {
	    ... on PullRequest {
		  statusCheckRollup {
		    state
          }
		  number
		  title
		  repository {
		    nameWithOwner
		  }
		  createdAt
		  url
		  changedFiles
		  additions
		  deletions
		  reviewDecision
		  author {
		    login
		  }
		  mergeable
		  isDraft
		  state
		  updatedAt
		  totalCommentsCount
        }
      }
    }
  }
}
`

func (c *Client) fetchMyPRs() {
	query := fmt.Sprintf("is:pr author:@me %s %s", c.getIncludeDraftQueryArg(), c.getIncludeClosedQueryArg())
	myQuery := fmt.Sprintf(template, query)
	myPrsData := c.executeQuery(myQuery)
	c.data <- &MyPRs{SearchResults: myPrsData}
}

func (c *Client) fetchMyRequests() {
	query := fmt.Sprintf("is:pr review-requested:@me %s %s", c.getIncludeDraftQueryArg(), c.getIncludeClosedQueryArg())
	myQuery := fmt.Sprintf(template, query)
	myPrsData := c.executeQuery(myQuery)
	c.data <- &MyRequets{SearchResults: myPrsData}
}

func (c *Client) fetchAllPRs() {
	if len(c.repositories) == 0 {
		c.data <- &AllPRs{SearchResults: result.Ok(PullRequestSearchResults{})}
		return
	}
	query := fmt.Sprintf("is:pr %s %s %s", c.getRepoQueryArg(), c.getIncludeDraftQueryArg(), c.getIncludeClosedQueryArg())
	myPrsQuery := fmt.Sprintf(template, query)
	myPrsData := c.executeQuery(myPrsQuery)
	c.data <- &AllPRs{SearchResults: myPrsData}
}

func (c *Client) executeQuery(query string) result.Result[PullRequestSearchResults] {
	cmd := exec.CommandContext(c.ctx, "gh", "api", "graphql", "-f", fmt.Sprintf("query=%s", query))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return result.Error[PullRequestSearchResults](fmt.Errorf("%s: %w", output, err))
	}
	var response PullRequestSearchResults
	err = json.Unmarshal(output, &response)
	if err != nil {
		return result.Error[PullRequestSearchResults](err)
	}
	return result.Ok(response)
}

func (c *Client) getIncludeDraftQueryArg() string {
	if !c.includeDrafts {
		return "draft:false"
	}
	return ""
}

func (c *Client) getIncludeClosedQueryArg() string {
	if !c.includeClosed {
		return "is:open"
	}
	return ""
}

func (c *Client) getRepoQueryArg() string {
	repositories := sliceutils.MapNoError(func(r string) string { return "repo:" + r }, c.repositories)
	return strings.Join(repositories, " ")
}

func (c *Client) loadConfig() error {
	type configuration struct {
		IncludeClosed bool
		IncludeDrafts bool
		Repositories  []string
	}
	configPath := getConfigPath()
	configResult := result.FlatMap(load[configuration], configPath)
	if configResult.IsError() {
		return configResult.Error()
	}
	config := configResult.MustGet()
	c.includeClosed = config.IncludeClosed
	c.includeDrafts = config.IncludeDrafts
	c.repositories = config.Repositories
	return nil
}

func getConfigPath() result.Result[string] {
	configRoot, present := os.LookupEnv("XDG_CONFIG_HOME")
	if present {
		return result.Ok(configPathFromConfigRoot(configRoot))
	}
	homeDir := result.New(os.UserHomeDir())
	return result.MapNoError(configPathFromHomeDir, homeDir)
}

func configPathFromHomeDir(d string) string {
	configRoot := path.Join(d, ".config")
	return configPathFromConfigRoot(configRoot)
}

func configPathFromConfigRoot(d string) string {
	return path.Join(d, "gh-my", "config.json")
}

func load[T any](path string) result.Result[T] {
	content := result.New(os.ReadFile(path))
	return result.FlatMap(unmarshal[T], content)
}

func unmarshal[T any](content []byte) result.Result[T] {
	var t T
	err := json.Unmarshal(content, &t)
	return result.New(t, err)
}
