package gitclient

import (
	"cred-alert/db"

	"path/filepath"

	git "github.com/libgit2/git2go"
)

type looper struct {
	repoRepository db.RepositoryRepository
}

type Looper interface {
	ScanCurrentState(owner string, repository string, callback ScanCallback) error
}

type ScanCallback func(sha string, path string, content []byte)

func NewLooper(
	repoRepository db.RepositoryRepository,
) *looper {
	return &looper{
		repoRepository: repoRepository,
	}
}

func (l *looper) ScanCurrentState(owner string, repository string, callback ScanCallback) error {
	repo, _ := l.repoRepository.Find(owner, repository)

	return walk(repo.Path, git.BranchLocal, callback)
}

func walk(repositoryPath string, branchType git.BranchType, callback ScanCallback) error {
	repo, err := git.OpenRepository(repositoryPath)
	if err != nil {
		return err
	}
	defer repo.Free()

	it, err := repo.NewBranchIterator(branchType)
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
