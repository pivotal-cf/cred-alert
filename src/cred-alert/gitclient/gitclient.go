package gitclient

import (
	"bufio"
	"bytes"
	"cred-alert/mimetype"
	"cred-alert/scanners"
	"cred-alert/scanners/filescanner"
	"cred-alert/sniff"
	"errors"
	"fmt"
	"strings"

	"github.com/BurntSushi/locker"

	"code.cloudfoundry.org/lager"

	git "gopkg.in/libgit2/git2go.v24"
)

const defaultRemoteName = "origin"

var ErrInterrupted = errors.New("interrupted")

type client struct {
	privateKeyPath string
	publicKeyPath  string
	locker         *locker.Locker
}

func New(privateKeyPath, publicKeyPath string) *client {
	return &client{
		privateKeyPath: privateKeyPath,
		publicKeyPath:  publicKeyPath,
		locker:         locker.NewLocker(),
	}
}

func (c *client) BranchTargets(repoPath string) (map[string]string, error) {
	c.locker.RLock(repoPath)
	defer c.locker.RUnlock(repoPath)

	repo, err := git.OpenRepository(repoPath)
	if err != nil {
		return nil, err
	}
	defer repo.Free()

	it, err := repo.NewBranchIterator(git.BranchAll)
	if err != nil {
		return nil, err
	}

	var branch *git.Branch
	branches := map[string]string{}
	for {
		branch, _, err = it.Next()
		if err != nil {
			break
		}

		branchName, err := branch.Name()
		if err != nil {
			break
		}

		target := branch.Target()
		if target == nil { // origin/HEAD has no target
			continue
		}

		branches[branchName] = branch.Target().String()
	}

	if branch != nil {
		branch.Free()
	}

	return branches, nil
}

func (c *client) Clone(sshURL, repoPath string) error {
	c.locker.Lock(repoPath)
	defer c.locker.Unlock(repoPath)

	cloneOptions := &git.CloneOptions{
		FetchOptions: newFetchOptions(c.privateKeyPath, c.publicKeyPath),
	}

	_, err := git.Clone(sshURL, repoPath, cloneOptions)

	return err
}

func (c *client) GetParents(repoPath, childSha string) ([]string, error) {
	c.locker.RLock(repoPath)
	defer c.locker.RUnlock(repoPath)

	repo, err := git.OpenRepository(repoPath)
	if err != nil {
		return nil, err
	}

	child, err := git.NewOid(childSha)
	if err != nil {
		return nil, err
	}

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

	parents := []string{}

	for i := uint(0); i < commit.ParentCount(); i++ {
		parents = append(parents, commit.ParentId(i).String())
	}

	return parents, nil
}

const nullGitObjectID = "0000000000000000000000000000000000000000"

func (c *client) Fetch(repoPath string) (map[string][]string, error) {
	c.locker.Lock(repoPath)
	defer c.locker.Unlock(repoPath)

	repo, err := git.OpenRepository(repoPath)
	if err != nil {
		return nil, err
	}
	defer repo.Free()

	remote, err := repo.Remotes.Lookup(defaultRemoteName)
	if err != nil {
		return nil, err
	}
	defer remote.Free()

	changes := map[string][]string{}
	updateTipsCallback := func(refname string, a *git.Oid, b *git.Oid) git.ErrorCode {
		if a.String() != nullGitObjectID && b.String() != nullGitObjectID {
			changes[refname] = []string{a.String(), b.String()}
		}

		return 0
	}

	fetchOptions := newFetchOptions(c.privateKeyPath, c.publicKeyPath)
	fetchOptions.RemoteCallbacks.UpdateTipsCallback = updateTipsCallback

	var msg string
	err = remote.Fetch([]string{}, fetchOptions, msg)
	if err != nil {
		return nil, err
	}

	return changes, nil
}

func (c *client) HardReset(repoPath, sha string) error {
	c.locker.Lock(repoPath)
	defer c.locker.Unlock(repoPath)

	repo, err := git.OpenRepository(repoPath)
	if err != nil {
		return err
	}
	defer repo.Free()

	oid, err := git.NewOid(sha)
	if err != nil {
		return err
	}

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

func (c *client) Diff(repoPath, parent, child string) (string, error) {
	c.locker.RLock(repoPath)
	defer c.locker.RUnlock(repoPath)

	repo, err := git.OpenRepository(repoPath)
	if err != nil {
		return "", err
	}
	defer repo.Free()

	var aTree *git.Tree
	if parent != "" {
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

func (c *client) BranchCredentialCounts(
	logger lager.Logger,
	repoPath string,
	sniffer sniff.Sniffer,
) (map[string]uint, error) {
	c.locker.RLock(repoPath)
	defer c.locker.RUnlock(repoPath)

	repo, err := git.OpenRepository(repoPath)
	if err != nil {
		return nil, err
	}
	defer repo.Free()

	it, err := repo.NewBranchIterator(git.BranchRemote)
	if err != nil {
		return nil, err
	}
	defer it.Free()

	var branch *git.Branch
	var target *git.Oid
	var commit *git.Commit
	var tree *git.Tree
	var blob *git.Blob

	branchCounter := newBranchCounter(logger, repo, sniffer)

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
			return nil, err
		}

		tree, err = commit.Tree()
		if err != nil {
			return nil, err
		}

		branchName, err := branch.Name()
		if err != nil {
			return nil, err
		}

		err = tree.Walk(branchCounter.forBranch(branchName))

		if err != nil {
			return nil, err
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

	return branchCounter.branchCounts, nil
}

func objectToTree(repo *git.Repository, sha string) (*git.Tree, error) {
	oid, err := git.NewOid(sha)
	if err != nil {
		return nil, err
	}

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
	return git.ErrorCode(0)
}

func newFetchOptions(privateKeyPath, publicKeyPath string) *git.FetchOptions {
	credentialsCallback := newCredentialsCallback(privateKeyPath, publicKeyPath)

	return &git.FetchOptions{
		UpdateFetchhead: true,
		RemoteCallbacks: git.RemoteCallbacks{
			CredentialsCallback:      credentialsCallback,
			CertificateCheckCallback: certificateCheckCallback,
		},
	}
}

type branchCounter struct {
	entryCounts  map[git.Oid]uint
	branchCounts map[string]uint

	logger  lager.Logger
	repo    *git.Repository
	sniffer sniff.Sniffer
}

func newBranchCounter(logger lager.Logger, repo *git.Repository, sniffer sniff.Sniffer) *branchCounter {
	return &branchCounter{
		entryCounts:  make(map[git.Oid]uint),
		branchCounts: make(map[string]uint),

		logger:  logger,
		repo:    repo,
		sniffer: sniffer,
	}
}

func (b *branchCounter) forBranch(branchName string) git.TreeWalkCallback {
	return func(root string, entry *git.TreeEntry) int {
		if entry.Type == git.ObjectBlob {
			if count, ok := b.entryCounts[*entry.Id]; ok {
				if count > 0 {
					b.branchCounts[branchName] += count
				}
				return 0
			}

			blob, err := b.repo.LookupBlob(entry.Id)
			if err != nil {
				return -1
			}

			var count uint
			r := bufio.NewReader(bytes.NewReader(blob.Contents()))
			mime := mimetype.Mimetype(b.logger, r)
			if mime == "" || strings.HasPrefix(mime, "text") {
				b.sniffer.Sniff(
					b.logger,
					filescanner.New(r, entry.Name),
					func(lager.Logger, scanners.Violation) error {
						count++
						return nil
					},
				)
			}

			b.entryCounts[*entry.Id] = count
			b.branchCounts[branchName] += count
		}

		return 0
	}
}
