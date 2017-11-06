package gitclient

import "gopkg.in/libgit2/git2go.v26"

//go:generate counterfeiter . FileLookup

type FileLookup interface {
	FileContents(repoPath string, branch string, filePath string) ([]byte, error)
}

func NewFileLookup() FileLookup {
	return &fileLookup{}
}

type fileLookup struct{}

func (f *fileLookup) FileContents(repoPath string, branchName string, path string) ([]byte, error) {
	repo, err := git.OpenRepository(repoPath)
	if err != nil {
		return nil, err
	}
	defer repo.Free()

	branch, err := repo.LookupBranch("origin/"+branchName, git.BranchRemote)
	if err != nil {
		return nil, err
	}
	defer branch.Free()

	ref, err := branch.Resolve()
	if err != nil {
		return nil, err
	}
	defer ref.Free()

	refTarget := branch.Target()

	commit, err := repo.LookupCommit(refTarget)
	if err != nil {
		return nil, err
	}
	defer commit.Free()

	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}
	defer tree.Free()

	treeEntry, err := tree.EntryByPath(path)
	if err != nil {
		return nil, err
	}

	blob, err := repo.LookupBlob(treeEntry.Id)
	if err != nil {
		return nil, err
	}
	defer blob.Free()

	return blob.Contents(), nil
}
