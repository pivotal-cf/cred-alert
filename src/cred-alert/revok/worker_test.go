package revok_test

import (
	"cred-alert/db"
	"cred-alert/db/dbfakes"
	"cred-alert/gitclient/gitclientfakes"
	"cred-alert/metrics"
	"cred-alert/metrics/metricsfakes"
	"cred-alert/revok"
	"cred-alert/revok/revokfakes"
	"cred-alert/sniff"
	"cred-alert/sniff/snifffakes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	git "github.com/libgit2/git2go"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Worker", func() {
	var (
		logger               *lagertest.TestLogger
		clock                *fakeclock.FakeClock
		ghClient             *revokfakes.FakeGitHubClient
		gitClient            *gitclientfakes.FakeClient
		workdir              string
		sniffer              *snifffakes.FakeSniffer
		interval             time.Duration
		scanRepository       *dbfakes.FakeScanRepository
		repositoryRepository *dbfakes.FakeRepositoryRepository
		fetchRepository      *dbfakes.FakeFetchRepository
		emitter              *metricsfakes.FakeEmitter

		firstScan     *dbfakes.FakeActiveScan
		secondScan    *dbfakes.FakeActiveScan
		successMetric *metricsfakes.FakeCounter
		failedMetric  *metricsfakes.FakeCounter

		currentRepositoryID uint
		currentFetchID      uint

		runner  ifrit.Runner
		process ifrit.Process
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("revok")
		clock = fakeclock.NewFakeClock(time.Now())

		ghClient = &revokfakes.FakeGitHubClient{}
		ghClient.ListRepositoriesStub = func(lager.Logger) ([]revok.GitHubRepository, error) {
			if ghClient.ListRepositoriesCallCount() == 1 {
				return []revok.GitHubRepository{boshSampleReleaseRepository}, nil
			}

			return []revok.GitHubRepository{
				boshSampleReleaseRepository,
				cfMessageBusRepository,
			}, nil
		}

		gitClient = &gitclientfakes.FakeClient{}
		gitClient.CloneStub = func(url, dest string) error {
			err := os.MkdirAll(dest, os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
			switch url {
			case "git@github.com:cloudfoundry/bosh-sample-release.git":
				ioutil.WriteFile(filepath.Join(dest, "file-a1"), []byte("bus-pass"), os.ModePerm)
				ioutil.WriteFile(filepath.Join(dest, "file-b1"), []byte("credential"), os.ModePerm)
			case "git@github.com:cloudfoundry/cf-message-bus.git":
				ioutil.WriteFile(filepath.Join(dest, "file-a2"), []byte("bus-pass"), os.ModePerm)
				ioutil.WriteFile(filepath.Join(dest, "file-b2"), []byte("credential"), os.ModePerm)
				ioutil.WriteFile(filepath.Join(dest, "file-c2"), []byte("credential"), os.ModePerm)
			}

			return nil
		}

		interval = 1 * time.Hour

		scanRepository = &dbfakes.FakeScanRepository{}
		firstScan = &dbfakes.FakeActiveScan{}
		secondScan = &dbfakes.FakeActiveScan{}
		scanRepository.StartStub = func(lager.Logger, string, *db.Repository, *db.Fetch) db.ActiveScan {
			if scanRepository.StartCallCount() == 1 {
				return firstScan
			}
			return secondScan
		}

		repositoryRepository = &dbfakes.FakeRepositoryRepository{}
		currentRepositoryID = 0
		repositoryRepository.FindOrCreateStub = func(r *db.Repository) error {
			currentRepositoryID++
			r.ID = currentRepositoryID
			return nil
		}

		currentFetchID = 0
		fetchRepository = &dbfakes.FakeFetchRepository{}
		fetchRepository.SaveFetchStub = func(l lager.Logger, f *db.Fetch) error {
			currentFetchID++
			f.ID = currentFetchID
			return nil
		}

		emitter = &metricsfakes.FakeEmitter{}
		successMetric = &metricsfakes.FakeCounter{}
		failedMetric = &metricsfakes.FakeCounter{}
		emitter.CounterStub = func(name string) metrics.Counter {
			switch name {
			case "revok.success_jobs":
				return successMetric
			case "revok.failed_jobs":
				return failedMetric
			}
			return &metricsfakes.FakeCounter{}
		}

		var err error
		workdir, err = ioutil.TempDir("", "revok-test")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
		os.RemoveAll(workdir)
	})

	JustBeforeEach(func() {
		sniffer = &snifffakes.FakeSniffer{}
		sniffer.SniffStub = func(l lager.Logger, s sniff.Scanner, h sniff.ViolationHandlerFunc) error {
			for s.Scan(logger) {
				line := s.Line(logger)
				if strings.Contains(string(line.Content), "credential") {
					h(l, *line)
				}
			}

			return nil
		}

		runner = revok.New(
			logger,
			clock,
			workdir,
			ghClient,
			gitClient,
			sniffer,
			interval,
			scanRepository,
			repositoryRepository,
			fetchRepository,
			emitter,
		)
		process = ginkgomon.Invoke(runner)
	})

	It("does work immediately on start", func() {
		Eventually(ghClient.ListRepositoriesCallCount).Should(Equal(1))
	})

	It("does work once per hour", func() {
		Eventually(ghClient.ListRepositoriesCallCount).Should(Equal(1))
		clock.Increment(interval)
		Eventually(ghClient.ListRepositoriesCallCount).Should(Equal(2))
	})

	It("tries to clone each repo", func() {
		Eventually(gitClient.CloneCallCount).Should(Equal(1))
		url, dest := gitClient.CloneArgsForCall(0)
		Expect(url).To(Equal("git@github.com:cloudfoundry/bosh-sample-release.git"))
		Expect(dest).To(Equal(filepath.Join(workdir, "cloudfoundry", "bosh-sample-release")))
	})

	It("tries to store the cloned repo in the database", func() {
		Eventually(firstScan.FinishCallCount).Should(Equal(1))
		Expect(repositoryRepository.FindOrCreateCallCount()).To(Equal(1))
		repo := repositoryRepository.FindOrCreateArgsForCall(0)
		Expect(repo.Owner).To(Equal("cloudfoundry"))
		Expect(repo.Name).To(Equal("bosh-sample-release"))
		Expect(repo.Path).To(Equal(filepath.Join(workdir, "cloudfoundry", "bosh-sample-release")))
		Expect(repo.SSHURL).To(Equal("git@github.com:cloudfoundry/bosh-sample-release.git"))
		Expect(repo.Private).To(BeFalse())
		Expect(repo.DefaultBranch).To(Equal("master"))
		Expect(repo.RawJSON).To(MatchJSON([]byte(boshSampleReleaseRepositoryJSON)))
	})

	It("scans each file in the default branch of each cloned repo", func() {
		Eventually(sniffer.SniffCallCount).Should(Equal(2))
		clock.Increment(interval)
		Eventually(sniffer.SniffCallCount).Should(Equal(5))
	})

	It("tries to store information in the database about found credentials", func() {
		Eventually(scanRepository.StartCallCount).Should(Equal(1))
		_, scanType, repository, fetch := scanRepository.StartArgsForCall(0)
		Expect(scanType).To(Equal("dir-scan"))
		Expect(repository.ID).To(Equal(currentRepositoryID))
		Expect(fetch).To(BeNil())

		Eventually(firstScan.RecordCredentialCallCount).Should(Equal(1))
		Eventually(firstScan.FinishCallCount).Should(Equal(1))

		clock.Increment(interval)

		Eventually(scanRepository.StartCallCount).Should(Equal(2))
		_, scanType, repository, fetch = scanRepository.StartArgsForCall(1)
		Expect(scanType).To(Equal("dir-scan"))
		Expect(repository.ID).To(Equal(currentRepositoryID))
		Expect(fetch).To(BeNil())

		Eventually(secondScan.RecordCredentialCallCount).Should(Equal(2))
		Eventually(secondScan.FinishCallCount).Should(Equal(1))
	})

	It("increments the success metric", func() {
		Eventually(firstScan.FinishCallCount).Should(Equal(1))
		Expect(successMetric.IncCallCount()).To(Equal(1))

		clock.Increment(interval)

		Eventually(secondScan.FinishCallCount).Should(Equal(1))
		Expect(successMetric.IncCallCount()).To(Equal(2))
	})

	Context("when there is an error storing the repository in the database", func() {
		BeforeEach(func() {
			repositoryRepository.FindOrCreateReturns(errors.New("an-error"))
		})

		It("does nothing and continues", func() {
			Consistently(gitClient.CloneCallCount).Should(BeZero())
		})
	})

	Context("when there is an error storing credentials in the database", func() {
		BeforeEach(func() {
			firstScan.FinishReturns(errors.New("an-error"))
		})

		It("increments the failed metric", func() {
			Eventually(firstScan.FinishCallCount).Should(Equal(1))
			Expect(failedMetric.IncCallCount()).To(Equal(1))
		})
	})

	Context("when a repo has already been cloned", func() {
		BeforeEach(func() {
			os.MkdirAll(filepath.Join(workdir, "cloudfoundry", "bosh-sample-release"), os.ModeDir|os.ModePerm)
		})

		It("tries to clone only repos it hasn't seen before", func() {
			Eventually(ghClient.ListRepositoriesCallCount).Should(Equal(1))

			Expect(gitClient.CloneCallCount()).To(BeZero()) // repo from first request already exists

			clock.Increment(interval)

			Eventually(ghClient.ListRepositoriesCallCount).Should(Equal(2))

			Eventually(gitClient.CloneCallCount).Should(Equal(1))

			url, dest := gitClient.CloneArgsForCall(0)
			Expect(url).To(Equal("git@github.com:cloudfoundry/cf-message-bus.git"))
			Expect(dest).To(Equal(filepath.Join(workdir, "cloudfoundry", "cf-message-bus")))
		})

		It("fetches updates for the repo", func() {
			Eventually(gitClient.FetchCallCount).Should(Equal(1))
			Expect(gitClient.FetchArgsForCall(0)).To(Equal(filepath.Join(workdir, "cloudfoundry", "bosh-sample-release")))
		})

		Context("when the remote has changes", func() {
			var (
				oldOid1 *git.Oid
				newOid1 *git.Oid
				oldOid2 *git.Oid
				newOid2 *git.Oid
				changes map[string][]*git.Oid
			)

			BeforeEach(func() {
				var err error
				oldOid1, err = git.NewOid("fce98866a7d559757a0a501aa548e244a46ad00a")
				Expect(err).NotTo(HaveOccurred())
				newOid1, err = git.NewOid("3f5c0cc6c73ddb1a3aa05725c48ca1223367fb74")
				Expect(err).NotTo(HaveOccurred())
				oldOid2, err = git.NewOid("7257894438275f68380aa6d75015ef7a0ca6757b")
				Expect(err).NotTo(HaveOccurred())
				newOid2, err = git.NewOid("53fc72ccf2ef176a02169aeebf5c8427861e9b0e")
				Expect(err).NotTo(HaveOccurred())

				changes = map[string][]*git.Oid{
					"refs/remotes/origin/master":  {oldOid1, newOid1},
					"refs/remotes/origin/develop": {oldOid2, newOid2},
				}

				gitClient.FetchReturns(changes, nil)

				gitClient.DiffStub = func(repositoryPath string, a, b *git.Oid) (string, error) {
					if gitClient.DiffCallCount() == 1 {
						return `diff --git a/stuff.txt b/stuff.txt
index f2e4113..fa5a232 100644
--- a/stuff.txt
+++ b/stuff.txt
@@ -1 +1,2 @@
-old
+credential
+something-else`, nil
					}

					return `--git a/stuff.txt b/stuff.txt
index fa5a232..1e13fe8 100644
--- a/stuff.txt
+++ b/stuff.txt
@@ -1,2 +1 @@
-old
-content
+credential`, nil
				}
			})

			It("does a diff scan on the changes", func() {
				Eventually(gitClient.DiffCallCount).Should(Equal(2))

				// for synchronizing the unordered map returned by Fetch
				expectedOids := map[string][]*git.Oid{
					oldOid1.String(): []*git.Oid{oldOid1, newOid1},
					oldOid2.String(): []*git.Oid{oldOid2, newOid2},
				}

				actualOids := map[string][]*git.Oid{}

				dest, a, b := gitClient.DiffArgsForCall(0)
				Expect(dest).To(Equal(filepath.Join(workdir, "cloudfoundry", "bosh-sample-release")))
				actualOids[a.String()] = []*git.Oid{a, b}

				// this is the only way to detect anything was scanned
				Expect(firstScan.RecordCredentialCallCount()).To(Equal(1))

				dest, c, d := gitClient.DiffArgsForCall(1)
				Expect(dest).To(Equal(filepath.Join(workdir, "cloudfoundry", "bosh-sample-release")))
				actualOids[c.String()] = []*git.Oid{c, d}

				Expect(actualOids).To(Equal(expectedOids))

				Expect(secondScan.RecordCredentialCallCount()).To(Equal(1))
			})

			It("tries to store information in the database about the fetch", func() {
				Eventually(firstScan.FinishCallCount).Should(Equal(1))
				Expect(fetchRepository.SaveFetchCallCount()).To(Equal(1))
				_, fetch := fetchRepository.SaveFetchArgsForCall(0)
				Expect(fetch.Path).To(Equal(filepath.Join(workdir, "cloudfoundry", "bosh-sample-release")))
				bs, err := json.Marshal(changes)
				Expect(err).NotTo(HaveOccurred())
				Expect(fetch.Changes).To(Equal(bs))
				Expect(fetch.Repository.ID).To(BeNumerically(">", 0))
			})

			It("tries to store information in the database about found credentials", func() {
				Eventually(scanRepository.StartCallCount).Should(Equal(2))
				_, scanType, repository, fetch := scanRepository.StartArgsForCall(0)
				Expect(scanType).To(Equal("diff-scan"))
				Expect(repository.ID).To(Equal(currentRepositoryID))
				Expect(fetch.ID).To(Equal(currentFetchID))

				Eventually(firstScan.RecordCredentialCallCount).Should(Equal(1))
				Eventually(firstScan.FinishCallCount).Should(Equal(1))

				Eventually(secondScan.RecordCredentialCallCount).Should(Equal(1))
				Eventually(secondScan.FinishCallCount).Should(Equal(1))
			})

			It("increments the success metric", func() {
				Eventually(successMetric.IncCallCount).Should(Equal(2))
			})

			Context("when there is an error saving the fetch to the database", func() {
				BeforeEach(func() {
					fetchRepository.SaveFetchReturns(errors.New("an-error"))
				})

				It("does not try to diff anything", func() {
					Consistently(gitClient.DiffCallCount).Should(BeZero())
				})
			})

			XIt("it does a msg scan on the changes", func() {
			})

			XContext("when there is an error getting the diff for the changes", func() {
				BeforeEach(func() {
					gitClient.DiffStub = func(dest string, a *git.Oid, b *git.Oid) (string, error) {
						if gitClient.DiffCallCount() == 1 {
							return "", errors.New("an-error")
						}

						return `--git a/stuff.txt b/stuff.txt
index fa5a232..1e13fe8 100644
--- a/stuff.txt
+++ b/stuff.txt
@@ -1,2 +1 @@
-old
-content
+credential`, nil
					}
				})

				XIt("increments a metric which doesn't exist yet", func() {
				})
			})

			Context("when there is an error storing credentials in the database", func() {
				BeforeEach(func() {
					firstScan.FinishReturns(errors.New("an-error"))
				})

				It("increments the failed metric", func() {
					Eventually(firstScan.FinishCallCount).Should(Equal(1))
					Expect(failedMetric.IncCallCount()).To(Equal(1))
				})
			})
		})

		XContext("when the remote includes a fresh push", func() {
			var (
				oldOid, newOid *git.Oid
			)

			BeforeEach(func() {
				var err error
				oldOid, err = git.NewOid("0000000000000000000000000000000000000000")
				Expect(err).NotTo(HaveOccurred())
				newOid, err = git.NewOid("3f5c0cc6c73ddb1a3aa05725c48ca1223367fb74")
				Expect(err).NotTo(HaveOccurred())

				gitClient.FetchReturns(map[string][]*git.Oid{
					"refs/remotes/origin/master": {oldOid, newOid},
				}, nil)

				gitClient.DiffStub = func(repositoryPath string, a, b *git.Oid) (string, error) {
					if gitClient.DiffCallCount() == 1 {
						return `diff --git a/stuff.txt b/stuff.txt
index f2e4113..fa5a232 100644
--- a/stuff.txt
+++ b/stuff.txt
@@ -1 +1,2 @@
-old
+credential
+something-else`, nil
					}

					return "", nil
				}
			})

			It("resets the repo to the new head and does a dir scan, probably", func() {
			})
		})

		Context("when fetching the remote results in an error", func() {
			var (
				oldOid, newOid *git.Oid
			)

			BeforeEach(func() {
				var err error
				oldOid, err = git.NewOid("0000000000000000000000000000000000000000")
				Expect(err).NotTo(HaveOccurred())
				newOid, err = git.NewOid("3f5c0cc6c73ddb1a3aa05725c48ca1223367fb74")
				Expect(err).NotTo(HaveOccurred())

				gitClient.FetchStub = func(dest string) (map[string][]*git.Oid, error) {
					if gitClient.FetchCallCount() == 1 {
						return nil, errors.New("an-error")
					}

					return map[string][]*git.Oid{
						"refs/remotes/origin/master": {oldOid, newOid},
					}, nil
				}
			})

			It("does not save the fetch", func() {
				Consistently(fetchRepository.SaveFetchCallCount).Should(BeZero())
			})

			It("does not try to diff anything", func() {
				Consistently(gitClient.DiffCallCount).Should(BeZero())
			})
		})
	})

	Context("when cloning fails", func() {
		BeforeEach(func() {
			gitClient.CloneStub = func(url, dest string) error {
				err := os.MkdirAll(dest, os.ModePerm)
				Expect(err).NotTo(HaveOccurred())
				return errors.New("an-error")
			}
		})

		It("cleans up the failed clone destination, if any", func() {
			Eventually(gitClient.CloneCallCount).Should(Equal(1))
			clock.Increment(interval)
			Eventually(gitClient.CloneCallCount).Should(BeNumerically(">", 1)) // to be sure first clone has completed

			for i := 0; i < gitClient.CloneCallCount(); i++ {
				_, dest := gitClient.CloneArgsForCall(0)
				_, err := os.Lstat(dest)
				Expect(os.IsNotExist(err)).To(BeTrue())
			}
		})

		It("doesn't try to scan the failed clone", func() {
			Eventually(gitClient.CloneCallCount).Should(Equal(1))
			clock.Increment(interval)
			Eventually(gitClient.CloneCallCount).Should(BeNumerically(">", 1)) // to be sure first clone has completed

			// this isn't the greatest assertion because the sniffer will error out
			// before trying to sniff if the directory is missing, which it will be
			// because we'll have cleaned it up. "erroring out here" is just printing
			// the error to stdout.
			Consistently(sniffer.SniffCallCount).Should(BeZero())
		})
	})
})

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
