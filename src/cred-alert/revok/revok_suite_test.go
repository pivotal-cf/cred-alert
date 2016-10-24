package revok_test

import (
	"cred-alert/revok"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	git "github.com/libgit2/git2go"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

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

var boshSampleReleaseRepository = revok.GitHubRepository{
	Name:          "bosh-sample-release",
	Owner:         "cloudfoundry",
	SSHURL:        "git@github.com:cloudfoundry/bosh-sample-release.git",
	Private:       false,
	DefaultBranch: "master",
	RawJSON:       []byte(boshSampleReleaseRepositoryJSON),
}

var cfMessageBusRepository = revok.GitHubRepository{
	Name:          "cf-message-bus",
	Owner:         "cloudfoundry",
	SSHURL:        "git@github.com:cloudfoundry/cf-message-bus.git",
	Private:       false,
	DefaultBranch: "master",
	RawJSON:       []byte(cfMessageBusJSON),
}

var boshSampleReleaseRepositoryJSON = `{
	"id": 3953650,
	"name": "bosh-sample-release",
	"full_name": "cloudfoundry/bosh-sample-release",
	"owner": {
		"login": "cloudfoundry",
		"id": 621746,
		"avatar_url": "https://avatars.githubusercontent.com/u/621746?v=3",
		"gravatar_id": "",
		"url": "https://api.github.com/users/cloudfoundry",
		"html_url": "https://github.com/cloudfoundry",
		"followers_url": "https://api.github.com/users/cloudfoundry/followers",
		"following_url": "https://api.github.com/users/cloudfoundry/following{/other_user}",
		"gists_url": "https://api.github.com/users/cloudfoundry/gists{/gist_id}",
		"starred_url": "https://api.github.com/users/cloudfoundry/starred{/owner}{/repo}",
		"subscriptions_url": "https://api.github.com/users/cloudfoundry/subscriptions",
		"organizations_url": "https://api.github.com/users/cloudfoundry/orgs",
		"repos_url": "https://api.github.com/users/cloudfoundry/repos",
		"events_url": "https://api.github.com/users/cloudfoundry/events{/privacy}",
		"received_events_url": "https://api.github.com/users/cloudfoundry/received_events",
		"type": "Organization",
		"site_admin": false
	},
	"private": false,
	"html_url": "https://github.com/cloudfoundry/bosh-sample-release",
	"description": "",
	"fork": false,
	"url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release",
	"forks_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/forks",
	"keys_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/keys{/key_id}",
	"collaborators_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/collaborators{/collaborator}",
	"teams_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/teams",
	"hooks_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/hooks",
	"issue_events_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/issues/events{/number}",
	"events_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/events",
	"assignees_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/assignees{/user}",
	"branches_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/branches{/branch}",
	"tags_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/tags",
	"blobs_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/git/blobs{/sha}",
	"git_tags_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/git/tags{/sha}",
	"git_refs_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/git/refs{/sha}",
	"trees_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/git/trees{/sha}",
	"statuses_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/statuses/{sha}",
	"languages_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/languages",
	"stargazers_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/stargazers",
	"contributors_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/contributors",
	"subscribers_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/subscribers",
	"subscription_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/subscription",
	"commits_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/commits{/sha}",
	"git_commits_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/git/commits{/sha}",
	"comments_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/comments{/number}",
	"issue_comment_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/issues/comments{/number}",
	"contents_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/contents/{+path}",
	"compare_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/compare/{base}...{head}",
	"merges_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/merges",
	"archive_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/{archive_format}{/ref}",
	"downloads_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/downloads",
	"issues_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/issues{/number}",
	"pulls_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/pulls{/number}",
	"milestones_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/milestones{/number}",
	"notifications_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/notifications{?since,all,participating}",
	"labels_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/labels{/name}",
	"releases_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/releases{/id}",
	"deployments_url": "https://api.github.com/repos/cloudfoundry/bosh-sample-release/deployments",
	"created_at": "2012-04-06T21:28:03Z",
	"updated_at": "2016-07-07T19:17:10Z",
	"pushed_at": "2015-12-04T21:31:34Z",
	"git_url": "git://github.com/cloudfoundry/bosh-sample-release.git",
	"ssh_url": "git@github.com:cloudfoundry/bosh-sample-release.git",
	"clone_url": "https://github.com/cloudfoundry/bosh-sample-release.git",
	"svn_url": "https://github.com/cloudfoundry/bosh-sample-release",
	"homepage": "",
	"size": 88303,
	"stargazers_count": 34,
	"watchers_count": 34,
	"language": "Shell",
	"has_issues": true,
	"has_downloads": true,
	"has_wiki": false,
	"has_pages": false,
	"forks_count": 34,
	"mirror_url": null,
	"open_issues_count": 5,
	"forks": 34,
	"open_issues": 5,
	"watchers": 34,
	"default_branch": "master",
	"permissions": {
		"admin": false,
		"push": false,
		"pull": true
	}
}`

var cfMessageBusJSON = `{
	"id": 10250069,
	"name": "cf-message-bus",
	"full_name": "cloudfoundry/cf-message-bus",
	"owner": {
		"login": "cloudfoundry",
		"id": 621746,
		"avatar_url": "https://avatars.githubusercontent.com/u/621746?v=3",
		"gravatar_id": "",
		"url": "https://api.github.com/users/cloudfoundry",
		"html_url": "https://github.com/cloudfoundry",
		"followers_url": "https://api.github.com/users/cloudfoundry/followers",
		"following_url": "https://api.github.com/users/cloudfoundry/following{/other_user}",
		"gists_url": "https://api.github.com/users/cloudfoundry/gists{/gist_id}",
		"starred_url": "https://api.github.com/users/cloudfoundry/starred{/owner}{/repo}",
		"subscriptions_url": "https://api.github.com/users/cloudfoundry/subscriptions",
		"organizations_url": "https://api.github.com/users/cloudfoundry/orgs",
		"repos_url": "https://api.github.com/users/cloudfoundry/repos",
		"events_url": "https://api.github.com/users/cloudfoundry/events{/privacy}",
		"received_events_url": "https://api.github.com/users/cloudfoundry/received_events",
		"type": "Organization",
		"site_admin": false
	},
	"private": false,
	"html_url": "https://github.com/cloudfoundry/cf-message-bus",
	"description": "",
	"fork": false,
	"url": "https://api.github.com/repos/cloudfoundry/cf-message-bus",
	"forks_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/forks",
	"keys_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/keys{/key_id}",
	"collaborators_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/collaborators{/collaborator}",
	"teams_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/teams",
	"hooks_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/hooks",
	"issue_events_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/issues/events{/number}",
	"events_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/events",
	"assignees_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/assignees{/user}",
	"branches_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/branches{/branch}",
	"tags_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/tags",
	"blobs_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/git/blobs{/sha}",
	"git_tags_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/git/tags{/sha}",
	"git_refs_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/git/refs{/sha}",
	"trees_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/git/trees{/sha}",
	"statuses_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/statuses/{sha}",
	"languages_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/languages",
	"stargazers_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/stargazers",
	"contributors_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/contributors",
	"subscribers_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/subscribers",
	"subscription_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/subscription",
	"commits_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/commits{/sha}",
	"git_commits_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/git/commits{/sha}",
	"comments_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/comments{/number}",
	"issue_comment_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/issues/comments{/number}",
	"contents_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/contents/{+path}",
	"compare_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/compare/{base}...{head}",
	"merges_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/merges",
	"archive_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/{archive_format}{/ref}",
	"downloads_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/downloads",
	"issues_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/issues{/number}",
	"pulls_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/pulls{/number}",
	"milestones_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/milestones{/number}",
	"notifications_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/notifications{?since,all,participating}",
	"labels_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/labels{/name}",
	"releases_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/releases{/id}",
	"deployments_url": "https://api.github.com/repos/cloudfoundry/cf-message-bus/deployments",
	"created_at": "2013-05-23T18:06:03Z",
	"updated_at": "2016-07-13T19:07:34Z",
	"pushed_at": "2016-07-13T19:08:18Z",
	"git_url": "git://github.com/cloudfoundry/cf-message-bus.git",
	"ssh_url": "git@github.com:cloudfoundry/cf-message-bus.git",
	"clone_url": "https://github.com/cloudfoundry/cf-message-bus.git",
	"svn_url": "https://github.com/cloudfoundry/cf-message-bus",
	"homepage": null,
	"size": 121,
	"stargazers_count": 1,
	"watchers_count": 1,
	"language": "Ruby",
	"has_issues": true,
	"has_downloads": true,
	"has_wiki": true,
	"has_pages": false,
	"forks_count": 7,
	"mirror_url": null,
	"open_issues_count": 0,
	"forks": 7,
	"open_issues": 0,
	"watchers": 1,
	"default_branch": "master",
	"permissions": {
		"admin": false,
		"push": false,
		"pull": true
	}
}`
