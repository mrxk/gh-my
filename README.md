# Github my plugin

This plugin provides a [bubbletea](https://github.com/charmbracelet/bubbletea)
TUI for viewing all your pull requests in the terminal. It is useful when you
are part of many different organizations and projects and want to see all your
PRs in one place.

## Install

This plugin requires that the [github cli](https://cli.github.com/) be installed
and configured.

```bash
gh extension install github.com/mrxk/gh-my
```

## Uninstall

```bash
gh extension remove my
```

## Usage

```bash
My Github Plugin: my

Usage:
        my [(prs|requests|all)] [options]

Options:
        -d, --include-drafts               Include draft PRs.
        -c, --include-closed               Include closed PRs.
        -w <interval>, --watch=<interval>  Poll every <interval>.
```

### gh my prs

The `gh my prs` command will show PRs created by the current user.

### gh my requests

The `gh my requests` command will show PRs that the current user has been
requested to review.

### gh my all

The `gh my all` command will show all PRs for all configured repositories.

### Key bindings

* `[esc]`: Exit the application.
* `[enter]`: Open the selected PR in the default browser.
* `[tab]`: Show the next view.
* `[shift-tab]`: Show the previous view.
* `r`: Reload PRs.
* `w`: Show more columns (wide view).
* `[up]|k`: Move up a line in the PR list.
* `[down]|j`: Move down a line in the PR list.
* `[pgup]`: Move up a page in the PR list.
* `[pgdn]`: Move down a page in the PR list.
* `[home]|g`: Go to the top of the list.
* `[end]|G`: Go to the bottom of the list.

## Configuration

This plugin reads a JSON file from `${XDG_CONFIG_HOME}/gh-my/config.json`. If
`${XDG_CONFIG_HOME}` is not set then `${HOME}/.config/gh-my/config.json` is
used. This JSON file contains a single object with the following fields (all are
optional).

* `includeClosed`: Bool. When true, closed PRs are included by default.
* `includeDrafts`: Bool. When true, draft PRs are included by default.
* `repositories`: String array. The listed repositories will be queried for the
  `all` view.
* `defaultView`: String array. The listed columns will be included in the
  default view. Default [ "checks", "mergeable", "approved", "title", "author",
  "repository", "change", "updatedAt" ].
* `wideView`: String array. The listed columns will be included in the wide
  view. Default [ "checks", "mergeable", "approved", "draft", "title", "url",
  "author", "repository", "change", "state", "comments", "updatedAt" ].

Valid view columns include the following:
* approved
* author
* change
* checks
* comments
* draft
* mergable
* repository
* state
* title
* updatedAt
* url

### Example

```
{
    "includeClosed": false,
    "includeDrafts": true,
    "repositories": [
        "org1/repo1",
        "org1/repo2",
        "org2/repo1"
    ],
    "defaultView": [ "checks", "mergeable", "approved", "title", "repository" ]
}
```

## Columns

By default the following columns are displayed.

* `C`: The PR checks status.
  * ✅: All checks have passed.
  * ❌: One or more checks have failed.
  * ⏳: Checks are running.
* `M`: The PR mergeability status.
  * ✅: The PR can be merged cleanly.
  * ❌: The PR is conflicting.
  * ⬆️: The PR is behind.
* `A`: The PR approval status.
  * ✅: The PR is approved.
* `Title`: The title of the PR.
* `Author`: The author of the PR.
* `Repository`: The repository name with the organization stripped.
* `Change`: The number of files changed in the PR along with the addition and
  deletion counts.
* `UpdatedAt`: The time this PR was last updated.

When wide mode is enabled (by pressing the `w` key) the following additional
columns are displayed.

* `Url`: The URL of the PR.
* `Draft`: If the PR is a draft, the value of this column will be 📝.
* `State`: If the PR is merged, the value of this column will be 🚀. If the PR
  was closed without being merged, the value will be 🗑.
* `Comments`: The number of comments on the PR.
