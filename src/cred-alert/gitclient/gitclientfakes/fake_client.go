// This file was generated by counterfeiter
package gitclientfakes

import (
	"cred-alert/gitclient"
	"sync"

	git "github.com/libgit2/git2go"
)

type FakeClient struct {
	CloneStub        func(string, string) (*git.Repository, error)
	cloneMutex       sync.RWMutex
	cloneArgsForCall []struct {
		arg1 string
		arg2 string
	}
	cloneReturns struct {
		result1 *git.Repository
		result2 error
	}
	GetParentsStub        func(*git.Repository, *git.Oid) ([]*git.Oid, error)
	getParentsMutex       sync.RWMutex
	getParentsArgsForCall []struct {
		arg1 *git.Repository
		arg2 *git.Oid
	}
	getParentsReturns struct {
		result1 []*git.Oid
		result2 error
	}
	FetchStub        func(string) (map[string][]*git.Oid, error)
	fetchMutex       sync.RWMutex
	fetchArgsForCall []struct {
		arg1 string
	}
	fetchReturns struct {
		result1 map[string][]*git.Oid
		result2 error
	}
	HardResetStub        func(string, *git.Oid) error
	hardResetMutex       sync.RWMutex
	hardResetArgsForCall []struct {
		arg1 string
		arg2 *git.Oid
	}
	hardResetReturns struct {
		result1 error
	}
	DiffStub        func(repositoryPath string, a, b *git.Oid) (string, error)
	diffMutex       sync.RWMutex
	diffArgsForCall []struct {
		repositoryPath string
		a              *git.Oid
		b              *git.Oid
	}
	diffReturns struct {
		result1 string
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeClient) Clone(arg1 string, arg2 string) (*git.Repository, error) {
	fake.cloneMutex.Lock()
	fake.cloneArgsForCall = append(fake.cloneArgsForCall, struct {
		arg1 string
		arg2 string
	}{arg1, arg2})
	fake.recordInvocation("Clone", []interface{}{arg1, arg2})
	fake.cloneMutex.Unlock()
	if fake.CloneStub != nil {
		return fake.CloneStub(arg1, arg2)
	} else {
		return fake.cloneReturns.result1, fake.cloneReturns.result2
	}
}

func (fake *FakeClient) CloneCallCount() int {
	fake.cloneMutex.RLock()
	defer fake.cloneMutex.RUnlock()
	return len(fake.cloneArgsForCall)
}

func (fake *FakeClient) CloneArgsForCall(i int) (string, string) {
	fake.cloneMutex.RLock()
	defer fake.cloneMutex.RUnlock()
	return fake.cloneArgsForCall[i].arg1, fake.cloneArgsForCall[i].arg2
}

func (fake *FakeClient) CloneReturns(result1 *git.Repository, result2 error) {
	fake.CloneStub = nil
	fake.cloneReturns = struct {
		result1 *git.Repository
		result2 error
	}{result1, result2}
}

func (fake *FakeClient) GetParents(arg1 *git.Repository, arg2 *git.Oid) ([]*git.Oid, error) {
	fake.getParentsMutex.Lock()
	fake.getParentsArgsForCall = append(fake.getParentsArgsForCall, struct {
		arg1 *git.Repository
		arg2 *git.Oid
	}{arg1, arg2})
	fake.recordInvocation("GetParents", []interface{}{arg1, arg2})
	fake.getParentsMutex.Unlock()
	if fake.GetParentsStub != nil {
		return fake.GetParentsStub(arg1, arg2)
	} else {
		return fake.getParentsReturns.result1, fake.getParentsReturns.result2
	}
}

func (fake *FakeClient) GetParentsCallCount() int {
	fake.getParentsMutex.RLock()
	defer fake.getParentsMutex.RUnlock()
	return len(fake.getParentsArgsForCall)
}

func (fake *FakeClient) GetParentsArgsForCall(i int) (*git.Repository, *git.Oid) {
	fake.getParentsMutex.RLock()
	defer fake.getParentsMutex.RUnlock()
	return fake.getParentsArgsForCall[i].arg1, fake.getParentsArgsForCall[i].arg2
}

func (fake *FakeClient) GetParentsReturns(result1 []*git.Oid, result2 error) {
	fake.GetParentsStub = nil
	fake.getParentsReturns = struct {
		result1 []*git.Oid
		result2 error
	}{result1, result2}
}

func (fake *FakeClient) Fetch(arg1 string) (map[string][]*git.Oid, error) {
	fake.fetchMutex.Lock()
	fake.fetchArgsForCall = append(fake.fetchArgsForCall, struct {
		arg1 string
	}{arg1})
	fake.recordInvocation("Fetch", []interface{}{arg1})
	fake.fetchMutex.Unlock()
	if fake.FetchStub != nil {
		return fake.FetchStub(arg1)
	} else {
		return fake.fetchReturns.result1, fake.fetchReturns.result2
	}
}

func (fake *FakeClient) FetchCallCount() int {
	fake.fetchMutex.RLock()
	defer fake.fetchMutex.RUnlock()
	return len(fake.fetchArgsForCall)
}

func (fake *FakeClient) FetchArgsForCall(i int) string {
	fake.fetchMutex.RLock()
	defer fake.fetchMutex.RUnlock()
	return fake.fetchArgsForCall[i].arg1
}

func (fake *FakeClient) FetchReturns(result1 map[string][]*git.Oid, result2 error) {
	fake.FetchStub = nil
	fake.fetchReturns = struct {
		result1 map[string][]*git.Oid
		result2 error
	}{result1, result2}
}

func (fake *FakeClient) HardReset(arg1 string, arg2 *git.Oid) error {
	fake.hardResetMutex.Lock()
	fake.hardResetArgsForCall = append(fake.hardResetArgsForCall, struct {
		arg1 string
		arg2 *git.Oid
	}{arg1, arg2})
	fake.recordInvocation("HardReset", []interface{}{arg1, arg2})
	fake.hardResetMutex.Unlock()
	if fake.HardResetStub != nil {
		return fake.HardResetStub(arg1, arg2)
	} else {
		return fake.hardResetReturns.result1
	}
}

func (fake *FakeClient) HardResetCallCount() int {
	fake.hardResetMutex.RLock()
	defer fake.hardResetMutex.RUnlock()
	return len(fake.hardResetArgsForCall)
}

func (fake *FakeClient) HardResetArgsForCall(i int) (string, *git.Oid) {
	fake.hardResetMutex.RLock()
	defer fake.hardResetMutex.RUnlock()
	return fake.hardResetArgsForCall[i].arg1, fake.hardResetArgsForCall[i].arg2
}

func (fake *FakeClient) HardResetReturns(result1 error) {
	fake.HardResetStub = nil
	fake.hardResetReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeClient) Diff(repositoryPath string, a *git.Oid, b *git.Oid) (string, error) {
	fake.diffMutex.Lock()
	fake.diffArgsForCall = append(fake.diffArgsForCall, struct {
		repositoryPath string
		a              *git.Oid
		b              *git.Oid
	}{repositoryPath, a, b})
	fake.recordInvocation("Diff", []interface{}{repositoryPath, a, b})
	fake.diffMutex.Unlock()
	if fake.DiffStub != nil {
		return fake.DiffStub(repositoryPath, a, b)
	} else {
		return fake.diffReturns.result1, fake.diffReturns.result2
	}
}

func (fake *FakeClient) DiffCallCount() int {
	fake.diffMutex.RLock()
	defer fake.diffMutex.RUnlock()
	return len(fake.diffArgsForCall)
}

func (fake *FakeClient) DiffArgsForCall(i int) (string, *git.Oid, *git.Oid) {
	fake.diffMutex.RLock()
	defer fake.diffMutex.RUnlock()
	return fake.diffArgsForCall[i].repositoryPath, fake.diffArgsForCall[i].a, fake.diffArgsForCall[i].b
}

func (fake *FakeClient) DiffReturns(result1 string, result2 error) {
	fake.DiffStub = nil
	fake.diffReturns = struct {
		result1 string
		result2 error
	}{result1, result2}
}

func (fake *FakeClient) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.cloneMutex.RLock()
	defer fake.cloneMutex.RUnlock()
	fake.getParentsMutex.RLock()
	defer fake.getParentsMutex.RUnlock()
	fake.fetchMutex.RLock()
	defer fake.fetchMutex.RUnlock()
	fake.hardResetMutex.RLock()
	defer fake.hardResetMutex.RUnlock()
	fake.diffMutex.RLock()
	defer fake.diffMutex.RUnlock()
	return fake.invocations
}

func (fake *FakeClient) recordInvocation(key string, args []interface{}) {
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

var _ gitclient.Client = new(FakeClient)
