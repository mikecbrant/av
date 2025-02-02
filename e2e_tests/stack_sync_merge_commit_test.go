package e2e_tests

import (
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/aviator-co/av/internal/meta"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestStackSyncMergeCommit(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.Dir())

	// To start, we create a simple two-stack where each stack has a single commit.
	// Our stack looks like:
	//     stack-1: main -> 1a -> 2b
	//     stack-2:                \ -> 2a -> 2b
	require.Equal(t, 0, Cmd(t, "git", "checkout", "-b", "stack-1").ExitCode)
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n"), gittest.WithMessage("Commit 1a"))
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n1b\n"), gittest.WithMessage("Commit 1b"))
	RequireAv(t, "stack", "branch", "stack-2")
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n1b\n2a\n"), gittest.WithMessage("Commit 2a"))
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n1b\n2a\n2b\n"), gittest.WithMessage("Commit 2b"))

	// Everything up to date now, so this should be a no-op.
	require.Equal(t, 0, Av(t, "stack", "sync", "--no-fetch", "--no-push").ExitCode)

	// We simulate a merge here so that our history looks like:
	//     main:    X            / -> 1S
	//     stack-1:  \ -> 1a -> 2b
	//     stack-2:              \ -> 2a -> 2b
	// where 1S is the squash-merge commit of 2b onto main. Note that since it's
	// a squash commit, 1S is not a *merge commit* in the Git definition.
	var squashCommit string
	gittest.WithCheckoutBranch(t, repo, "main", func() {
		oldHead, err := repo.RevParse(&git.RevParse{Rev: "HEAD"})
		require.NoError(t, err, "failed to get HEAD")

		RequireCmd(t, "git", "merge", "--squash", "stack-1")
		// `git merge --squash` doesn't actually create the commit, so we have to
		// do that separately.
		RequireCmd(t, "git", "commit", "--no-edit")
		squashCommit, err = repo.RevParse(&git.RevParse{Rev: "HEAD"})
		require.NoError(t, err, "failed to get squash commit")
		require.NotEqual(t, oldHead, squashCommit, "squash commit should be different from old HEAD")
	})

	stack1Meta, _ := meta.ReadBranch(repo, "stack-1")
	stack1Meta.MergeCommit = squashCommit
	require.NoError(t, meta.WriteBranch(repo, stack1Meta))

	require.Equal(t, 0,
		Cmd(t, "git", "merge-base", "--is-ancestor", "stack-1", "stack-2").ExitCode,
		"HEAD of stack-1 should be an ancestor of HEAD of stack-2 before running sync",
	)
	require.NotEqual(t, 0,
		Cmd(t, "git", "merge-base", "--is-ancestor", squashCommit, "stack-2").ExitCode,
		"squash commit of stack-1 should not be an ancestor of HEAD of stack-1 before running sync",
	)

	RequireAv(t, "stack", "sync", "--no-fetch", "--no-push")

	require.Equal(t, 0,
		Cmd(t, "git", "merge-base", "--is-ancestor", squashCommit, "stack-2").ExitCode,
		"squash commit of stack-1 should be an ancestor of HEAD of stack-1 after running sync",
	)
}
