# GitHub Committer

This is a simple utility which creates a __signed__ commit using the GitHub graphql API.
It uses the `GITHUB_TOKEN` environment variable with an action to authenticate. 

## Installation

```
go install github.com/Harry-Moore-dev/github-commiter@latest
```

## Usage

```help
Usage:
github-committer [OPTIONS]

Application Options:
-r, --repository= the repository to push commits to
-b, --branch=     the branch to push commits to (creates new branch if named branch doesn't exist)
-m, --message=    the commit message to use (default: updated with github-signer)

Help Options:
-h, --help        Show this help message
```

## Example

```
github-committer -r iwarapter/example -b main -m 'example commit message'
```