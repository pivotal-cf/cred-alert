package queue_test

import (
	"cred-alert/queue"

	"github.com/google/go-github/github"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type dummyTask struct {
	data map[string]interface{}
}

func newDummyTask() *dummyTask {
	dummyTask := &dummyTask{}
	dummyTask.data = make(map[string]interface{})
	return dummyTask
}

func (t *dummyTask) Data() map[string]interface{} {
	return t.data
}

func (t *dummyTask) Receipt() string {
	return ""
}

var _ = Describe("PushEventTask", func() {
	var (
		task          queue.Task
		expectedEvent github.PushEvent
	)

	BeforeEach(func() {
		id := 0
		expectedEvent.PushID = &id

		task = queue.NewPushEventTask(expectedEvent)
	})

	It("returns the event with which it was created", func() {
		Expect(task.Data()["event"]).To(Equal(expectedEvent))
	})

	Context("GetEvent", func() {
		It("gets event from a task", func() {
			event, err := queue.GetEvent(task)
			Expect(err).ToNot(HaveOccurred())
			Expect(event).To(Equal(expectedEvent))
		})

		It("return error if there's no event", func() {
			dummyTask := newDummyTask()
			_, err := queue.GetEvent(dummyTask)
			Expect(err).To(HaveOccurred())
		})

		It("return error if the event is not a push event", func() {
			dummyTask := newDummyTask()
			dummyTask.Data()["event"] = 0
			_, err := queue.GetEvent(dummyTask)
			Expect(err).To(HaveOccurred())
		})
	})
})
