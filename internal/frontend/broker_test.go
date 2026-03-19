package frontend

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SSEBroker", func() {
	var broker *SSEBroker

	BeforeEach(func() {
		broker = NewSSEBroker()
		broker.SetPingInterval(1 * time.Second)
	})

	Describe("Broadcast", func() {
		It("should send events to all connected clients", func() {
			server := httptest.NewServer(broker)
			defer server.Close()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)

			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())

			defer resp.Body.Close()

			Eventually(func() int {
				return broker.ClientCount()
			}, "5s", "100ms").Should(Equal(1))

			broker.Broadcast("test-event", "refresh")

			reader := bufio.NewReader(resp.Body)
			found := make(chan bool, 1)

			go func() {
				defer GinkgoRecover()

				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}

					if strings.Contains(line, "event: test-event") {
						found <- true

						return
					}
				}
			}()

			Eventually(found, "5s").Should(Receive(BeTrue()))
		})

		It("should format multiline data payloads correctly per SSE spec", func() {
			server := httptest.NewServer(broker)
			defer server.Close()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())

			defer resp.Body.Close()

			Eventually(func() int { return broker.ClientCount() }, "5s", "100ms").Should(Equal(1))

			broker.Broadcast("multi-event", "line1\nline2")

			reader := bufio.NewReader(resp.Body)
			found := make(chan bool, 1)

			go func() {
				defer GinkgoRecover()

				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}

					if strings.Contains(line, "event: multi-event") {
						data1, _ := reader.ReadString('\n')
						data2, _ := reader.ReadString('\n')

						if data1 == "data: line1\n" && data2 == "data: line2\n" {
							found <- true
						}

						return
					}
				}
			}()

			Eventually(found, "5s").Should(Receive(BeTrue()))
		})
	})

	Describe("Client Management", func() {
		It("should remove clients on disconnect", func() {
			server := httptest.NewServer(broker)
			defer server.Close()

			ctx, cancel := context.WithCancel(context.Background())
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)

			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() int { return broker.ClientCount() }).Should(Equal(1))

			resp.Body.Close()
			cancel()

			Eventually(func() int { return broker.ClientCount() }, "5s").Should(Equal(0))
		})

		It("should disconnect all clients safely without panicking", func() {
			server := httptest.NewServer(broker)
			defer server.Close()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())

			defer resp.Body.Close()

			Eventually(func() int { return broker.ClientCount() }).Should(Equal(1))
			Expect(func() { broker.CloseAll() }).NotTo(Panic())
			Eventually(func() int { return broker.ClientCount() }, "5s").Should(Equal(0))
		})
	})
})
