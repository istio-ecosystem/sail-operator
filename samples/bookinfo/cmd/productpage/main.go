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

// The productpage service renders the book info web page and the JSON API.
//
// This is a pure-Go re-implementation of the original Python/Flask
// productpage service from samples/bookinfo. It has no external dependencies:
// templates and static assets are embedded in the binary (go:embed), the
// session is a plain cookie, and the /metrics endpoint emits the
// request_result counter in Prometheus text format.
//
// The FLOOD_FACTOR environment variable, when > 0, makes /productpage issue
// FLOOD_FACTOR extra (discarded) requests to the reviews service before
// rendering the page, to demonstrate Istio rate limiting.
package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"bookinfo/internal/bookinfo"
)

//go:embed templates/* static/*
var webAssets embed.FS

const (
	detailsName = "details"
	reviewsName = "reviews"
	ratingsName = "ratings"

	productID = 0 // TODO: replace default value (same as the original)
)

var (
	servicesDomain = os.Getenv("SERVICES_DOMAIN")
	detailsBase    = serviceURL("DETAILS_HOSTNAME", "details", "DETAILS_SERVICE_PORT", "9080", detailsName)
	reviewsBase    = serviceURL("REVIEWS_HOSTNAME", "reviews", "REVIEWS_SERVICE_PORT", "9080", reviewsName)
	ratingsBase    = serviceURL("RATINGS_HOSTNAME", "ratings", "RATINGS_SERVICE_PORT", "9080", ratingsName)
	floodFactor    = 0
)

// serviceURL builds an upstream service base URL (including its endpoint
// prefix), honoring the SERVICES_DOMAIN suffix and the per-service
// HOSTNAME/PORT overrides.
func serviceURL(hostEnv, host, portEnv, port, endpoint string) string {
	domain := ""
	if servicesDomain != "" {
		domain = "." + servicesDomain
	}
	return fmt.Sprintf("http://%s%s:%s/%s",
		bookinfo.EnvOr(hostEnv, host), domain, bookinfo.EnvOr(portEnv, port), endpoint)
}

const (
	productTitle                     = "The Comedy of Errors"
	productDescription template.HTML = `<a href="https://en.wikipedia.org/wiki/The_Comedy_of_Errors">Wikipedia Summary</a>: The Comedy of ` +
		`Errors is one of <b>William Shakespeare's</b> early plays. It is his shortest and one of his most farcical ` +
		`comedies, with a major part of the humour coming from slapstick and mistaken identity, in addition to puns and word play.`
)

// products is the single-book sample catalog served by /api/v1/products.
var products = []map[string]any{{
	"id":              productID,
	"title":           productTitle,
	"descriptionHtml": productDescription,
}}

var (
	indexTemplate = mustParseTemplate("index.html", "templates/index.html")
	productPage   = mustParseTemplate("productpage.html", "templates/productpage.html")
)

// mustParseTemplate parses the named file from the embedded assets. ParseFS
// assigns the template's body to a template named after the file's base name.
func mustParseTemplate(name, path string) *template.Template {
	return template.Must(template.New(name).Funcs(templateFuncs).ParseFS(webAssets, path))
}

// templateFuncs provides helpers for the templates.
var templateFuncs = template.FuncMap{
	// starRange returns a slice of length n (clamped to 0..5) so the
	// templates can render n filled stars.
	"starRange": func(n int) []int { return intRange(n, 5) },
	// emptyStarRange returns a slice of length 5-n (clamped to 0..5) so the
	// templates can render the remaining empty stars.
	"emptyStarRange": func(n int) []int { return intRange(5-n, 5) },
}

func intRange(n, max int) []int {
	if n < 0 {
		n = 0
	}
	if n > max {
		n = max
	}
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("usage: %s port", os.Args[0])
	}
	port := os.Args[1]
	if f, err := strconv.Atoi(bookinfo.EnvOr("FLOOD_FACTOR", "0")); err == nil {
		floodFactor = f
	}
	log.Printf("start at port %s (flood_factor: %d)", port, floodFactor)
	bookinfo.RunServer(port, newServer())
}

