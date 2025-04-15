package main

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func initLogger() {
	logLevel := new(slog.LevelVar)
	ops := slog.HandlerOptions{
		Level: logLevel,
	}
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &ops))
	logLevel.Set(slog.LevelError)
}

func TestParseCSV(t *testing.T) {
	initLogger()

	// Create a temporary CSV file for testing
	tmpfile, err := os.CreateTemp("", "test.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(tmpfile.Name()); err != nil {
			t.Errorf("Error removing temporary file: %v", err)
		}
	}()

	// Write test data to the temporary CSV file
	csvData := []string{
		"https://before1/1,https://after1/dir/1",
		"https://before1/dir/2,https://after1/dir/dir/2",
		"https://before2/1,https://after2/dir/1",
	}
	for _, row := range csvData {
		_, err := tmpfile.WriteString(row + "\n")
		if err != nil {
			t.Fatal(err)
		}
	}

	// Close the temporary CSV file
	err = tmpfile.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Initialize
	redirectRules = NewRedirectRules()

	err = parseCSV(tmpfile.Name())
	if err != nil {
		t.Errorf("parseCSV returned an error: %v", err)
	}

	// Check the rules map
	expected := map[string]string{
		"before1/1":     "https://after1/dir/1",
		"before1/dir/2": "https://after1/dir/dir/2",
		"before2/1":     "https://after2/dir/1",
	}
	for k, v := range expected {
		if redirectRules.Rules[k][0].afterURL != v {
			t.Errorf("rule[%q] = %v, want %v", k, redirectRules.Rules[k], v)
		}
	}
}

func TestRedirect(t *testing.T) {
	initLogger()

	redirectRules = NewRedirectRules()
	if err := parseCSV("../../configs/examples.csv"); err != nil {
		t.Fatal("parseCSV: ", err)
	}

	// Test for defined rules
	expected := map[string]string{
		"https://before/hoge":             "https://after/yo",
		"https://before/hoge?a=1":         "https://after/yo?z=1",
		"https://before/hoge?a=2":         "https://after/yo?z=2",
		"https://before/hoge?b=2":         "https://after/yo?z=2",
		"https://before/hoge?c=3":         "https://after/yo", // no match returns default
		"https://before/hoge?a=1&b=2":     "https://after/yo?z=3",
		"https://before/hoge?b=2&a=1":     "https://after/yo?z=3", // order of query parameters does not matter
		"https://before/hoge?c=3&b=2&a=1": "https://after/yo?z=3", // no match query is ignored
		"https://before/hoge?a=3&b=4":     "https://after/yo?z=7",
		"https://before/hoge?a=3":         "https://after/yo", // no match returns default
		"http://before/fuga":              "https://after/dir/hey",
		"http://before/fuga?a=1":          "https://after/dir/hey",
		"https://anotherdomain/hoge":      "https://another/yo",
	}
	for requestURL, expectedLocation := range expected {
		req, err := http.NewRequest("GET", requestURL, nil)
		if err != nil {
			t.Fatal(err)
		}
		rr := httptest.NewRecorder()
		redirect(rr, req)

		// Check the response status code
		if rr.Code != http.StatusMovedPermanently {
			t.Errorf("expected status code %d, got %d", http.StatusMovedPermanently, rr.Code)
		}
		// Check the response header "Location"
		if location := rr.Header().Get("Location"); location != expectedLocation {
			t.Errorf("%s : expected location %s, got %s", requestURL, expectedLocation, location)
		}
	}

	// Test for not defined rules
	expected = map[string]string{
		"https://before/notdefined": "",
		"https://notdefined/hoge":   "",
		"http://before/piyo":        "", // default rule is not defined for this path
		"http://before/piyo?z=1":    "", // default rule is not defined for this path

	}
	for requestURL := range expected {
		req, err := http.NewRequest("GET", requestURL, nil)
		if err != nil {
			t.Fatal(err)
		}
		rr := httptest.NewRecorder()
		redirect(rr, req)

		// Check the response status code
		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status code %d, got %d", http.StatusNotFound, rr.Code)
		}
	}
}

func TestHealth(t *testing.T) {
	initLogger()

	expected := map[string]int{
		"https://before/health":     http.StatusOK,
		"https://notdefined/health": http.StatusOK,
	}
	for requestURL, expectedStatus := range expected {
		req, err := http.NewRequest("GET", requestURL, nil)
		if err != nil {
			t.Fatal(err)
		}
		rr := httptest.NewRecorder()
		health(rr, req)

		// Check the response status code
		if rr.Code != expectedStatus {
			t.Errorf("expected status code %d, got %d", expectedStatus, rr.Code)
		}
	}
}
