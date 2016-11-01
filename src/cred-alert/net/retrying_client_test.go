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
		var successfulResponse *http.Response

		BeforeEach(func() {
			successfulResponse = &http.Response{}
			fakeClient.DoStub = func(req *http.Request) (*http.Response, error) {
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

			clock.WaitForWatcherAndIncrement(750 * time.Millisecond)

			// 2
			Eventually(fakeClient.DoCallCount).Should(Equal(2))
			actualRequest = fakeClient.DoArgsForCall(1)
			Expect(actualRequest.URL).To(Equal(request.URL))
			Expect(actualRequest.Header).To(Equal(request.Header))
			Expect(actualRequest.ContentLength).To(BeNumerically("==", 4))

			actualBody, err = ioutil.ReadAll(actualRequest.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(actualBody).To(Equal([]byte("body")))

			clock.WaitForWatcherAndIncrement(1125 * time.Millisecond)

			// 3
			Eventually(fakeClient.DoCallCount).Should(Equal(3))
			actualRequest = fakeClient.DoArgsForCall(2)
			Expect(actualRequest.URL).To(Equal(request.URL))
			Expect(actualRequest.Header).To(Equal(request.Header))
			Expect(actualRequest.ContentLength).To(BeNumerically("==", 4))

			actualBody, err = ioutil.ReadAll(actualRequest.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(actualBody).To(Equal([]byte("body")))

			clock.WaitForWatcherAndIncrement(1687 * time.Millisecond)

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

			clock.WaitForWatcherAndIncrement(249 * time.Millisecond)
			Expect(fakeClient.DoCallCount()).To(Equal(1))
			clock.WaitForWatcherAndIncrement(501 * time.Millisecond)
			Eventually(fakeClient.DoCallCount).Should(Equal(2))

			clock.WaitForWatcherAndIncrement(374 * time.Millisecond)
			Expect(fakeClient.DoCallCount()).To(Equal(2))
			clock.WaitForWatcherAndIncrement(751 * time.Millisecond)
			Eventually(fakeClient.DoCallCount).Should(Equal(3))

			clock.WaitForWatcherAndIncrement(561 * time.Millisecond)
			Expect(fakeClient.DoCallCount()).To(Equal(3))
			clock.WaitForWatcherAndIncrement(1127 * time.Millisecond)
			Eventually(fakeClient.DoCallCount).Should(Equal(4))
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
			clock.WaitForWatcherAndIncrement(750 * time.Millisecond)

			Eventually(fakeClient.DoCallCount).Should(Equal(2))
			clock.WaitForWatcherAndIncrement(1125 * time.Millisecond)

			Eventually(fakeClient.DoCallCount).Should(Equal(3))
			clock.WaitForWatcherAndIncrement(1687 * time.Millisecond)

			Eventually(fakeClient.DoCallCount).Should(Equal(4))
		})
	})
})
