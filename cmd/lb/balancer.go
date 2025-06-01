package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/niglajkin/lab4/httptools"
	"github.com/niglajkin/lab4/signal"
)

var (
	port         = flag.Int("port", 8090, "load balancer port")
	timeoutSec   = flag.Int("timeout-sec", 3, "request timeout time in seconds")
	https        = flag.Bool("https", false, "whether backends support HTTPs")
	traceEnabled = flag.Bool("trace", false, "whether to include tracing information into responses")
)

var (
	serversPool = []string{
		"server1:8080",
		"server2:8080",
		"server3:8080",
	}
)

func scheme() string {
	if *https {
		return "https"
	}
	return "http"
}

var health = func(dst string) bool {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(*timeoutSec)*time.Second)
	req, _ := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s://%s/health", scheme(), dst), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

var forward = func(dst string, rw http.ResponseWriter, r *http.Request) error {
	ctx, _ := context.WithTimeout(r.Context(), time.Duration(*timeoutSec)*time.Second)
	fwdRequest := r.Clone(ctx)
	fwdRequest.RequestURI = ""
	fwdRequest.URL.Host = dst
	fwdRequest.URL.Scheme = scheme()
	fwdRequest.Host = dst

	resp, err := http.DefaultClient.Do(fwdRequest)
	if err != nil {
		log.Printf("Failed to get response from %s: %s", dst, err)
		rw.WriteHeader(http.StatusServiceUnavailable)
		return err
	}
	defer resp.Body.Close()

	for k, values := range resp.Header {
		for _, value := range values {
			rw.Header().Add(k, value)
		}
	}
	if *traceEnabled {
		rw.Header().Set("lb-from", dst)
	}
	log.Println("fwd", resp.StatusCode, resp.Request.URL)

	rw.WriteHeader(resp.StatusCode)
	_, err = io.Copy(rw, resp.Body)
	if err != nil {
		log.Printf("Failed to write response body: %s", err)
	}
	return nil
}

func main() {
	flag.Parse()

	trafficMap := make(map[string]uint64, len(serversPool))
	var mu sync.Mutex

	for _, srv := range serversPool {
		server := srv
		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				log.Println(server, "healthy:", health(server))
			}
		}()
	}

	frontend := httptools.CreateServer(*port, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		healthy := make([]string, 0, len(serversPool))
		for _, srv := range serversPool {
			if health(srv) {
				healthy = append(healthy, srv)
			}
		}
		if len(healthy) == 0 {
			http.Error(rw, "no healthy backends", http.StatusServiceUnavailable)
			return
		}

		mu.Lock()
		chosen := healthy[0]
		min := trafficMap[chosen]
		for _, srv := range healthy[1:] {
			if trafficMap[srv] < min {
				min, chosen = trafficMap[srv], srv
			}
		}
		mu.Unlock()

		countingWriter := &byteCounterResponseWriter{ResponseWriter: rw}
		if err := forward(chosen, countingWriter, r); err != nil {
			return
		}

		mu.Lock()
		trafficMap[chosen] += countingWriter.n
		mu.Unlock()
	}))

	log.Println("Starting load balancer…")
	log.Printf("Tracing support enabled: %t", *traceEnabled)
	frontend.Start()
	signal.WaitForTerminationSignal()
}

type byteCounterResponseWriter struct {
	http.ResponseWriter
	n uint64
}

func (bc *byteCounterResponseWriter) Write(p []byte) (int, error) {
	cnt, err := bc.ResponseWriter.Write(p)
	bc.n += uint64(cnt)
	return cnt, err
}
