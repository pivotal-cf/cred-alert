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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RetryingClient", func() {
	var (
		client     net.Client
		fakeClient *netfakes.FakeClient
	)

	BeforeEach(func() {
		fakeClient = &netfakes.FakeClient{}
		client = net.NewRetryingClient(fakeClient)
	})

	It("proxies requests to the underlying client", func() {
		body := strings.NewReader("My Special Body")
		request, err := http.NewRequest("POST", "http://example.com", body)
		Expect(err).NotTo(HaveOccurred())
		request.Header.Add("My-Special", "Header")

		expectedResponse := &http.Response{}
		fakeClient.DoReturns(expectedResponse, nil)

		resp, err := client.Do(request)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp).To(BeIdenticalTo(expectedResponse))

		Expect(fakeClient.DoCallCount()).To(Equal(1))

		actualRequest := fakeClient.DoArgsForCall(0)

		Expect(actualRequest.URL).To(Equal(request.URL))
		Expect(actualRequest.Header).To(Equal(request.Header))
		Expect(actualRequest.Method).To(Equal(request.Method))

		buf := bytes.NewBuffer([]byte{})
		buf.ReadFrom(actualRequest.Body)
		Expect(buf.Bytes()).To(Equal([]byte("My Special Body")))
	})

	It("retries the request when the first three requests fail", func() {
		expectedResponse := &http.Response{}
		err := errors.New("My Special Error")

		fakeClient.DoStub = func(req *http.Request) (*http.Response, error) {
			if fakeClient.DoCallCount() < 4 {
				return nil, errors.New("My Special Error")
			}

			return expectedResponse, nil
		}

		body := []byte("body")

		request, err := http.NewRequest("GET", "http://example.com", bytes.NewBuffer(body))
		Expect(err).NotTo(HaveOccurred())

		actualResponse, err := client.Do(request)
		Expect(err).ToNot(HaveOccurred())
		Expect(actualResponse).To(BeIdenticalTo(expectedResponse))

		Expect(fakeClient.DoCallCount()).To(Equal(4))

		actualRequest := fakeClient.DoArgsForCall(3)
		Expect(actualRequest.URL).To(Equal(request.URL))
		Expect(actualRequest.Header).To(Equal(request.Header))
		Expect(actualRequest.ContentLength).To(BeNumerically("==", 4))

		actualBody, err := ioutil.ReadAll(actualRequest.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(actualBody).To(Equal([]byte("body")))
	})

	It("retries the first request after random time (between 0.25 seconds and 0.75 seconds)", func() {
		expectedResponse := &http.Response{}
		var startTime []time.Time

		fakeClient.DoStub = func(req *http.Request) (*http.Response, error) {
			startTime = append(startTime, time.Now())

			if fakeClient.DoCallCount() < 4 {
				return nil, errors.New("My Special Error")
			}

			return expectedResponse, nil
		}

		request, err := http.NewRequest("GET", "http://example.com", bytes.NewBufferString("body"))
		Expect(err).NotTo(HaveOccurred())

		_, err = client.Do(request)
		Expect(err).NotTo(HaveOccurred())

		Expect(len(startTime)).To(Equal(4))
		Expect(startTime[1].Sub(startTime[0])).Should(BeNumerically(">=", 250*time.Millisecond))
		Expect(startTime[1].Sub(startTime[0])).Should(BeNumerically("<=", 750*time.Millisecond))

		Expect(startTime[2].Sub(startTime[1])).Should(BeNumerically(">=", 375*time.Millisecond))
		Expect(startTime[2].Sub(startTime[1])).Should(BeNumerically("<=", 1125*time.Millisecond))

		Expect(startTime[3].Sub(startTime[2])).Should(BeNumerically(">=", 562*time.Millisecond))
		Expect(startTime[3].Sub(startTime[2])).Should(BeNumerically("<=", 1687*time.Millisecond))
	})

	It("errors after three requests fail", func() {
		fakeClient.DoReturns(nil, errors.New("disaster"))

		request, err := http.NewRequest("GET", "http://example.com", bytes.NewBufferString("body"))
		Expect(err).NotTo(HaveOccurred())

		_, err = client.Do(request)
		Expect(err).To(MatchError("request failed after retry"))

		Expect(fakeClient.DoCallCount()).To(Equal(4))

		actualRequest := fakeClient.DoArgsForCall(3)
		Expect(actualRequest.URL).To(Equal(request.URL))
		Expect(actualRequest.Header).To(Equal(request.Header))
		Expect(actualRequest.ContentLength).To(BeNumerically("==", 4))

		actualBody, err := ioutil.ReadAll(actualRequest.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(actualBody).To(Equal([]byte("body")))
	})
})
