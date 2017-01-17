package gitclient

import (
	"path/filepath"

	git "github.com/libgit2/git2go"
)

type looper struct {
}

//go:generate counterfeiter . Looper

type Looper interface {
	ScanCurrentState(repositoryPath string, callback ScanCallback) error
}

type ScanCallback func(sha string, path string, content []byte)

func NewLooper() *looper {
	return &looper{}
}

func (l *looper) ScanCurrentState(repositoryPath string, callback ScanCallback) error {
	repo, err := git.OpenRepository(repositoryPath)
	if err != nil {
		return err
	}
	defer repo.Free()

	it, err := repo.NewBranchIterator(git.BranchRemote)
	if err != nil {
		return err
	}
	defer it.Free()

	var branch *git.Branch
	var target *git.Oid
	var commit *git.Commit
	var tree *git.Tree
	var blob *git.Blob

	for {
		branch, _, err = it.Next()
		if err != nil {
			break
		}

		target = branch.Target()
		if target == nil {
			continue
		}

		commit, err = repo.LookupCommit(target)
		if err != nil {
			return err
		}
		commitStr := commit.Id().String()

		tree, err = commit.Tree()
		if err != nil {
			return err
		}

		err = tree.Walk(func(root string, entry *git.TreeEntry) int {
			if entry.Type == git.ObjectBlob {
				blob, err = repo.LookupBlob(entry.Id)
				if err != nil {
					return -1
				}

				path := filepath.Join(root, entry.Name)
				callback(commitStr, path, blob.Contents())
			}

			return 0
		})

		if err != nil {
			return err
		}
	}

	if blob != nil {
		blob.Free()
	}

	if tree != nil {
		tree.Free()
	}

	if commit != nil {
		commit.Free()
	}

	if branch != nil {
		branch.Free()
	}

	return nil
}
