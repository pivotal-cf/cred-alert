package revok_test

import (
	"cred-alert/revok"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc/grpclog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	git "gopkg.in/libgit2/git2go.v25"

	"testing"
)

func init() {
	grpclog.SetLogger(log.New(ioutil.Discard, "", 0))
}

func TestRevok(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Revok Suite")
}

type createCommitResult struct {
	From *git.Oid
	To   *git.Oid
}

func createCommit(refName, repoPath, filePath string, contents []byte, commitMsg string, parentOid *git.Oid) createCommitResult {
	err := ioutil.WriteFile(filepath.Join(repoPath, filePath), contents, os.ModePerm)

	repo, err := git.OpenRepository(repoPath)
	Expect(err).NotTo(HaveOccurred())
	defer repo.Free()

	if parentOid == nil {
		referenceIterator, err := repo.NewReferenceIterator()
		Expect(err).NotTo(HaveOccurred())
		defer referenceIterator.Free()

		for {
			ref, err := referenceIterator.Next()
			if git.IsErrorCode(err, git.ErrIterOver) {
				break
			}
			Expect(err).NotTo(HaveOccurred())
			defer ref.Free()

			if ref.Name() == refName {
				parentOid = ref.Target()
				break
			}
		}
	}

	index, err := repo.Index()
	Expect(err).NotTo(HaveOccurred())
	defer index.Free()

	err = index.AddByPath(filePath)
	Expect(err).NotTo(HaveOccurred())

	treeOid, err := index.WriteTree()
	Expect(err).NotTo(HaveOccurred())

	tree, err := repo.LookupTree(treeOid)
	Expect(err).NotTo(HaveOccurred())
	defer tree.Free()

	sig := &git.Signature{
		Name:  "revok-test",
		Email: "revok-test@localhost",
		When:  time.Now(),
	}

	var newOid *git.Oid
	var parent *git.Commit
	if parentOid != nil {
		parentObject, err := repo.Lookup(parentOid)
		Expect(err).NotTo(HaveOccurred())
		defer parentObject.Free()

		parent, err = parentObject.AsCommit()
		Expect(err).NotTo(HaveOccurred())
		defer parent.Free()

		newOid, err = repo.CreateCommit(refName, sig, sig, commitMsg, tree, parent)
		Expect(err).NotTo(HaveOccurred())
	} else {
		newOid, err = repo.CreateCommit(refName, sig, sig, commitMsg, tree)
		Expect(err).NotTo(HaveOccurred())
	}

	if parent != nil {
		return createCommitResult{
			From: parent.Id(),
			To:   newOid,
		}
	}

	root, err := git.NewOid("0000000000000000000000000000000000000000")
	Expect(err).NotTo(HaveOccurred())

	return createCommitResult{
		From: root,
		To:   newOid,
	}
}

func createMerge(oid1, oid2 *git.Oid, repoPath string) createCommitResult {
	repo, err := git.OpenRepository(repoPath)
	Expect(err).NotTo(HaveOccurred())
	defer repo.Free()

	object, err := repo.Lookup(oid1)
	Expect(err).NotTo(HaveOccurred())
	defer object.Free()

	oid1Commit, err := object.AsCommit()
	Expect(err).NotTo(HaveOccurred())
	defer oid1Commit.Free()

	object, err = repo.Lookup(oid2)
	Expect(err).NotTo(HaveOccurred())
	defer object.Free()

	oid2Commit, err := object.AsCommit()
	Expect(err).NotTo(HaveOccurred())

	defer oid2Commit.Free()

	mergeOptions, err := git.DefaultMergeOptions()
	Expect(err).NotTo(HaveOccurred())

	idx, err := repo.MergeCommits(oid1Commit, oid2Commit, &mergeOptions)
	treeOid, err := idx.WriteTreeTo(repo)
	Expect(err).NotTo(HaveOccurred())

	tree, err := repo.LookupTree(treeOid)
	Expect(err).NotTo(HaveOccurred())
	defer tree.Free()

	sig := &git.Signature{
		Name:  "revok-test",
		Email: "revok-test@localhost",
		When:  time.Now(),
	}

	newOid, err := repo.CreateCommit("refs/heads/master", sig, sig, "merged", tree, oid1Commit, oid2Commit)
	Expect(err).NotTo(HaveOccurred())

	return createCommitResult{
		To: newOid,
	}
}

var org1Repo1 = revok.GitHubRepository{
	Owner:         "some-org",
	Name:          "org-1-repo-1",
	SSHURL:        "org-1-repo-1-ssh-url",
	Private:       true,
	DefaultBranch: "org-1-repo-1-branch",
	RawJSON:       []byte(`{"some-key":"some-value"}`),
}

var org1Repo2 = revok.GitHubRepository{
	Owner:         "some-org",
	Name:          "org-1-repo-2",
	SSHURL:        "org-1-repo-2-ssh-url",
	Private:       true,
	DefaultBranch: "org-1-repo-2-branch",
	RawJSON:       []byte(`{"some-key":"some-value"}`),
}

var org2Repo1 = revok.GitHubRepository{
	Owner:         "some-other-org",
	Name:          "org-2-repo-1",
	SSHURL:        "org-2-repo-1-ssh-url",
	Private:       true,
	DefaultBranch: "org-2-repo-1-branch",
	RawJSON:       []byte(`{"some-key":"some-value"}`),
}

var org2Repo2 = revok.GitHubRepository{
	Owner:         "some-other-org",
	Name:          "org-2-repo-2",
	SSHURL:        "org-2-repo-2-ssh-url",
	Private:       true,
	DefaultBranch: "org-2-repo-2-branch",
	RawJSON:       []byte(`{"some-key":"some-value"}`),
}
