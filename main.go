package main

import (
	"context"
	"encoding/base64"
	"log"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/jessevdk/go-flags"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

type Opts struct {
	Repository  string `short:"r" long:"repository" description:"the repository to push commits to" required:"true"`
	BranchName  string `short:"b" long:"branch" description:"the branch to push commits to" required:"true"`
	Message     string `short:"m" long:"message" description:"the commit message to use" default:"updated with github-signer"`
	PullRequest bool   `short:"p" long:"prmake" description:"automatically raises a pull request if set"`
}

func main() {
	ctx := context.Background()

	var opts Opts
	parser := flags.NewParser(&opts, flags.Default)
	_, err := parser.Parse()
	switch e := err.(type) {
	case *flags.Error:
		if e.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	case nil:
		break
	default:
		log.Fatal(err)
	}

	repo, status, err := openRepository()
	if err != nil {
		log.Fatalf("unable to open repository: %s", err)
	}
	changes := addChanges(status)

	client := createGhClient()

	oid, repoId, err := getMainOid(ctx, client, opts)
	if err != nil {
		log.Fatalf("unable to lookup oid: %s", err)
	}

	branchExists, err := CheckBranchExists(ctx, client, opts)
	if err != nil {
		log.Fatalf("unable to lookup branch: %s", err)
	}

	var revision *plumbing.Reference
	if !branchExists {
		err = CreateBranch(ctx, client, opts, repoId, oid)
		if err != nil {
			log.Fatalf("unable to create branch: %s", err)
		}
		revision, err = repo.Head()
		if err != nil {
			log.Fatalf("unable to find HEAD revision: %s", err)
		}
	} else {
		err = fetchRemote(repo)
		if err != nil {
			log.Printf("unable to fetch remote: %s", err)
		}
		refName := plumbing.ReferenceName("refs/remotes/origin/" + opts.BranchName)
		revision, err = repo.Reference(refName, true)
		if err != nil {
			log.Fatalf("unable to find HEAD for branch %s: %s", opts.BranchName, err)
		}
	}

	err = DoCommit(ctx, client, changes, opts, revision)
	if err != nil {
		log.Fatalf("unable to commit: %s", err)
	}

	if opts.PullRequest {
		err = CreatePullRequest(ctx, client, opts, repoId)
		if err != nil {
			log.Fatalf("unable to create PR: %s", err)
		}
	}
}

func openRepository() (*git.Repository, git.Status, error) {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return nil, nil, err
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return nil, nil, err
	}
	status, err := worktree.Status()
	if err != nil {
		return nil, nil, err
	}
	return repo, status, nil
}

func fetchRemote(repo *git.Repository) error {
	remote, err := repo.Remote("origin")
	if err != nil {
		return err
	}
	err = remote.Fetch(&git.FetchOptions{})
	if err != nil {
		return err
	}
	return nil
}

func addChanges(status git.Status) *[]githubv4.FileAddition {
	changes := &[]githubv4.FileAddition{}
	for name, status := range status {
		if status.Worktree == git.Modified || status.Staging == git.Added || status.Staging == git.Modified {
			log.Printf("adding %s", name)
			b, _ := os.ReadFile(name)
			content := base64.StdEncoding.EncodeToString(b)
			*changes = append(*changes, githubv4.FileAddition{
				Path:     githubv4.String(name),
				Contents: githubv4.Base64String(content),
			})
		}
	}
	if len(*changes) == 0 {
		log.Printf("no changes to commit, exiting")
		os.Exit(0)
	}
	return changes
}

func createGhClient() *githubv4.Client {
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	httpClient := oauth2.NewClient(context.Background(), src)

	client := githubv4.NewClient(httpClient)
	return client
}
