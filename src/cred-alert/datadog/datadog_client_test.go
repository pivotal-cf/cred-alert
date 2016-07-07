package datadog_test

import (
	"cred-alert/datadog"
	"encoding/json"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

type request struct {
	Series datadog.Series `json:"series"`
}

var _ = Describe("Datadog", func() {
	Describe("Client", func() {
		Describe("BuildCountMetric", func() {
			var client datadog.Client

			BeforeEach(func() {
				client = datadog.NewClient("api-key")
			})

			It("sets the counter name", func() {
				countMetric := client.BuildMetric(datadog.CounterMetricType, "countMetricName", 0)
				Expect(countMetric.Name).To(Equal("countMetricName"))
			})

			It("sets the count as a point with current time", func() {
				countMetric := client.BuildMetric(datadog.CounterMetricType, "countMetricName", 123)
				Expect(countMetric.Points).To(HaveLen(1))
				Expect(countMetric.Points[0].Timestamp).To(BeTemporally("~", time.Now(), time.Second))
				Expect(countMetric.Points[0].Value).To(Equal(float32(123)))
			})

			It("sets tags if given", func() {
				countMetric := client.BuildMetric(datadog.CounterMetricType, "countMetricName", 123, "tag1", "tag2")
				Expect(countMetric.Tags).To(HaveLen(2))
				Expect(countMetric.Tags[0]).To(Equal("tag1"))
				Expect(countMetric.Tags[1]).To(Equal("tag2"))
			})
		})

		Describe("PublishSeries", func() {
			var (
				server *ghttp.Server
			)

			BeforeEach(func() {
				server = ghttp.NewServer()
				datadog.APIURL = server.URL()
			})

			AfterEach(func() {
				if server != nil {
					server.Close()
				}
			})

			Context("when everything's great", func() {
				now := time.Now()

				BeforeEach(func() {
					server.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/api/v1/series", "api_key=api-key"),
						func(w http.ResponseWriter, r *http.Request) {
							var request request
							Expect(json.NewDecoder(r.Body).Decode(&request)).To(Succeed())
							metric := request.Series[0]

							Expect(metric.Name).To(Equal("memory.limit"))
							Expect(metric.Host).To(Equal("web-0"))
							Expect(metric.Tags).To(ConsistOf("application:atc"))

							Expect(metric.Points[0].Timestamp).NotTo(BeZero())
							Expect(metric.Points[0].Value).To(BeNumerically("~", 4.52, 0.01))

							Expect(metric.Points[1].Timestamp).To(Equal(time.Unix(now.Unix(), 0)))
							Expect(metric.Points[1].Value).To(BeNumerically("~", 23.22, 0.01))

							Expect(metric.Points[2].Timestamp).To(Equal(time.Unix(now.Unix(), 0)))
							Expect(metric.Points[2].Value).To(BeNumerically("~", 23.25, 0.01))
						},
						ghttp.RespondWith(http.StatusAccepted, "{}"),
					))
				})

				It("works", func() {
					client := datadog.NewClient("api-key")

					err := client.PublishSeries(datadog.Series{
						{
							Name: "memory.limit",
							Points: []datadog.Point{
								{now, 4.52},
								{now, 23.22},
								{now, 23.25},
							},
							Host: "web-0",
							Tags: []string{"application:atc"},
						},
					})

					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("when the server does not respond", func() {
				BeforeEach(func() {
					server.Close()
					server = nil
				})

				It("returns an error", func() {
					client := datadog.NewClient("api-key")

					err := client.PublishSeries(datadog.Series{})
					Expect(err).To(HaveOccurred())
				})
			})

			Context("when the server does not respond with 202", func() {
				BeforeEach(func() {
					server.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/api/v1/series", "api_key=api-key"),
						ghttp.RespondWith(http.StatusInternalServerError, "{}"),
					))
				})

				It("returns an error", func() {
					client := datadog.NewClient("api-key")

					err := client.PublishSeries(datadog.Series{})
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})
})
