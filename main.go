package main

import (
	"context"
	"encoding/base64"
	"log"
	"os"
	"strings"

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

	repo, err := git.PlainOpen(".")
	if err != nil {
		log.Fatalf("unable to open repository: %s", err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		log.Fatalf("unable to open repository: %s", err)
	}
	status, err := worktree.Status()
	if err != nil {
		log.Fatalf("unable to open repository: %s", err)
	}
	changes := AddChanges(status)

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
		revision, err = repo.Reference("refs/head/main", true)
		if err != nil {
			log.Fatalf("unable to find HEAD for branch main: %s", err)
		}
	} else {
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
		CreatePullRequest(ctx, client, opts, repoId)
		if err != nil {
			log.Fatalf("unable to create PR: %s", err)
		}
	}
}

func AddChanges(status git.Status) *[]githubv4.FileAddition {
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

func DoCommit(ctx context.Context, client *githubv4.Client, changes *[]githubv4.FileAddition, opts Opts, revision *plumbing.Reference) error {
	var mutation struct {
		CreateCommitOnBranch struct {
			Commit struct {
				Url githubv4.ID
			}
		} `graphql:"createCommitOnBranch(input: $input)"`
	}
	input := githubv4.CreateCommitOnBranchInput{
		Branch: githubv4.CommittableBranch{
			RepositoryNameWithOwner: githubv4.NewString(githubv4.String(opts.Repository)),
			BranchName:              githubv4.NewString(githubv4.String(opts.BranchName)),
		},
		Message: githubv4.CommitMessage{Headline: githubv4.String(opts.Message)},
		FileChanges: &githubv4.FileChanges{
			Additions: changes,
		},
		ExpectedHeadOid: githubv4.GitObjectID(revision.Hash().String()),
	}

	err := client.Mutate(ctx, &mutation, input, nil)
	if err != nil {
		return err
	}
	log.Printf("mutation complete: %s", mutation.CreateCommitOnBranch.Commit.Url)
	return nil
}

func CheckBranchExists(ctx context.Context, client *githubv4.Client, opts Opts) (bool, error) {
	var query struct {
		Repository struct {
			Ref struct {
				Name string
			} `graphql:"ref(qualifiedName: $branchName)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}
	variables := map[string]interface{}{
		"owner":      githubv4.String(strings.Split(opts.Repository, "/")[0]),
		"name":       githubv4.String(strings.Split(opts.Repository, "/")[1]),
		"branchName": githubv4.String("refs/heads/" + opts.BranchName),
	}

	err := client.Query(ctx, &query, variables)
	if err != nil {
		return false, err
	}

	if query.Repository.Ref.Name != "" {
		log.Printf("branch found: %s", query.Repository.Ref.Name)
		return true, nil
	} else {
		log.Printf("a branch with the name %s was not found", opts.BranchName)
		return false, nil
	}
}

func getMainOid(ctx context.Context, client *githubv4.Client, opts Opts) (githubv4.GitObjectID, githubv4.ID, error) {
	var query struct {
		Repository struct {
			ID  githubv4.ID
			Ref struct {
				Target struct {
					Oid githubv4.GitObjectID
				}
			} `graphql:"ref(qualifiedName: \"refs/heads/main\")"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}
	variables := map[string]interface{}{
		"owner": githubv4.String(strings.Split(opts.Repository, "/")[0]),
		"name":  githubv4.String(strings.Split(opts.Repository, "/")[1]),
	}

	err := client.Query(ctx, &query, variables)
	if err != nil {
		return "", "", err
	}
	return query.Repository.Ref.Target.Oid, query.Repository.ID, nil
}

func CreateBranch(ctx context.Context, client *githubv4.Client, opts Opts, repoId githubv4.ID, oid githubv4.GitObjectID) error {
	var mutation struct {
		CreateRef struct {
			ClientMutationID githubv4.String
		} `graphql:"createRef(input: $input)"`
	}
	input := githubv4.CreateRefInput{
		RepositoryID: repoId,
		Name:         githubv4.String("refs/heads/" + opts.BranchName),
		Oid:          oid,
	}

	err := client.Mutate(ctx, &mutation, input, nil)
	if err != nil {
		return err
	}
	log.Printf("%s branch created\n", opts.BranchName)
	return nil
}

func CreatePullRequest(ctx context.Context, client *githubv4.Client, opts Opts, repoId githubv4.ID) error {
	var mutation struct {
		CreatePullRequest struct {
			PullRequest struct {
				ID githubv4.ID
			}
		} `graphql:"createPullRequest(input: $input)"`
	}
	input := githubv4.CreatePullRequestInput{
		RepositoryID: repoId,
		BaseRefName:  "main",
		HeadRefName:  githubv4.String(opts.BranchName),
		Title:        githubv4.String(opts.Message),
	}

	err := client.Mutate(ctx, &mutation, input, nil)
	if err != nil {
		return err
	}
	log.Printf("pull request created %s\n", opts.BranchName)
	return nil
}
