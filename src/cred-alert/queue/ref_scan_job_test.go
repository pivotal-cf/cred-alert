package queue_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"archive/zip"
	"bytes"
	"net/http"
	"net/url"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"

	"cred-alert/githubclient"
	"cred-alert/githubclient/githubclientfakes"
	"cred-alert/inflator"
	"cred-alert/inflator/inflatorfakes"
	"cred-alert/metrics"
	"cred-alert/metrics/metricsfakes"
	"cred-alert/notifications/notificationsfakes"
	"cred-alert/queue"
	"cred-alert/sniff"
)

var _ = Describe("RefScan Job", func() {
	var (
		client *githubclientfakes.FakeClient

		logger *lagertest.TestLogger

		files []fileInfo

		job               *queue.RefScanJob
		server            *ghttp.Server
		sniffer           sniff.Sniffer
		plan              queue.RefScanPlan
		notifier          *notificationsfakes.FakeNotifier
		emitter           *metricsfakes.FakeEmitter
		credentialCounter *metricsfakes.FakeCounter
		expander          *inflatorfakes.FakeInflator
		scratchSpace      inflator.ScratchSpace

		tmpPath string
	)

	owner := "repo-owner"
	repo := "repo-name"
	ref := "reference"

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("ref-scan-test")

		server = ghttp.NewServer()
		plan = queue.RefScanPlan{
			Owner:      owner,
			Repository: repo,
			Ref:        ref,
			Private:    true,
		}

		sniffer = sniff.NewDefaultSniffer()
		client = &githubclientfakes.FakeClient{}
		notifier = &notificationsfakes.FakeNotifier{}
		credentialCounter = &metricsfakes.FakeCounter{}
		expander = &inflatorfakes.FakeInflator{}

		emitter = &metricsfakes.FakeEmitter{}
		emitter.CounterStub = func(name string) metrics.Counter {
			switch name {
			case "cred_alert.violations":
				return credentialCounter
			default:
				panic("unexpected counter name! " + name)
			}
		}

		expander.InflateStub = func(lgr lager.Logger, archivePath, destination string) error {
			e := inflator.New()
			return e.Inflate(lgr, archivePath, destination)
		}
	})

	JustBeforeEach(func() {
		tmpPath = filepath.Join(os.TempDir(), fmt.Sprintf("ref-scan-test-%d", GinkgoParallelNode()))
		scratchSpace = inflator.NewDeterministicScratch(tmpPath)

		job = queue.NewRefScanJob(plan, client, sniffer, notifier, emitter, expander, scratchSpace)
	})

	AfterEach(func() {
		server.Close()
		Expect(os.RemoveAll(tmpPath)).To(Succeed())
	})

	Describe("Run", func() {
		BeforeEach(func() {
			serverUrl, err := url.Parse(server.URL())
			Expect(err).NotTo(HaveOccurred())

			client.ArchiveLinkReturns(serverUrl, nil)

			files = []fileInfo{
				{"github-dir-abc123/readme.txt", "password: thisisapassword"},
				{"github-dir-abc123/go/gopher.txt", "Gopher names:\nGeorge\nGeoffrey\nGonzo"},
				{"github-dir-abc123/todo/todo.txt", "Get animal handling licence.\nWrite more examples."},
			}

			someZip := createZip(files)
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/"),
					ghttp.RespondWith(http.StatusOK, someZip.Bytes(), http.Header{}),
				),
			)
		})

		It("fetches a link from GitHub", func() {
			err := job.Run(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(client.ArchiveLinkCallCount()).To(Equal(1))
			returnedOwner, returnedRepo, returnedRef := client.ArchiveLinkArgsForCall(0)
			Expect(returnedOwner).To(Equal(owner))
			Expect(returnedRepo).To(Equal(repo))
			Expect(returnedRef).To(Equal(ref))
		})

		It("sends a notification when it finds a match", func() {
			err := job.Run(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(notifier.SendNotificationCallCount()).To(Equal(1))

			_, notification := notifier.SendNotificationArgsForCall(0)
			Expect(notification.Owner).To(Equal(plan.Owner))
			Expect(notification.Repository).To(Equal(plan.Repository))
			Expect(notification.SHA).To(Equal(ref))
			Expect(notification.Path).To(Equal("readme.txt"))
			Expect(notification.LineNumber).To(Equal(1))
			Expect(notification.Private).To(Equal(plan.Private))
		})

		Context("when the inflator fails", func() {
			BeforeEach(func() {
				expander = &inflatorfakes.FakeInflator{}
				expander.InflateReturns(errors.New("disaster"))
			})

			It("logs and returns an error", func() {
				err := job.Run(logger)
				Expect(err).To(HaveOccurred())
				Expect(logger).To(gbytes.Say("failed"))
			})
		})

		Context("when the notification fails to send", func() {
			BeforeEach(func() {
				notifier.SendNotificationReturns(errors.New("disaster"))
			})

			It("fails the job", func() {
				err := job.Run(logger)
				Expect(err).To(HaveOccurred())
			})
		})

		It("emits violations", func() {
			err := job.Run(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(credentialCounter.IncCallCount()).To(Equal(1))
			_, tags := credentialCounter.IncArgsForCall(0)
			Expect(tags).To(HaveLen(1))
			Expect(tags).To(ConsistOf("private"))
		})

		It("logs when credential is found", func() {
			err := job.Run(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(logger).To(gbytes.Say("handle-violation"))
		})

		Context("when the repo is public", func() {
			BeforeEach(func() {
				plan.Private = false
			})

			It("emits count with the public tag", func() {
				job.Run(logger)

				Expect(credentialCounter.IncCallCount()).To(Equal(1))
				_, tags := credentialCounter.IncArgsForCall(0)
				Expect(tags).To(HaveLen(1))
				Expect(tags).To(ConsistOf("public"))
			})

			It("sends a notification with private set to false", func() {
				err := job.Run(logger)
				Expect(err).NotTo(HaveOccurred())

				Expect(notifier.SendNotificationCallCount()).To(Equal(1))

				_, notification := notifier.SendNotificationArgsForCall(0)
				Expect(notification.Private).To(Equal(plan.Private))
			})
		})

		Context("when the ref is the nil ref (initial empty repo)", func() {
			BeforeEach(func() {
				plan = queue.RefScanPlan{
					Owner:      owner,
					Repository: repo,
					Ref:        "0000000000000000000000000000000000000000",
				}
			})

			It("should not perform a scan", func() {
				err := job.Run(logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(client.ArchiveLinkCallCount()).To(Equal(0))
			})

			It("should log that scanning was skipped", func() {
				job.Run(logger)
				Expect(logger).To(gbytes.Say("skipped-initial-nil-ref"))
			})
		})

		Context("when the github API returns not found", func() {
			BeforeEach(func() {
				client.ArchiveLinkReturns(nil, githubclient.ErrNotFound)
			})

			It("logs an error", func() {
				job.Run(logger)
				Expect(logger.Buffer()).To(gbytes.Say("archive-link.failed"))
			})

			It("does not return an error", func() {
				err := job.Run(logger)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the archive URL is nil", func() {
			BeforeEach(func() {
				client.ArchiveLinkReturns(nil, nil)
			})

			It("Returns an error", func() {
				err := job.Run(logger)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

type fileInfo struct {
	Name string
	Body string
}

func createZip(files []fileInfo) *bytes.Buffer {
	buf := new(bytes.Buffer)

	w := zip.NewWriter(buf)

	for _, file := range files {
		f, err := w.Create(file.Name)
		Expect(err).NotTo(HaveOccurred())

		_, err = f.Write([]byte(file.Body))
		Expect(err).NotTo(HaveOccurred())
	}

	err := w.Close()
	Expect(err).NotTo(HaveOccurred())

	return buf
}
