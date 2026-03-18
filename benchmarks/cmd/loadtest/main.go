package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	vegeta "github.com/tsenart/vegeta/v12/lib"
)

func main() {
	target := flag.String("target", "http://localhost:8080/v1/chat/completions", "gateway endpoint")
	rate := flag.Int("rate", 50, "requests per second")
	duration := flag.Duration("duration", 30*time.Second, "test duration")
	mockPort := flag.Int("mock-port", 9999, "port for mock OpenAI server")
	scenario := flag.String("scenario", "confident", "test scenario: confident, mixed, cache")
	tokenCount := flag.Int("tokens", 20, "tokens per mock response")
	chunkDelay := flag.Duration("chunk-delay", 5*time.Millisecond, "delay between SSE chunks")
	flag.Parse()

	mockAddr := fmt.Sprintf(":%d", *mockPort)
	srv := startMockServer(mockAddr, *scenario, *tokenCount, *chunkDelay)
	defer srv.Close()

	ready := false
	for i := range 50 {
		_ = i
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/v1/embeddings", *mockPort))
		if err == nil {
			resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		fmt.Fprintf(os.Stderr, "mock server did not start on port %d\n", *mockPort)
		os.Exit(1)
	}

	var targeter vegeta.Targeter

	switch *scenario {
	case "cache":
		body := []byte(`{"model":"auto","messages":[{"role":"user","content":"What is 2+2?"}],"stream":true}`)
		targeter = vegeta.NewStaticTargeter(vegeta.Target{
			Method: "POST",
			URL:    *target,
			Header: http.Header{"Content-Type": []string{"application/json"}},
			Body:   body,
		})
	default:
		targeter = newVaryingTargeter(*target)
	}

	attacker := vegeta.NewAttacker()
	rateObj := vegeta.Rate{Freq: *rate, Per: time.Second}

	var metrics vegeta.Metrics
	for res := range attacker.Attack(targeter, rateObj, *duration, *scenario) {
		metrics.Add(res)
	}
	metrics.Close()

	fmt.Printf("\nLoad Test Results (%s scenario)\n", *scenario)
	fmt.Printf("Duration:    %s\n", metrics.Duration.Round(time.Millisecond))
	fmt.Printf("Requests:    %d\n", metrics.Requests)
	fmt.Printf("Rate:        %.2f req/s\n", metrics.Rate)
	fmt.Printf("Throughput:  %.2f req/s\n", metrics.Throughput)
	fmt.Printf("Success:     %.2f%%\n", metrics.Success*100)
	fmt.Println()
	fmt.Println("Latency:")
	fmt.Printf("  P50:  %s\n", metrics.Latencies.P50)
	fmt.Printf("  P95:  %s\n", metrics.Latencies.P95)
	fmt.Printf("  P99:  %s\n", metrics.Latencies.P99)
	fmt.Printf("  Max:  %s\n", metrics.Latencies.Max)
	fmt.Printf("  Mean: %s\n", metrics.Latencies.Mean)
	fmt.Println()
	fmt.Println("Status Codes:")
	for code, count := range metrics.StatusCodes {
		fmt.Printf("  %s: %d\n", code, count)
	}
	if len(metrics.Errors) > 0 {
		fmt.Println()
		fmt.Println("Errors:")
		for _, e := range metrics.Errors {
			fmt.Printf("  %s\n", e)
		}
	}
}

func newVaryingTargeter(url string) vegeta.Targeter {
	return func(tgt *vegeta.Target) error {
		n := rand.Intn(1000000)
		body := fmt.Sprintf(`{"model":"auto","messages":[{"role":"user","content":"Question number %d: what is 2+2?"}],"stream":true}`, n)
		tgt.Method = "POST"
		tgt.URL = url
		tgt.Header = http.Header{"Content-Type": []string{"application/json"}}
		tgt.Body = []byte(body)
		return nil
	}
}
