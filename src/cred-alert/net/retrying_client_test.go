package net_test

import (
	"bytes"
	"cred-alert/net"
	"cred-alert/net/netfakes"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RetryingClient", func() {
	var (
		client     net.Client
		fakeClient *netfakes.FakeClient
		clock      *fakeclock.FakeClock
	)

	BeforeEach(func() {
		fakeClient = &netfakes.FakeClient{}
		clock = fakeclock.NewFakeClock(time.Now())
		client = net.NewRetryingClient(fakeClient, clock)
	})

	It("proxies requests to the underlying client", func() {
		body := strings.NewReader("My Special Body")
		request, err := http.NewRequest("POST", "http://example.com", body)
		Expect(err).NotTo(HaveOccurred())
		request.Header.Add("My-Special", "Header")

		successfulResponse := &http.Response{}
		fakeClient.DoReturns(successfulResponse, nil)

		resp, err := client.Do(request)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp).To(BeIdenticalTo(successfulResponse))

		Expect(fakeClient.DoCallCount()).To(Equal(1))

		actualRequest := fakeClient.DoArgsForCall(0)

		Expect(actualRequest.URL).To(Equal(request.URL))
		Expect(actualRequest.Header).To(Equal(request.Header))
		Expect(actualRequest.Method).To(Equal(request.Method))

		buf := bytes.NewBuffer([]byte{})
		buf.ReadFrom(actualRequest.Body)
		Expect(buf.Bytes()).To(Equal([]byte("My Special Body")))
	})

	Context("when the request fails three times, then succeeds", func() {
		var (
			successfulResponse *http.Response
			startTimes         []time.Time
		)

		BeforeEach(func() {
			successfulResponse = &http.Response{}
			fakeClient.DoStub = func(req *http.Request) (*http.Response, error) {
				startTimes = append(startTimes, clock.Now())

				if fakeClient.DoCallCount() < 4 {
					return nil, errors.New("My Special Error")
				}

				return successfulResponse, nil
			}
		})

		It("retries the request three times", func() {
			request, err := http.NewRequest("GET", "http://example.com", bytes.NewBufferString("body"))
			Expect(err).NotTo(HaveOccurred())

			go func() {
				GinkgoRecover()
				actualResponse, err := client.Do(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(actualResponse).To(BeIdenticalTo(successfulResponse))
			}()

			// 1
			Eventually(fakeClient.DoCallCount).Should(Equal(1))
			actualRequest := fakeClient.DoArgsForCall(0)
			Expect(actualRequest.URL).To(Equal(request.URL))
			Expect(actualRequest.Header).To(Equal(request.Header))
			Expect(actualRequest.ContentLength).To(BeNumerically("==", 4))

			actualBody, err := ioutil.ReadAll(actualRequest.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(actualBody).To(Equal([]byte("body")))

			clock.Increment(750 * time.Millisecond)

			// 2
			Eventually(fakeClient.DoCallCount).Should(Equal(2))
			actualRequest = fakeClient.DoArgsForCall(1)
			Expect(actualRequest.URL).To(Equal(request.URL))
			Expect(actualRequest.Header).To(Equal(request.Header))
			Expect(actualRequest.ContentLength).To(BeNumerically("==", 4))

			actualBody, err = ioutil.ReadAll(actualRequest.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(actualBody).To(Equal([]byte("body")))

			clock.Increment(1125 * time.Millisecond)

			// 3
			Eventually(fakeClient.DoCallCount).Should(Equal(3))
			actualRequest = fakeClient.DoArgsForCall(2)
			Expect(actualRequest.URL).To(Equal(request.URL))
			Expect(actualRequest.Header).To(Equal(request.Header))
			Expect(actualRequest.ContentLength).To(BeNumerically("==", 4))

			actualBody, err = ioutil.ReadAll(actualRequest.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(actualBody).To(Equal([]byte("body")))

			clock.Increment(1687 * time.Millisecond)

			// 4
			Eventually(fakeClient.DoCallCount).Should(Equal(4))
			actualRequest = fakeClient.DoArgsForCall(3)
			Expect(actualRequest.URL).To(Equal(request.URL))
			Expect(actualRequest.Header).To(Equal(request.Header))
			Expect(actualRequest.ContentLength).To(BeNumerically("==", 4))

			actualBody, err = ioutil.ReadAll(actualRequest.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(actualBody).To(Equal([]byte("body")))
		})

		It("retries the request three times with random sleep intervals)", func() {
			request, err := http.NewRequest("GET", "http://example.com", bytes.NewBufferString("body"))
			Expect(err).NotTo(HaveOccurred())

			go func() {
				GinkgoRecover()
				_, err := client.Do(request)
				Expect(err).NotTo(HaveOccurred())
			}()

			Eventually(fakeClient.DoCallCount).Should(Equal(1))
			clock.Increment(750 * time.Millisecond)

			Eventually(fakeClient.DoCallCount).Should(Equal(2))
			clock.Increment(1125 * time.Millisecond)

			Eventually(fakeClient.DoCallCount).Should(Equal(3))
			clock.Increment(1687 * time.Millisecond)

			Eventually(fakeClient.DoCallCount).Should(Equal(4))
			Eventually(startTimes).Should(HaveLen(4))

			Expect(startTimes[1].Sub(startTimes[0])).Should(BeNumerically(">=", 250*time.Millisecond))
			Expect(startTimes[1].Sub(startTimes[0])).Should(BeNumerically("<=", 750*time.Millisecond))

			Expect(startTimes[2].Sub(startTimes[1])).Should(BeNumerically(">=", 375*time.Millisecond))
			Expect(startTimes[2].Sub(startTimes[1])).Should(BeNumerically("<=", 1125*time.Millisecond))

			Expect(startTimes[3].Sub(startTimes[2])).Should(BeNumerically(">=", 562*time.Millisecond))
			Expect(startTimes[3].Sub(startTimes[2])).Should(BeNumerically("<=", 1687*time.Millisecond))
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
				GinkgoRecover()
				_, err := client.Do(request)
				Expect(err).To(MatchError("request failed after retry"))
			}()

			Eventually(fakeClient.DoCallCount).Should(Equal(1))
			clock.Increment(750 * time.Millisecond)

			Eventually(fakeClient.DoCallCount).Should(Equal(2))
			clock.Increment(1125 * time.Millisecond)

			Eventually(fakeClient.DoCallCount).Should(Equal(3))
			clock.Increment(1687 * time.Millisecond)

			Eventually(fakeClient.DoCallCount).Should(Equal(4))
		})
	})
})
