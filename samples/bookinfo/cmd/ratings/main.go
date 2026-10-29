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

// The ratings service returns star ratings for books.
//
// This is a pure-Go re-implementation of the original Node.js ratings service
// from samples/bookinfo. The MongoDB/MySQL backends are replaced by
// hard-coded in-memory data (the seeded values from the original database
// init scripts), so the service runs with zero external dependencies.
//
// Behavior is selected with the SERVICE_VERSION environment variable:
//
//	v1            (default) in-memory ratings, supports POST
//	v2            "database-backed" ratings (hard-coded seed data), no POST
//	v-faulty      returns 503 for half the requests
//	v-delayed     delays half the requests by 7 seconds
//	v-unavailable toggles availability every 60 seconds
//	v-unhealthy   toggles health (and availability) every 15 minutes
package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"bookinfo/internal/bookinfo"
)

// ratingsResponse is the JSON document returned for a product.
type ratingsResponse struct {
	ID      int            `json:"id"`
	Ratings map[string]any `json:"ratings"`
}

// defaultRatings are the values seeded in the original MongoDB/MySQL
// databases (ratings_data.json / mysqldb-init.sql).
var defaultRatings = map[string]any{
	"Reviewer1": 5,
	"Reviewer2": 4,
}

var (
	mu               sync.Mutex
	userAddedRatings = map[int]map[string]any{} // used to demonstrate POST functionality
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("usage: %s port", os.Args[0])
	}
	port := os.Args[1]
	version := bookinfo.EnvOr("SERVICE_VERSION", "v1")
	log.Printf("start at port %s (version %s)\n", port, version)
	bookinfo.RunServer(port, newServer(version))
}

func newServer(version string) http.Handler {
	var unavailable, healthy atomic.Bool
	healthy.Store(true)

	switch version {
	case "v-unavailable":
		// Make the service unavailable once in 60 seconds.
		go toggleEvery(60*time.Second, func() { unavailable.Store(!unavailable.Load()) })
	case "v-unhealthy":
		// Make the service unavailable once in 15 minutes for 15 minutes.
		// 15 minutes is chosen since the Kubernetes's exponential back-off is reset after 10
		// minutes of successful execution.
		go toggleEvery(15*time.Minute, func() {
			healthy.Store(!healthy.Load())
			unavailable.Store(!unavailable.Load())
		})
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		if healthy.Load() {
			bookinfo.WriteJSON(w, http.StatusOK, map[string]string{"status": "Ratings is healthy"})
			return
		}
		bookinfo.WriteJSON(w, http.StatusInternalServerError, map[string]string{"status": "Ratings is not healthy"})
	})

	mux.HandleFunc("/ratings/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			bookinfo.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "please provide numeric product ID"})
			return
		}
		switch r.Method {
		case http.MethodGet:
			handleGet(w, id, version, &unavailable)
		case http.MethodPost:
			handlePost(w, r, id, version)
		default:
			bookinfo.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	})

	return mux
}

func toggleEvery(d time.Duration, f func()) {
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for range ticker.C {
		f()
	}
}

func handleGet(w http.ResponseWriter, id int, version string, unavailable *atomic.Bool) {
	// We default to using in-memory data, if SERVICE_VERSION is not set to v2.
	if version == "v2" {
		bookinfo.WriteJSON(w, http.StatusOK, ratingsResponse{ID: id, Ratings: defaultRatings})
		return
	}

	switch version {
	case "v-faulty":
		// In half of the cases return error, in another half proceed as usual.
		if rand.Float64() <= 0.5 {
			bookinfo.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Service unavailable"})
			return
		}
	case "v-delayed":
		// In half of the cases delay for 7 seconds, in another half proceed as usual.
		if rand.Float64() <= 0.5 {
			time.Sleep(7 * time.Second)
		}
	case "v-unavailable", "v-unhealthy":
		if unavailable.Load() {
			bookinfo.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Service unavailable"})
			return
		}
	}

	bookinfo.WriteJSON(w, http.StatusOK, localReviews(id))
}

func handlePost(w http.ResponseWriter, r *http.Request, id int, version string) {
	var ratings map[string]any
	if err := json.NewDecoder(r.Body).Decode(&ratings); err != nil || ratings == nil {
		bookinfo.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "please provide valid ratings JSON"})
		return
	}

	if version == "v2" { // the version that is "backed" by a database
		bookinfo.WriteJSON(w, http.StatusNotImplemented, map[string]string{"error": "Post not implemented for database backed ratings"})
		return
	}

	mu.Lock()
	userAddedRatings[id] = ratings
	mu.Unlock()

	bookinfo.WriteJSON(w, http.StatusOK, localReviews(id))
}

func localReviews(id int) ratingsResponse {
	mu.Lock()
	defer mu.Unlock()
	if ratings, ok := userAddedRatings[id]; ok {
		return ratingsResponse{ID: id, Ratings: ratings}
	}
	return ratingsResponse{ID: id, Ratings: defaultRatings}
}
