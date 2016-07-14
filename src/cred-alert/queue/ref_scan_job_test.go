package queue_test

import (
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
	"cred-alert/notifications/notificationsfakes"
	"cred-alert/queue"
	"cred-alert/scanners"
	"cred-alert/sniff"
)

var _ = Describe("RefScan Job", func() {
	var (
		client *githubfakes.FakeClient

		logger *lagertest.TestLogger

		job               *queue.RefScanJob
		server            *ghttp.Server
		sniffFunc         sniff.SniffFunc
		plan              queue.RefScanPlan
		notifier          *notificationsfakes.FakeNotifier
		emitter           *metricsfakes.FakeEmitter
		credentialCounter *metricsfakes.FakeCounter
	)

	owner := "repo-owner"
	repo := "repo-name"
	repoFullName := owner + "/" + repo
	ref := "reference"

	BeforeEach(func() {
		server = ghttp.NewServer()
		plan = queue.RefScanPlan{
			Owner:      owner,
			Repository: repo,
			Ref:        ref,
		}

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
	})

	JustBeforeEach(func() {
		job = queue.NewRefScanJob(plan, client, sniffFunc, notifier, emitter)
	})

	Describe("Run", func() {
		wasSniffed := false
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
			sniffFunc = func(lgr lager.Logger, scanner sniff.Scanner, handleViolation func(scanners.Line)) {
				wasSniffed = true
				handleViolation(scanners.Line{
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
			_, owner, repo := client.ArchiveLinkArgsForCall(0)
			Expect(owner).To(Equal("repo-owner"))
			Expect(repo).To(Equal("repo-name"))
		})

		It("Scans the archive", func() {
			err := job.Run(logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(wasSniffed).To(BeTrue())
		})

		It("sends a notification when it finds a match", func() {
			err := job.Run(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(notifier.SendNotificationCallCount()).To(Equal(len(files)))
			_, repository, sha, line := notifier.SendNotificationArgsForCall(0)

			Expect(repository).To(Equal(repoFullName))
			Expect(sha).To(Equal(ref))
			Expect(line).To(Equal(scanners.Line{
				Path:       filePath,
				LineNumber: lineNumber,
				Content:    fileContent,
			}))
		})

		It("emits violations", func() {
			job.Run(logger)
			Expect(credentialCounter.IncCallCount()).To(Equal(len(files)))
		})

		It("logs when credential is found", func() {
			job.Run(logger)
			Expect(logger).To(gbytes.Say("found-credential"))
			Expect(logger).To(gbytes.Say("found-credential"))
			Expect(logger).To(gbytes.Say("found-credential"))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"line-number":%d`, lineNumber)))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"owner":"%s"`, owner)))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"path":"%s"`, filePath)))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"ref":"%s"`, ref)))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"repository":"%s"`, repo)))
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
