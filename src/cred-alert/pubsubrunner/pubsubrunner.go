package pubsubrunner

import (
	"context"
	"cred-alert/queue"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"syscall"
	"time"

	"cloud.google.com/go/pubsub"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

type Runner struct {
	process ifrit.Process
	pid     int
	tmpDir  string
	client  *pubsub.Client
}

func (runner *Runner) Setup() {
	port := 8900 + GinkgoParallelNode()
	hostPort := fmt.Sprintf("127.0.0.1:%d", port)

	var err error
	runner.tmpDir, err = ioutil.TempDir("", "pubsubrunner")
	Expect(err).ToNot(HaveOccurred())

	cmd := exec.Command(
		"gcloud",
		"beta",
		"emulators",
		"pubsub",
		"start",
		"--host-port", hostPort,
		"--data-dir", runner.tmpDir,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	pubSub := ginkgomon.New(ginkgomon.Config{
		Name:              "pubsub-emulator",
		Command:           cmd,
		AnsiColorCode:     "35",
		StartCheck:        "INFO: Server started, listening on",
		StartCheckTimeout: 10 * time.Second,
	})

	runner.process = ginkgomon.Invoke(pubSub)
	runner.pid = cmd.Process.Pid

	err = os.Setenv("PUBSUB_EMULATOR_HOST", hostPort)
	Expect(err).ToNot(HaveOccurred())

	ctx := context.Background()
	runner.client, err = pubsub.NewClient(ctx, "testing")
	Expect(err).ToNot(HaveOccurred())
}

func (runner *Runner) Teardown() {
	err := runner.client.Close()
	Expect(err).NotTo(HaveOccurred())

	pgid, err := syscall.Getpgid(runner.pid)
	Expect(err).NotTo(HaveOccurred())

	err = syscall.Kill(-pgid, 2) // SIGINT
	Expect(err).NotTo(HaveOccurred())

	Eventually(runner.process.Wait()).Should(Receive())

	os.RemoveAll(runner.tmpDir)
	os.Unsetenv("PUBSUB_EMULATOR_HOST")
}

func (runner *Runner) Reset() {
	runner.Teardown()
	runner.Setup()
}

func (runner *Runner) CreateSubscription(tid, sid string) {
	ctx := context.Background()

	topic, err := runner.client.CreateTopic(ctx, tid)
	Expect(err).NotTo(HaveOccurred())

	_, err = runner.client.CreateSubscription(ctx, sid, topic, 10*time.Second, nil)
	Expect(err).NotTo(HaveOccurred())
}

func (runner *Runner) Client() *pubsub.Client {
	return runner.client
}

func (runner *Runner) LastMessage(subID string) queue.Task {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	it, err := runner.client.Subscription(subID).Pull(ctx, pubsub.MaxPrefetch(1))
	Expect(err).NotTo(HaveOccurred())
	defer it.Stop()

	message, err := it.Next()
	Expect(err).NotTo(HaveOccurred())
	message.Done(true)

	return basicTask{
		id:      message.Attributes["id"],
		typee:   message.Attributes["type"],
		payload: string(message.Data),
	}
}

type basicTask struct {
	id      string
	typee   string
	payload string
}

func (t basicTask) ID() string {
	return t.id
}

func (t basicTask) Type() string {
	return t.typee
}

func (t basicTask) Payload() string {
	return t.payload
}
