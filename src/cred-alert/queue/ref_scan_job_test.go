package queue_test

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"archive/zip"
	"bytes"
	"log"
	"net/http"
	"net/url"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"

	"cred-alert/github/githubfakes"
	"cred-alert/metrics"
	"cred-alert/metrics/metricsfakes"
	"cred-alert/mimetype/mimetypefakes"
	"cred-alert/notifications/notificationsfakes"
	"cred-alert/queue"
	"cred-alert/scanners"
	"cred-alert/sniff"
	"cred-alert/sniff/snifffakes"
)

var _ = Describe("RefScan Job", func() {
	var (
		client *githubfakes.FakeClient

		logger *lagertest.TestLogger

		job               *queue.RefScanJob
		server            *ghttp.Server
		sniffer           *snifffakes.FakeSniffer
		plan              queue.RefScanPlan
		notifier          *notificationsfakes.FakeNotifier
		emitter           *metricsfakes.FakeEmitter
		credentialCounter *metricsfakes.FakeCounter
		mimetype          *mimetypefakes.FakeMimetype
	)

	owner := "repo-owner"
	repo := "repo-name"
	repoFullName := owner + "/" + repo
	ref := "reference"
	id := "my-id"

	BeforeEach(func() {
		server = ghttp.NewServer()
		plan = queue.RefScanPlan{
			Owner:      owner,
			Repository: repo,
			Ref:        ref,
			Private:    true,
		}

		sniffer = new(snifffakes.FakeSniffer)
		client = &githubfakes.FakeClient{}
		logger = lagertest.NewTestLogger("ref-scan")
		notifier = &notificationsfakes.FakeNotifier{}
		emitter = &metricsfakes.FakeEmitter{}
		credentialCounter = &metricsfakes.FakeCounter{}
		emitter.CounterStub = func(name string) metrics.Counter {
			switch name {
			case "cred_alert.violations":
				return credentialCounter
			default:
				panic("unexpected counter name! " + name)
			}
		}
		mimetype = &mimetypefakes.FakeMimetype{}
		mimetype.TypeByBufferReturns("text/some-text", nil)
	})

	JustBeforeEach(func() {
		job = queue.NewRefScanJob(plan, client, sniffer, notifier, emitter, mimetype, id)
	})

	Describe("Run", func() {
		filePath := "some/file/path"
		fileContent := "content"
		lineNumber := 2

		BeforeEach(func() {
			serverUrl, _ := url.Parse(server.URL())
			client.ArchiveLinkReturns(serverUrl, nil)
			someZip := createZip()
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/"),
					ghttp.RespondWith(http.StatusOK, someZip.Bytes(), http.Header{}),
				),
			)
			sniffer.SniffStub = func(lgr lager.Logger, scanner sniff.Scanner, handleViolation func(scanners.Line) error) error {
				return handleViolation(scanners.Line{
					Path:       filePath,
					LineNumber: lineNumber,
					Content:    fileContent,
				})
			}
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

		It("Scans the archive", func() {
			err := job.Run(logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(sniffer.SniffCallCount()).To(BeNumerically(">", 0))
		})

		It("sends a notification when it finds a match", func() {
			err := job.Run(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(notifier.SendNotificationCallCount()).To(Equal(len(files)))
			_, repository, sha, line, private := notifier.SendNotificationArgsForCall(0)

			Expect(repository).To(Equal(repoFullName))
			Expect(sha).To(Equal(ref))
			Expect(line).To(Equal(scanners.Line{
				Path:       filePath,
				LineNumber: lineNumber,
				Content:    fileContent,
			}))
			Expect(private).To(Equal(plan.Private))
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

			Expect(credentialCounter.IncCallCount()).To(Equal(len(files)))
			_, tags := credentialCounter.IncArgsForCall(0)
			Expect(tags).To(HaveLen(1))
			Expect(tags).To(ConsistOf("private"))
		})

		It("logs when credential is found", func() {
			err := job.Run(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(logger).To(gbytes.Say("found-credential"))
			Expect(logger).To(gbytes.Say("found-credential"))
			Expect(logger).To(gbytes.Say("found-credential"))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"line-number":%d`, lineNumber)))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"owner":"%s"`, owner)))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"path":"%s"`, filePath)))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"private":%v`, plan.Private)))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"ref":"%s"`, ref)))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"repository":"%s"`, repo)))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"task-id":"%s"`, id)))
		})

		Context("when the repo is public", func() {
			BeforeEach(func() {
				plan.Private = false
			})

			It("emits count with the public tag", func() {
				job.Run(logger)

				Expect(credentialCounter.IncCallCount()).To(Equal(len(files)))
				_, tags := credentialCounter.IncArgsForCall(0)
				Expect(tags).To(HaveLen(1))
				Expect(tags).To(ConsistOf("public"))
			})

			It("sends a notification with private set to false", func() {
				job.Run(logger)

				Expect(notifier.SendNotificationCallCount()).To(Equal(len(files)))
				_, _, _, _, private := notifier.SendNotificationArgsForCall(0)
				Expect(private).To(Equal(false))
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

		Context("when file is not text", func() {
			BeforeEach(func() {
				mimetype.TypeByBufferReturns("application/octet-stream", nil)
			})

			It("should not perform a scan", func() {
				job.Run(logger)
				Expect(sniffer.SniffCallCount()).To(Equal(0))
			})

			Context("when there is an error getting the type", func() {
				BeforeEach(func() {
					mimetype.TypeByBufferReturns("unknown", errors.New("disaster"))
				})

				It("logs an error", func() {
					job.Run(logger)
					Expect(logger).To(gbytes.Say("mimetype-error"))
				})
			})
		})
	})
})

var files = []struct {
	Name, Body string
}{
	{"readme.txt", `lolz`},
	{"gopher.txt", "Gopher names:\nGeorge\nGeoffrey\nGonzo"},
	{"todo.txt", "Get animal handling licence.\nWrite more examples."},
}

func createZip() *bytes.Buffer {
	// Create a buffer to write our archive to.
	buf := new(bytes.Buffer)

	// Create a new zip archive.
	w := zip.NewWriter(buf)

	// Add some files to the archive.
	for _, file := range files {
		f, err := w.Create(file.Name)
		if err != nil {
			log.Fatal(err)
		}
		_, err = f.Write([]byte(file.Body))
		if err != nil {
			log.Fatal(err)
		}
	}

	// Make sure to check the error on Close.
	w.Close()

	return buf
}