func newServer() http.Handler {
	mux := http.NewServeMux()

	// The UI. /index.html is served as an alias for /, as in the original.
	showIndex := func(w http.ResponseWriter, r *http.Request) {
		renderTemplate(w, indexTemplate, map[string]any{"ServiceTable": serviceTableHTML()})
	}
	mux.HandleFunc("GET /{$}", showIndex)
	mux.HandleFunc("GET /index.html", showIndex)

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "Product page is healthy")
	})

	mux.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		http.SetCookie(w, &http.Cookie{
			Name: "session", Value: r.FormValue("username"), Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode,
		})
		redirectToReferer(w, r)
	})

	mux.HandleFunc("GET /logout", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "", Path: "/", MaxAge: -1})
		redirectToReferer(w, r)
	})

	mux.HandleFunc("GET /productpage", handleProductPage)

	// The API:
	mux.HandleFunc("GET /api/v1/products", func(w http.ResponseWriter, r *http.Request) {
		bookinfo.WriteJSON(w, http.StatusOK, products)
	})
	mux.Handle("GET /api/v1/products/{id}", apiHandler(detailsName, getProductDetails))
	mux.Handle("GET /api/v1/products/{id}/reviews", apiHandler(reviewsName, getProductReviews))
	mux.Handle("GET /api/v1/products/{id}/ratings", apiHandler(ratingsName, getProductRatings))

	mux.HandleFunc("GET /metrics", writeMetrics)

	// Static assets (CSS, images) embedded in the binary.
	staticFS, _ := fs.Sub(webAssets, "static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	return mux
}

// apiHandler serves an upstream product endpoint as JSON, substituting the
// standard error document when the upstream request fails.
func apiHandler(app string, fetch func(r *http.Request, id string) (int, []byte)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status, body := fetch(r, r.PathValue("id"))
		if status != http.StatusOK {
			bookinfo.WriteJSON(w, status, map[string]any{"error": unavailableText(app)})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	})
}

func handleProductPage(w http.ResponseWriter, r *http.Request) {
	user := ""
	if c, err := r.Cookie("session"); err == nil {
		user = c.Value
	}
	id := strconv.Itoa(productID)

	detailsStatus, detailsBody := getProductDetails(r, id)

	if floodFactor > 0 {
		// Flood reviews with unnecessary requests to demonstrate Istio
		// rate limiting, concurrently.
		var wg sync.WaitGroup
		for i := 0; i < floodFactor; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				getProductReviews(r, id) // the response is disregarded
			}()
		}
		wg.Wait()
	}

	reviewsStatus, reviewsBody := getProductReviews(r, id)
	renderTemplate(w, productPage, buildPageData(user, detailsStatus, detailsBody, reviewsStatus, reviewsBody))
}

// Data providers
//
// productRequest performs a GET on base+"/"+id and returns the upstream
// status code and raw response body (500/empty on transport failure).
// Callers record the final outcome in the request_result counter.

func productRequest(base, id string, r *http.Request) (int, []byte) {
	res, err := sendRequest(r, base+"/"+id)
	if err != nil {
		return http.StatusInternalServerError, nil
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, body
}

func getProductDetails(r *http.Request, id string) (int, []byte) {
	status, body := productRequest(detailsBase, id, r)
	incResult(detailsName, status)
	return status, body
}

func getProductRatings(r *http.Request, id string) (int, []byte) {
	status, body := productRequest(ratingsBase, id, r)
	incResult(ratingsName, status)
	return status, body
}

// Do not remove. Bug introduced explicitly for illustration in fault
// injection task.
func getProductReviews(r *http.Request, id string) (int, []byte) {
	status, reviews := productRequest(reviewsBase, id, r)
	if status != http.StatusOK {
		status, reviews = productRequest(reviewsBase, id, r)
	}
	incResult(reviewsName, status)
	return status, reviews
}

// unavailableText is the message shown (and returned by the API) when an
// upstream request fails.
func unavailableText(app string) string {
	return fmt.Sprintf("Sorry, product %s are currently unavailable for this book.", app)
}

// sendRequest performs a GET on url, forwarding the tracing and session
// headers (see bookinfo.ForwardedHeaders).
func sendRequest(r *http.Request, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	bookinfo.Forward(r, req)
	// Like the original, end-user comes only from the session, never from
	// the caller.
	req.Header.Del("end-user")
	if c, err := r.Cookie("session"); err == nil && c.Value != "" {
		req.Header.Set("end-user", c.Value)
	}
	return httpClient.Do(req)
}

// httpClient deliberately avoids keeping connections alive so that load is
// distributed evenly across many versions of the backends (same rationale as
// the original, which did not pool).
var httpClient = &http.Client{
	Timeout:   3 * time.Second,
	Transport: &http.Transport{DisableKeepAlives: true},
}

// redirectToReferer sends a 302 back to the page the request came from
// (Flask's redirect(request.referrer)).
func redirectToReferer(w http.ResponseWriter, r *http.Request) {
	loc := r.Referer()
	if loc == "" {
		loc = "/"
	}
	http.Redirect(w, r, loc, http.StatusFound)
}

// Rendering

type reviewDisplay struct {
	Reviewer    string
	Text        string
	HasRating   bool
	Stars       int
	Color       string
	RatingError string
}

type pageData struct {
	User            string
	ProductTitle    string
	ProductDescHTML template.HTML
	DetailsStatus   int
	DetailsError    string
	Details         map[string]any
	ReviewsStatus   int
	ReviewsError    string
	Reviews         []reviewDisplay
	PodName         string
	ClusterName     string
}

func buildPageData(user string, detailsStatus int, detailsBody []byte, reviewsStatus int, reviewsBody []byte) pageData {
	p := pageData{
		User:            user,
		ProductTitle:    productTitle,
		ProductDescHTML: productDescription,
		DetailsStatus:   detailsStatus,
		ReviewsStatus:   reviewsStatus,
	}
	if detailsStatus == http.StatusOK {
		_ = json.Unmarshal(detailsBody, &p.Details)
	} else {
		p.DetailsError = unavailableText(detailsName)
	}
	if reviewsStatus != http.StatusOK {
		p.ReviewsError = unavailableText(reviewsName)
		return p
	}
	var raw struct {
		PodName     string `json:"podname"`
		ClusterName string `json:"clustername"`
		Reviews     []struct {
			Reviewer string `json:"reviewer"`
			Text     string `json:"text"`
			Rating   *struct {
				Stars int    `json:"stars"`
				Color string `json:"color"`
				Error string `json:"error"`
			} `json:"rating"`
		} `json:"reviews"`
	}
	if err := json.Unmarshal(reviewsBody, &raw); err != nil {
		p.ReviewsStatus = http.StatusInternalServerError
		p.ReviewsError = unavailableText(reviewsName)
		return p
	}
	p.PodName, p.ClusterName = raw.PodName, raw.ClusterName
	for _, rv := range raw.Reviews {
		d := reviewDisplay{Reviewer: rv.Reviewer, Text: rv.Text}
		if rv.Rating != nil {
			d.HasRating, d.Stars, d.Color, d.RatingError = true, rv.Rating.Stars, rv.Rating.Color, rv.Rating.Error
		}
		p.Reviews = append(p.Reviews, d)
	}
	return p
}

func renderTemplate(w http.ResponseWriter, tmpl *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("template error: %v", err)
	}
}

