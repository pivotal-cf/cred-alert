// This file was generated by counterfeiter
package monitorfakes

import (
	"context"
	"cred-alert/monitor"
	"sync"

	"github.com/google/go-github/github"
)

type FakeRateClient struct {
	RateLimitsStub        func(context.Context) (*github.RateLimits, *github.Response, error)
	rateLimitsMutex       sync.RWMutex
	rateLimitsArgsForCall []struct {
		arg1 context.Context
	}
	rateLimitsReturns struct {
		result1 *github.RateLimits
		result2 *github.Response
		result3 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeRateClient) RateLimits(arg1 context.Context) (*github.RateLimits, *github.Response, error) {
	fake.rateLimitsMutex.Lock()
	fake.rateLimitsArgsForCall = append(fake.rateLimitsArgsForCall, struct {
		arg1 context.Context
	}{arg1})
	fake.recordInvocation("RateLimits", []interface{}{arg1})
	fake.rateLimitsMutex.Unlock()
	if fake.RateLimitsStub != nil {
		return fake.RateLimitsStub(arg1)
	}
	return fake.rateLimitsReturns.result1, fake.rateLimitsReturns.result2, fake.rateLimitsReturns.result3
}

func (fake *FakeRateClient) RateLimitsCallCount() int {
	fake.rateLimitsMutex.RLock()
	defer fake.rateLimitsMutex.RUnlock()
	return len(fake.rateLimitsArgsForCall)
}

func (fake *FakeRateClient) RateLimitsArgsForCall(i int) context.Context {
	fake.rateLimitsMutex.RLock()
	defer fake.rateLimitsMutex.RUnlock()
	return fake.rateLimitsArgsForCall[i].arg1
}

func (fake *FakeRateClient) RateLimitsReturns(result1 *github.RateLimits, result2 *github.Response, result3 error) {
	fake.RateLimitsStub = nil
	fake.rateLimitsReturns = struct {
		result1 *github.RateLimits
		result2 *github.Response
		result3 error
	}{result1, result2, result3}
}

func (fake *FakeRateClient) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.rateLimitsMutex.RLock()
	defer fake.rateLimitsMutex.RUnlock()
	return fake.invocations
}

func (fake *FakeRateClient) recordInvocation(key string, args []interface{}) {
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

var _ monitor.RateClient = new(FakeRateClient)
