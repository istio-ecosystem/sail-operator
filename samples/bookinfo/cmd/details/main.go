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

// The details service returns book details.
//
// This is a pure-Go re-implementation of the original Ruby/WEBrick details
// service from samples/bookinfo. It has no external dependencies: the book
// data is hard-coded. When ENABLE_EXTERNAL_BOOK_SERVICE=true, details are
// fetched from the Google Books API using the standard library HTTP client
// (the DO_NOT_ENCRYPT env var selects HTTP instead of HTTPS, as in the
// original).
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

// externalISBN is the ISBN of an edition of The Comedy of Errors that has
// Shakespeare as the single author.
const externalISBN = "0486424618"

// bookDetails is the JSON document returned for a book.
type bookDetails struct {
	ID        int    `json:"id"`
	Author    string `json:"author"`
	Year      any    `json:"year"`
	Type      string `json:"type"`
	Pages     int    `json:"pages"`
	Publisher string `json:"publisher"`
	Language  string `json:"language"`
	ISBN10    string `json:"ISBN-10"`
	ISBN13    string `json:"ISBN-13"`
}

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("usage: %s port", os.Args[0])
	}
	port := os.Args[1]
	log.Printf("start at port %s", port)
	bookinfo.RunServer(port, newServer())
}

func newServer() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		bookinfo.WriteJSON(w, http.StatusOK, map[string]string{"status": "Details is healthy"})
	})

	mux.HandleFunc("GET /details/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			bookinfo.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "please provide numeric product id"})
			return
		}
		details, err := getBookDetails(id, r)
		if err != nil {
			bookinfo.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		bookinfo.WriteJSON(w, http.StatusOK, details)
	})

	return mux
}

// getBookDetails returns the details for the book with the given id.
func getBookDetails(id int, r *http.Request) (any, error) {
	if os.Getenv("ENABLE_EXTERNAL_BOOK_SERVICE") == "true" {
		return fetchDetailsFromExternalService(id, r)
	}

	// TODO: provide details on different books.
	return bookDetails{
		ID:        id,
		Author:    "William Shakespeare",
		Year:      1595,
		Type:      "paperback",
		Pages:     200,
		Publisher: "PublisherA",
		Language:  "English",
		ISBN10:    "1234567890",
		ISBN13:    "123-1234567890",
	}, nil
}

// googleBook is the subset of the Google Books API response we use.
type googleBook struct {
	Authors             []string `json:"authors"`
	PublishedDate       string   `json:"publishedDate"`
	PrintType           string   `json:"printType"`
	PageCount           int      `json:"pageCount"`
	Publisher           string   `json:"publisher"`
	Language            string   `json:"language"`
	IndustryIdentifiers []struct {
		Type       string `json:"type"`
		Identifier string `json:"identifier"`
	} `json:"industryIdentifiers"`
}

// fetchDetailsFromExternalService queries the Google Books API for the book
// with the hard-coded externalISBN and formats the response the same way the
// original service did.
//
// DO_NOT_ENCRYPT is used to configure the details service to use either HTTP
// (true) or HTTPS (false, default) when calling the external service to
// retrieve the book information.
func fetchDetailsFromExternalService(id int, r *http.Request) (any, error) {
	scheme, port := "https", 443
	if os.Getenv("DO_NOT_ENCRYPT") == "true" {
		scheme, port = "http", 80
	}
	url := fmt.Sprintf("%s://www.googleapis.com:%d/books/v1/volumes?q=isbn:%s", scheme, port, externalISBN)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	bookinfo.ForwardTraceHeaders(r, req)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("external book service returned %d", resp.StatusCode)
	}

	var payload struct {
		Items []struct {
			VolumeInfo googleBook `json:"volumeInfo"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("could not decode response from external book service: %v", err)
	}
	if len(payload.Items) == 0 {
		return nil, fmt.Errorf("external book service returned no results")
	}
	book := payload.Items[0].VolumeInfo

	language := "unknown"
	if book.Language == "en" {
		language = "English"
	}
	typ := "unknown"
	if book.PrintType == "BOOK" {
		typ = "paperback"
	}

	return bookDetails{
		ID:        id,
		Author:    firstOr(book.Authors, "unknown"),
		Year:      book.PublishedDate,
		Type:      typ,
		Pages:     book.PageCount,
		Publisher: book.Publisher,
		Language:  language,
		ISBN10:    getISBN(book, "ISBN_10"),
		ISBN13:    getISBN(book, "ISBN_13"),
	}, nil
}

func firstOr(s []string, def string) string {
	if len(s) == 0 {
		return def
	}
	return s[0]
}

func getISBN(book googleBook, isbnType string) string {
	for _, identifier := range book.IndustryIdentifiers {
		if identifier.Type == isbnType {
			return identifier.Identifier
		}
	}
	return ""
}
