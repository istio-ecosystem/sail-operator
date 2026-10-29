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

// The reviews service returns book reviews, optionally decorated with star
// ratings from the ratings service.
//
// This is a pure-Go re-implementation of the original reviews service from
// samples/bookinfo (which was a Java WebSphere Liberty application). All
// review text is hard-coded.
//
// Behavior is selected with environment variables (as in the original
// images):
//
//	SERVICE_VERSION   v1, v2 or v3 (informational only)
//	ENABLE_RATINGS    "true" for v2/v3: fetch star ratings from the ratings service
//	STAR_COLOR        "black" (v2) or "red" (v3)
//
// v3 additionally uses a short (2.5s) timeout to the ratings service, which
// is what makes the v3 red stars visibly flake when the ratings service is
// faulty or delayed.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"bookinfo/internal/bookinfo"
)

var (
	ratingsEnabled = os.Getenv("ENABLE_RATINGS") == "true"
	starColor      = bookinfo.EnvOr("STAR_COLOR", "black")
	podName        = os.Getenv("HOSTNAME")
	clusterName    = bookinfo.EnvOr("CLUSTER_NAME", "null")
)

// rating is either a star rating or an error explaining why no rating is
// available.
type rating struct {
	Stars int    `json:"stars,omitempty"`
	Color string `json:"color,omitempty"`
	Error string `json:"error,omitempty"`
}

// review is a single book review.
type review struct {
	Reviewer string  `json:"reviewer"`
	Text     string  `json:"text"`
	Rating   *rating `json:"rating,omitempty"`
}

// reviewsResponse is the JSON document returned for a product.
type reviewsResponse struct {
	ID          string   `json:"id"`
	PodName     string   `json:"podname"`
	ClusterName string   `json:"clustername"`
	Reviews     []review `json:"reviews"`
}

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("usage: %s port", os.Args[0])
	}
	port := os.Args[1]
	log.Printf("start at port %s (ratings enabled: %t, star color: %s)", port, ratingsEnabled, starColor)
	bookinfo.RunServer(port, newServer())
}

func newServer() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		bookinfo.WriteJSON(w, http.StatusOK, map[string]string{"status": "Reviews is healthy"})
	})

	mux.HandleFunc("GET /reviews/{id}", func(w http.ResponseWriter, r *http.Request) {
		// The original (JAX-RS) endpoint only matches integer product ids
		// and returns 404 for anything else.
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.NotFound(w, r)
			return
		}

		stars1, stars2 := -1, -1
		if ratingsEnabled {
			if ratings := getRatings(r, id); ratings != nil {
				if v, ok := (*ratings)["Reviewer1"]; ok {
					stars1 = v
				}
				if v, ok := (*ratings)["Reviewer2"]; ok {
					stars2 = v
				}
			}
		}

		bookinfo.WriteJSON(w, http.StatusOK, buildResponse(id, stars1, stars2))
	})

	return mux
}

// ratingsServiceURL builds the base URL of the ratings service from the same
// environment variables the original used.
func ratingsServiceURL() string {
	domain := ""
	if d := os.Getenv("SERVICES_DOMAIN"); d != "" {
		domain = "." + d
	}
	host := bookinfo.EnvOr("RATINGS_HOSTNAME", "ratings")
	port := bookinfo.EnvOr("RATINGS_SERVICE_PORT", "9080")
	return fmt.Sprintf("http://%s%s:%s/ratings", host, domain, port)
}

// getRatings fetches the ratings for a product from the ratings service, or
// nil if the service cannot be reached or returns an error.
//
// The timeout matches the original: 10s for the black-star version, 2.5s for
// the red-star version (which is what makes the v3 red stars visibly flake).
func getRatings(r *http.Request, id int) *map[string]int {
	timeout := 10 * time.Second
	if starColor != "black" {
		timeout = 2500 * time.Millisecond
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, fmt.Sprintf("%s/%d", ratingsServiceURL(), id), nil)
	if err != nil {
		log.Printf("Error: unable to create request: %v", err)
		return nil
	}
	bookinfo.Forward(r, req)

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error: unable to contact %s: %v", req.URL, err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error: unable to contact %s got status of %d", req.URL, resp.StatusCode)
		return nil
	}

	var out struct {
		Ratings map[string]int `json:"ratings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		log.Printf("Error: unable to decode response from %s: %v", req.URL, err)
		return nil
	}
	return &out.Ratings
}

func buildResponse(id, stars1, stars2 int) reviewsResponse {
	mk := func(reviewer, text string, stars int) review {
		rv := review{Reviewer: reviewer, Text: text}
		if ratingsEnabled {
			if stars != -1 {
				rv.Rating = &rating{Stars: stars, Color: starColor}
			} else {
				rv.Rating = &rating{Error: "Ratings service is currently unavailable"}
			}
		}
		return rv
	}

	return reviewsResponse{
		ID:          strconv.Itoa(id),
		PodName:     podName,
		ClusterName: clusterName,
		Reviews: []review{
			mk("Reviewer1", "An extremely entertaining play by Shakespeare. The slapstick humour is refreshing!", stars1),
			mk("Reviewer2", "Absolutely fun and entertaining. The play lacks thematic depth when compared to other plays by Shakespeare.", stars2),
		},
	}
}
