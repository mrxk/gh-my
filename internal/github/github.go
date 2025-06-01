package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

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
					MergeStateStatus   string `json:"mergeStateStatus"`
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
		  mergeStateStatus
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

type Option func(string) string

func WithDrafts(include bool) func(string) string {
	return func(query string) string {
		if include {
			return query
		}
		return query + " draft:false"
	}
}

func WithClosed(include bool) func(string) string {
	return func(query string) string {
		if include {
			return query
		}
		return query + " is:open"
	}
}

func ForRepositories(repositories []string) func(string) string {
	return func(query string) string {
		repositories = sliceutils.MapNoError(func(r string) string { return "repo:" + r }, repositories)
		return query + strings.Join(repositories, " ")
	}
}

func ForMyPRs(query string) string {
	if len(query) != 0 {
		query = query + " "
	}
	return query + "is:pr author:@me"
}

func ForMyRequests(query string) string {
	if len(query) != 0 {
		query = query + " "
	}
	return query + "is:pr review-requested:@me"
}

func ExecuteQuery(ctx context.Context, options ...Option) result.Result[PullRequestSearchResults] {
	query := ""
	for _, option := range options {
		query = option(query)
	}
	query = fmt.Sprintf(template, query)
	cmd := exec.CommandContext(ctx, "gh", "api", "graphql", "-f", fmt.Sprintf("query=%s", query))
	cmd.Env = env
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

var env []string

func init() {
	env = os.Environ()
	for i := range env {
		if strings.HasPrefix(env[i], "GH_NO_UPDATE_NOTIFIER=") {
			env[i] = "GH_NO_UPDATE_NOTIFIER=1"
			return
		}
	}
	env = append(env, "GH_NO_UPDATE_NOTIFIER=1")
}
