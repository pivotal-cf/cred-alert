package gitclient

import (
	"fmt"
	"strings"

	git "github.com/libgit2/git2go"
)

const defaultRemoteName = "origin"

type client struct {
	cloneOptions *git.CloneOptions
}

//go:generate counterfeiter . Client

type Client interface {
	Clone(string, string) (*git.Repository, error)
	GetParents(*git.Repository, *git.Oid) ([]*git.Oid, error)
	Fetch(string) (map[string][]*git.Oid, error)
	HardReset(string, *git.Oid) error
	Diff(repositoryPath string, a, b *git.Oid) (string, error)
}

func New(privateKeyPath, publicKeyPath string) *client {
	credentialsCallback := newCredentialsCallback(privateKeyPath, publicKeyPath)
	return &client{
		cloneOptions: &git.CloneOptions{
			FetchOptions: &git.FetchOptions{
				UpdateFetchhead: true,
				RemoteCallbacks: git.RemoteCallbacks{
					CredentialsCallback:      credentialsCallback,
					CertificateCheckCallback: certificateCheckCallback,
				},
			},
		},
	}
}

func (c *client) Clone(sshURL, dest string) (*git.Repository, error) {
	return git.Clone(sshURL, dest, c.cloneOptions)
}

func (c *client) GetParents(repo *git.Repository, child *git.Oid) ([]*git.Oid, error) {
	object, err := repo.Lookup(child)
	if err != nil {
		return nil, err
	}
	defer object.Free()

	commit, err := object.AsCommit()
	if err != nil {
		return nil, err
	}
	defer commit.Free()

	var parents []*git.Oid
	var i uint
	for i = 0; i < commit.ParentCount(); i++ {
		parents = append(parents, commit.ParentId(i))
	}

	return parents, nil
}

func (c *client) Fetch(repositoryPath string) (map[string][]*git.Oid, error) {
	repo, err := git.OpenRepository(repositoryPath)
	if err != nil {
		return nil, err
	}
	defer repo.Free()

	remote, err := repo.Remotes.Lookup(defaultRemoteName)
	if err != nil {
		return nil, err
	}
	defer remote.Free()

	changes := map[string][]*git.Oid{}
	updateTipsCallback := func(refname string, a *git.Oid, b *git.Oid) git.ErrorCode {
		changes[refname] = []*git.Oid{a, b}
		return 0
	}

	// bleh
	c.cloneOptions.FetchOptions.RemoteCallbacks.UpdateTipsCallback = updateTipsCallback

	var msg string
	err = remote.Fetch([]string{}, c.cloneOptions.FetchOptions, msg)
	if err != nil {
		return nil, err
	}

	return changes, nil
}

func (c *client) HardReset(repositoryPath string, oid *git.Oid) error {
	repo, err := git.OpenRepository(repositoryPath)
	if err != nil {
		return err
	}
	defer repo.Free()

	object, err := repo.Lookup(oid)
	if err != nil {
		return err
	}
	defer object.Free()

	commit, err := object.AsCommit()
	if err != nil {
		return err
	}
	defer commit.Free()

	return repo.ResetToCommit(commit, git.ResetHard, &git.CheckoutOpts{
		Strategy: git.CheckoutForce,
	})
}

func newCredentialsCallback(privateKeyPath, publicKeyPath string) git.CredentialsCallback {
	return func(url string, username string, allowedTypes git.CredType) (git.ErrorCode, *git.Cred) {
		passphrase := ""
		ret, cred := git.NewCredSshKey(username, publicKeyPath, privateKeyPath, passphrase)
		if ret != 0 {
			fmt.Printf("ret: %d\n", ret)
		}
		return git.ErrorCode(ret), &cred
	}
}

func certificateCheckCallback(cert *git.Certificate, valid bool, hostname string) git.ErrorCode {
	// should return an error code if the cert isn't valid
	return 0
}

func (c *client) Diff(repositoryPath string, parent, child *git.Oid) (string, error) {
	repo, err := git.OpenRepository(repositoryPath)
	if err != nil {
		return "", err
	}
	defer repo.Free()

	var aTree *git.Tree
	if parent != nil {
		var err error
		aTree, err = objectToTree(repo, parent)
		if err != nil {
			return "", err
		}
		defer aTree.Free()
	}

	bTree, err := objectToTree(repo, child)
	if err != nil {
		return "", err
	}
	defer bTree.Free()

	options, err := git.DefaultDiffOptions()
	if err != nil {
		return "", err
	}

	diff, err := repo.DiffTreeToTree(aTree, bTree, &options)
	if err != nil {
		return "", err
	}
	defer diff.Free()

	numDeltas, err := diff.NumDeltas()
	if err != nil {
		return "", err
	}

	var results []string
	for i := 0; i < numDeltas; i++ {
		patch, err := diff.Patch(i)
		if err != nil {
			return "", err
		}
		patchString, err := patch.String()
		if err != nil {
			return "", err
		}
		patch.Free()

		results = append(results, patchString)
	}

	return strings.Join(results, "\n"), nil
}

func objectToTree(repo *git.Repository, oid *git.Oid) (*git.Tree, error) {
	object, err := repo.Lookup(oid)
	if err != nil {
		return nil, err
	}
	defer object.Free()

	commit, err := object.AsCommit()
	if err != nil {
		return nil, err
	}
	defer commit.Free()

	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}

	return tree, nil
}
