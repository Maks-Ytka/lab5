package integration

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

const baseAddress = "http://balancer:8090"

var client = http.Client{
	Timeout: 3 * time.Second,
}

func TestBalancer(t *testing.T) {
	if _, exists := os.LookupEnv("INTEGRATION_TEST"); !exists {
		t.Skip("Integration test is not enabled")
	}

	const numRequests = 5
	fromSet := make(map[string]struct{})

	for i := range [numRequests]int{} {
		resp, err := client.Get(fmt.Sprintf("%s/api/v1/some-data", baseAddress))
		if err != nil {
			t.Fatalf("request %d failed: %v", i+1, err)
		}
		resp.Body.Close()

		from := resp.Header.Get("lb-from")
		if from == "" {
			t.Fatalf("request %d returned no lb-from header", i+1)
		}
		t.Logf("request %d from [%s]", i+1, from)
		fromSet[from] = struct{}{}

		time.Sleep(100 * time.Millisecond)
	}

	if len(fromSet) < 2 {
		t.Errorf("expected requests to be routed to at least 2 different backends, got %d unique", len(fromSet))
	}
}

func BenchmarkBalancer(b *testing.B) {
	if _, exists := os.LookupEnv("INTEGRATION_TEST"); !exists {
		b.Skip("Integration benchmark is not enabled")
	}

	for i := range make([]int, b.N) {
		resp, err := client.Get(fmt.Sprintf("%s/api/v1/some-data", baseAddress))
		if err != nil {
			b.Fatalf("request %d failed: %v", i+1, err)
		}
		if resp.StatusCode != http.StatusOK {
			b.Fatalf("request %d returned status %d", i+1, resp.StatusCode)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}
