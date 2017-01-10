package revok

import (
	"os"
	"sync"

	"github.com/robfig/cron"
)

type ScheduleRunner struct {
	cron    *cron.Cron
	cronMut *sync.Mutex

	jobWg *sync.WaitGroup
}

func NewScheduleRunner() *ScheduleRunner {
	return &ScheduleRunner{
		cron:    cron.New(),
		cronMut: &sync.Mutex{},
		jobWg:   &sync.WaitGroup{},
	}
}

func (s *ScheduleRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	s.cron.Start()

	close(ready)

	select {
	case <-signals:
		s.cron.Stop()
		s.jobWg.Wait()
	}

	return nil
}

func (s *ScheduleRunner) ScheduleWork(cron string, work func()) {
	s.cronMut.Lock()
	defer s.cronMut.Unlock()

	s.cron.AddFunc(cron, func() {
		s.jobWg.Add(1)
		defer s.jobWg.Done()

		work()
	})
}
