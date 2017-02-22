package net_test

import (
	"bytes"
	"cred-alert/net"
	"cred-alert/net/netfakes"
	"errors"
	"net/http"
	"strings"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"

	"fmt"
	"net/http/httputil"

	"io/ioutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

var _ = Describe("RetryingClient", func() {
	var (
		client     net.Client
		fakeClient *netfakes.FakeClient
		clock      *fakeclock.FakeClock
		epoch      time.Time
	)

	BeforeEach(func() {
		fakeClient = &netfakes.FakeClient{}
		epoch = time.Now()
		clock = fakeclock.NewFakeClock(epoch)
		client = net.NewRetryingClient(fakeClient, clock)
	})

	It("proxies requests to the underlying client", func() {
		request, err := http.NewRequest("POST", "http://example.com", strings.NewReader("My Special Body"))
		Expect(err).NotTo(HaveOccurred())
		request.Header.Add("My-Special", "Header")

		successfulResponse := &http.Response{}
		fakeClient.DoReturns(successfulResponse, nil)

		resp, err := client.Do(request)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp).To(BeIdenticalTo(successfulResponse))

		Expect(fakeClient.DoCallCount()).To(Equal(1))

		actualRequest := fakeClient.DoArgsForCall(0)

		Expect(actualRequest).To(BeSameRequestAs(request))
		buf := bytes.NewBuffer([]byte{})
		buf.ReadFrom(actualRequest.Body)
		Expect(buf.Bytes()).To(Equal([]byte("My Special Body")))
	})

	Context("when the request fails for less than 1 minute, then succeeds", func() {
		var successfulResponse *http.Response

		BeforeEach(func() {
			successfulResponse = &http.Response{}
			fakeClient.DoStub = func(req *http.Request) (*http.Response, error) {
				if clock.Since(epoch) < 50*time.Second {
					return nil, errors.New("My Special Error")
				}

				return successfulResponse, nil
			}
		})

		It("retries the request as many times as required (up to the 1 minute)", func() {
			request, err := http.NewRequest("GET", "http://example.com", bytes.NewBufferString("body"))
			Expect(err).NotTo(HaveOccurred())

			go func() {
				// time goes on...
				for {
					clock.Increment(1 * time.Millisecond)
				}
			}()

			actualResponse, err := client.Do(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(actualResponse).To(BeIdenticalTo(successfulResponse))

			Expect(fakeClient.DoCallCount()).Should(BeNumerically(">", 1))

			for i := 0; i < fakeClient.DoCallCount(); i++ {
				actualRequest := fakeClient.DoArgsForCall(i)

				Expect(actualRequest).To(BeSameRequestAs(request))
				actualBody, err := ioutil.ReadAll(actualRequest.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(actualBody).To(Equal([]byte("body")))
			}
		})
	})

	Context("when the request continually fails", func() {
		BeforeEach(func() {
			fakeClient.DoReturns(nil, errors.New("disaster"))
		})

		It("returns an error", func() {
			request, err := http.NewRequest("GET", "http://example.com", bytes.NewBufferString("body"))
			Expect(err).NotTo(HaveOccurred())

			go func() {
				// time goes on...
				for {
					clock.Increment(1 * time.Millisecond)
				}
			}()

			_, err = client.Do(request)
			Expect(err).To(MatchError("request failed after retry: disaster"))

			Expect(fakeClient.DoCallCount()).Should(BeNumerically(">", 1))

			for i := 0; i < fakeClient.DoCallCount(); i++ {
				actualRequest := fakeClient.DoArgsForCall(i)

				Expect(actualRequest).To(BeSameRequestAs(request))
				actualBody, err := ioutil.ReadAll(actualRequest.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(actualBody).To(Equal([]byte("body")))
			}
		})
	})
})

func BeSameRequestAs(expected *http.Request) types.GomegaMatcher {
	return &beSameRequestMatcher{
		expected: expected,
	}
}

type beSameRequestMatcher struct {
	expected *http.Request
}

func (matcher *beSameRequestMatcher) Match(actual interface{}) (success bool, err error) {
	request, ok := actual.(*http.Request)
	if !ok {
		return false, fmt.Errorf("BeSameRequestAs matcher expects an http.Request")
	}

	expectedDump, err := httputil.DumpRequestOut(matcher.expected, false)
	if err != nil {
		return false, fmt.Errorf("Failed to dump expected request: %s", err.Error())
	}

	actualDump, err := httputil.DumpRequestOut(request, false)
	if err != nil {
		return false, fmt.Errorf("Failed to dump actual request: %s", err.Error())
	}

	return Equal(expectedDump).Match(actualDump)
}

func (matcher *beSameRequestMatcher) FailureMessage(actual interface{}) (message string) {
	expectedDump := matcher.requestDump(matcher.expected)
	actualDump := matcher.requestDump(actual)

	return fmt.Sprintf("Expected\n\t%q\nto be same request as\n\t%q", actualDump, expectedDump)
}

func (matcher *beSameRequestMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	expectedDump := matcher.requestDump(matcher.expected)
	actualDump := matcher.requestDump(actual)

	return fmt.Sprintf("Expected\n\t%q\n not to be same request as\n\t%q", actualDump, expectedDump)
}

func (matcher *beSameRequestMatcher) requestDump(actual interface{}) []byte {
	request, ok := actual.(*http.Request)
	if !ok {
		panic("unexpected type!")
	}

	dump, err := httputil.DumpRequestOut(request, false)
	if err != nil {
		panic("error dumping request: " + err.Error())
	}

	return dump
}
