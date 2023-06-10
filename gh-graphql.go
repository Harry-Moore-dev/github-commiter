package main

import (
	"context"
	"log"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/shurcooL/githubv4"
)

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
