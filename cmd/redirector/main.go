package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"

	_ "go.uber.org/automaxprocs"
)

var version, gitcommit string

type redirectPattern struct {
	dest string
}

// scheme string which this process supports
var reTargetScheme = regexp.MustCompile(`https?://`)

// patterns is a map of source string and destination pattern.
// source string is a combination of host and path. (= URI without scheme)
var patterns map[string]redirectPattern

func parseCSV(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// initialize patterns
	patterns = make(map[string]redirectPattern)

	validLineNum := 0
	r := csv.NewReader(f)
	for i := 1; ; i++ {
		records, err := r.Read()
		if err == io.EOF {
			break
		}
		if len(records) < 2 {
			log.Printf("invalid row format: %v(line:%d)\n", records, i)
			continue
		}
		// remove scheme from source
		src := reTargetScheme.ReplaceAllString(records[0], "")
		// scheme is not included in source, reject it
		if src == "" || src == records[0] {
			log.Printf("invalid source format: %v(line:%d)\n", records, i)
			continue
		}
		// if records has more than 2 columns, ignore after the 3rd column
		patterns[src] = redirectPattern{
			dest: records[1],
		}
		validLineNum++
	}

	log.Printf("%d configurations loaded from CSV", validLineNum)
	return nil
}

func redirect(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	path := r.URL.Path
	query := r.URL.RawQuery

	p, ok := patterns[host+path]
	if !ok {
		http.NotFound(w, r)
		log.Printf("not found: %s", path)
	}
	dest := fmt.Sprintf("%s?%s", p.dest, query)

	http.Redirect(w, r, dest, 301)
	log.Printf("redirected: %s%s -> %s", r.Host, r.URL, dest)
}

func main() {
	versionFlag := flag.Bool("version", false, "Show version")
	path := flag.String("csv", "", "Redirect list CSV file path")
	port := flag.Int("port", 8080, "Listening TCP port number")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("%s (rev:%s)\n", version, gitcommit)
		os.Exit(0)
	}
	if err := parseCSV(*path); err != nil {
		log.Fatal("parseCSV: ", err)
	}
	log.Printf("%d redirect parameters applied.", len(patterns))

	http.HandleFunc("/", redirect)
	log.Printf("Listening on :%d\n", *port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