// serviceTableHTML renders the service tree shown on the index page,
// reproducing the original json2html output, including the quirk that the
// root (productpage) node is labelled with the details host.
func serviceTableHTML() template.HTML {
	host := func(base, endpoint string) string {
		return template.HTMLEscapeString(strings.TrimSuffix(base, "/"+endpoint))
	}
	const attrs = `class="table table-condensed table-bordered table-hover"`
	return template.HTML(fmt.Sprintf(
		`<table %[1]s>`+
			`<tr><th>name</th><td>%[2]s</td></tr>`+
			`<tr><th>endpoint</th><td>details</td></tr>`+
			`<tr><th>children</th><td>`+
			`<table %[1]s><thead><tr><th>name</th><th>endpoint</th><th>children</th></tr></thead><tbody>`+
			`<tr><td>%[2]s</td><td>details</td><td></td></tr>`+
			`<tr><td>%[3]s</td><td>reviews</td><td>`+
			`<table %[1]s><thead><tr><th>name</th><th>endpoint</th><th>children</th></tr></thead><tbody>`+
			`<tr><td>%[4]s</td><td>ratings</td><td></td></tr>`+
			`</tbody></table>`+
			`</td></tr>`+
			`</tbody></table>`+
			`</td></tr>`+
			`</table>`,
		attrs, host(detailsBase, detailsName), host(reviewsBase, reviewsName), host(ratingsBase, ratingsName)))
}

// request_result counter, with the same labels as the original
// prometheus_client Counter, emitted as Prometheus text at /metrics.

type counterKey struct {
	app  string
	code int
}

var (
	metricsMu    sync.Mutex
	resultCounts = map[counterKey]int64{}
)

func incResult(app string, code int) {
	metricsMu.Lock()
	resultCounts[counterKey{app, code}]++
	metricsMu.Unlock()
}

func writeMetrics(w http.ResponseWriter, r *http.Request) {
	metricsMu.Lock()
	defer metricsMu.Unlock()

	keys := make([]counterKey, 0, len(resultCounts))
	for k := range resultCounts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].app < keys[j].app || (keys[i].app == keys[j].app && keys[i].code < keys[j].code)
	})

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprintln(w, "# HELP request_result_total Results of requests")
	fmt.Fprintln(w, "# TYPE request_result_total counter")
	for _, k := range keys {
		fmt.Fprintf(w, "request_result_total{destination_app=%q,response_code=%q} %d\n", k.app, strconv.Itoa(k.code), resultCounts[k])
	}
}
