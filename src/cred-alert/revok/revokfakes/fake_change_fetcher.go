// This file was generated by counterfeiter
package revokfakes

import (
	"cred-alert/revok"
	"sync"

	"code.cloudfoundry.org/lager"
)

type FakeChangeFetcher struct {
	FetchStub        func(logger lager.Logger, owner string, name string) error
	fetchMutex       sync.RWMutex
	fetchArgsForCall []struct {
		logger lager.Logger
		owner  string
		name   string
	}
	fetchReturns struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeChangeFetcher) Fetch(logger lager.Logger, owner string, name string) error {
	fake.fetchMutex.Lock()
	fake.fetchArgsForCall = append(fake.fetchArgsForCall, struct {
		logger lager.Logger
		owner  string
		name   string
	}{logger, owner, name})
	fake.recordInvocation("Fetch", []interface{}{logger, owner, name})
	fake.fetchMutex.Unlock()
	if fake.FetchStub != nil {
		return fake.FetchStub(logger, owner, name)
	}
	return fake.fetchReturns.result1
}

func (fake *FakeChangeFetcher) FetchCallCount() int {
	fake.fetchMutex.RLock()
	defer fake.fetchMutex.RUnlock()
	return len(fake.fetchArgsForCall)
}

func (fake *FakeChangeFetcher) FetchArgsForCall(i int) (lager.Logger, string, string) {
	fake.fetchMutex.RLock()
	defer fake.fetchMutex.RUnlock()
	return fake.fetchArgsForCall[i].logger, fake.fetchArgsForCall[i].owner, fake.fetchArgsForCall[i].name
}

func (fake *FakeChangeFetcher) FetchReturns(result1 error) {
	fake.FetchStub = nil
	fake.fetchReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeChangeFetcher) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.fetchMutex.RLock()
	defer fake.fetchMutex.RUnlock()
	return fake.invocations
}

func (fake *FakeChangeFetcher) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ revok.ChangeFetcher = new(FakeChangeFetcher)