# GitHub Committer

This is a simple utility which creates a __signed__ commit using the GitHub graphql API.
It uses the `GITHUB_TOKEN` environment variable with an action to authenticate. 

## Installation

```
go install github.com/Harry-Moore-dev/github-committer@latest
```

## Usage

```help
Usage:
github-committer [OPTIONS]

Application Options:
-r, --repository= the repository to push commits to
-b, --branch=     the branch to push commits to (creates new branch if named branch doesn't exist)
-m, --message=    the commit message to use (default: updated with github-signer)
-p, --prmake=     automatically raises a pull request if set (default: false)

Help Options:
-h, --help        Show this help message
```

## Example
0
```
github-committer -r Harry-Moore-dev/github-committer -b branchname -m 'example commit' -p
```