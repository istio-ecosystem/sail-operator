// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package bookinfo holds helpers shared by the Bookinfo demo services.
package bookinfo

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// ForwardedHeaders is the set of headers propagated from an incoming request
// to outgoing requests, so the Istio proxies can tie together the spans of a
// single distributed trace. The list mirrors the original Bookinfo services,
// which keep it in sync across productpage, details and reviews.
var ForwardedHeaders = []string{
	"x-request-id",
	"x-ot-span-context",
	"x-datadog-trace-id",
	"x-datadog-parent-id",
	"x-datadog-sampling-priority",
	"traceparent",
	"tracestate",
	"x-cloud-trace-context",
	"grpc-trace-bin",
	"x-b3-traceid",
	"x-b3-spanid",
	"x-b3-parentspanid",
	"x-b3-sampled",
	"x-b3-flags",
	"sw8",
	"end-user",
	"user-agent",
	"cookie",
	"authorization",
	"jwt",
}

// Forward copies the headers in ForwardedHeaders from src onto the outgoing
// request dst. Header names are matched case-insensitively.
func Forward(src, dst *http.Request) {
	for _, name := range ForwardedHeaders {
		for _, v := range src.Header.Values(name) {
			dst.Header.Add(name, v)
		}
	}
}

// traceHeaders is the subset of ForwardedHeaders that carry only distributed
// tracing context, excluding session/authentication headers such as
// "cookie", "authorization", "jwt" and "end-user".
var traceHeaders = []string{
	"x-request-id",
	"x-ot-span-context",
	"x-datadog-trace-id",
	"x-datadog-parent-id",
	"x-datadog-sampling-priority",
	"traceparent",
	"tracestate",
	"x-cloud-trace-context",
	"grpc-trace-bin",
	"x-b3-traceid",
	"x-b3-spanid",
	"x-b3-parentspanid",
	"x-b3-sampled",
	"x-b3-flags",
	"sw8",
}

// ForwardTraceHeaders copies only the distributed tracing headers from src
// onto the outgoing request dst. Use this instead of Forward when the
// destination is an external, untrusted service, so session/authentication
// headers are not leaked to it.
func ForwardTraceHeaders(src, dst *http.Request) {
	for _, name := range traceHeaders {
		for _, v := range src.Header.Values(name) {
			dst.Header.Add(name, v)
		}
	}
}

// WriteJSON writes v as a JSON response with the given status code. HTML
// characters are not escaped, matching the original services' output.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

// EnvOr returns the value of the environment variable key, or def if it is
// not set (or empty).
func EnvOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// RunServer starts an HTTP server serving handler with SIGTERM/SIGINT handling
func RunServer(port string, handler http.Handler) {
	srv := &http.Server{Addr: ":" + port, Handler: loggingHandler(handler)}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()
	log.Printf("listening on port %s", port)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	<-ctx.Done()

	log.Print("shutting down gracefully")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown error: %v", err)
	}
}

func loggingHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.RequestURI)
		next.ServeHTTP(w, r)
	})
}
