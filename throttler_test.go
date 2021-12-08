package mythrottler

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestThrottlingWrapper_Correctness(t *testing.T) {
	var testSuite = []struct {
		testName         string
		allowedPerSecond int
		requestNum       int
	}{
		{
			testName:         "10 allowed, 10 rps",
			allowedPerSecond: 10,
			requestNum:       10,
		},
		{
			testName:         "100 allowed, 200rps",
			allowedPerSecond: 100,
			requestNum:       200,
		},
		{
			testName:         "300 allowed, 1000rps",
			allowedPerSecond: 300,
			requestNum:       1000,
		},
	}

	var cnt int32
	var startTime time.Time
	var elapsedSeconds float64

	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				cnt++
				elapsedSeconds = time.Since(startTime).Seconds()
				fmt.Printf("request n: %d, time elapsed: %f sec\n", cnt, elapsedSeconds)
			}))

	t.Run("correctness in single-goroutine environment", func(t *testing.T) {
		for _, test := range testSuite {
			t.Run(test.testName, func(t *testing.T) {
				throttled, _ := NewThrottler(
					http.DefaultTransport,
					test.allowedPerSecond,
					time.Second,
					nil,
					nil,
					nil,
					true)

				client := http.Client{Transport: throttled}
				cnt, startTime = 0, time.Now()
				for i := 0; i < test.requestNum; i++ {
					_, _ = client.Get(srv.URL)
				}

				eps := 0.5
				if math.Abs(float64(test.requestNum/test.allowedPerSecond)-elapsedSeconds) > eps {
					t.Errorf("expected and elapsed time difference is too different in test %s: %f\n", test.testName,
						math.Abs(float64(test.requestNum/test.allowedPerSecond)-elapsedSeconds))
				}
			})
		}
	})

	t.Run("correctness in concurrent environment", func(t *testing.T) {

		for _, test := range testSuite {
			t.Run(test.testName, func(t *testing.T) {
				throttled, _ := NewThrottler(
					http.DefaultTransport,
					test.allowedPerSecond,
					time.Second,
					nil,
					nil,
					nil,
					true)

				client := http.Client{Transport: throttled}

				var wg sync.WaitGroup
				cnt, startTime = 0, time.Now()
				for i := 0; i < test.requestNum; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						_, _ = client.Get(srv.URL)
					}()
				}
				wg.Wait()

				eps := 0.5
				if math.Abs(float64(test.requestNum/test.allowedPerSecond)-elapsedSeconds) > eps {
					t.Errorf("expected and elapsed time difference is too different in test %s: %f\n", test.testName,
						math.Abs(float64(test.requestNum/test.allowedPerSecond)-elapsedSeconds))
				}
			})
		}
	})
}

func TestThrottlingWrapper_DontAllowOverqueue(t *testing.T) {

	requestsAllowed := 10

	throttled, _ := NewThrottler(
		http.DefaultTransport,
		requestsAllowed,
		time.Second,
		nil,
		nil,
		nil,
		false)

	var cnt int
	var timeStart time.Time
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		cnt++
		fmt.Printf("request n: %d, time elapsed: %f sec\n", cnt, time.Since(timeStart).Seconds())
	}))

	timeStart = time.Now()
	client := http.Client{Transport: throttled}
	errCount := 0
	after := time.After(3 * time.Second)

	req, _ := http.NewRequest("GET", srv.URL, &net.TCPConn{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-after:
				return
			default:
				_, err := client.Transport.RoundTrip(req)
				if err != nil {
					errCount++
					if err.Error() != OverqueueDisallowedError.Error() {
						fmt.Errorf("expected error: %s; instead got %s\n",
							OverqueueDisallowedError, err.Error())
					}
				}
			}
		}
	}()
	wg.Wait()

	fmt.Println("error count: ", errCount)
	if errCount == 0 {
		t.Error("number of denied requests is expected to be non-zero")
	}
}
