package net_test

import (
	"bytes"
	"cred-alert/net"
	"cred-alert/net/netfakes"
	"errors"
	"net/http"
	"strings"

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
		request, _ := http.NewRequest("POST", "http://example.com", body)
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
		doCalls := 0

		expectedResponse := &http.Response{}
		err := errors.New("My Special Error")

		fakeClient.DoStub = func(req *http.Request) (*http.Response, error) {
			doCalls += 1

			if doCalls < 4 {
				return nil, err
			}

			return expectedResponse, nil
		}

		request, _ := http.NewRequest("GET", "http://example.com", nil)

		actualResponse, err := client.Do(request)
		Expect(err).ToNot(HaveOccurred())
		Expect(actualResponse).To(BeIdenticalTo(expectedResponse))

		Expect(fakeClient.DoCallCount()).To(Equal(4))

		actualRequest := fakeClient.DoArgsForCall(3)
		Expect(actualRequest).To(Equal(request))
	})

	It("errors after three requests fail", func() {
		expectedError := errors.New("My Special Error")
		fakeClient.DoReturns(nil, expectedError)

		request, _ := http.NewRequest("GET", "http://example.com", nil)
		_, err := client.Do(request)
		Expect(err).To(Equal(expectedError))

		Expect(fakeClient.DoCallCount()).To(Equal(4))

		actualRequest := fakeClient.DoArgsForCall(3)
		Expect(actualRequest).To(Equal(request))
	})
})
