package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	_ "go.uber.org/automaxprocs"
)

var version, gitcommit string

type RedirectRule struct {
	beforeHost  string
	beforePath  string
	beforeQuery url.Values
	afterURL    string
}

type RedirectRules struct {
	// Rules is a map of source string and destination Rule.
	// A key is a combination of host and path.
	Rules map[string][]RedirectRule
}

var redirectRules *RedirectRules

func NewRedirectRules() *RedirectRules {
	return &RedirectRules{
		Rules: make(map[string][]RedirectRule),
	}
}

func (rrs *RedirectRules) AddRedirectRule(src, dest string) error {
	// parse source column
	srcURL, err := url.Parse(src)
	if err != nil {
		return fmt.Errorf("invalid source format: %s", src)
	}
	// scheme check
	if srcURL.Scheme != "http" && srcURL.Scheme != "https" {
		return fmt.Errorf("invalid source scheme: %s", src)
	}

	rule := RedirectRule{
		beforeHost:  srcURL.Host,
		beforePath:  srcURL.Path,
		beforeQuery: srcURL.Query(),
		afterURL:    dest,
	}
	key := srcURL.Host + srcURL.Path
	rrs.Rules[key] = append(rrs.Rules[key], rule)

	fmt.Printf("AddRedirectRule: %s + %v -> %s\n", key, rule.beforeQuery, rule.afterURL)
	return nil
}

func (rrs *RedirectRules) GetRedirectLocation(req *http.Request) (dest string, err error) {
	key := req.Host + req.URL.Path
	rules, ok := rrs.Rules[key]
	if !ok {
		return "", fmt.Errorf("not found: %s", key)
	}

	// find the best match from the redirect rules
	maxMatchCount := 0
	for _, p := range rules {
		if len(p.beforeQuery) == 0 && dest == "" {
			// if no queries, set this Rule as default redirect
			log.Printf("-- default dest found: '%s'\n", p.afterURL)
			dest = p.afterURL
		}

		matchCount := 0
		for reqQueryKey, reqQueryValues := range req.URL.Query() {
			// if rule does not contain request query key, skip this query check
			if !p.beforeQuery.Has(reqQueryKey) {
				continue
			}

			// check this query values
			for _, v := range reqQueryValues {
				if p.beforeQuery.Get(reqQueryKey) == v {
					matchCount++
					log.Printf("-- query match: '%s: %s (matched: %d)'\n", reqQueryKey, v, matchCount)
					break
				}
			}
		}

		if matchCount == 0 {
			continue
		}
		if matchCount < len(p.beforeQuery) {
			log.Printf("-- this rule is ignored because it does not meet the number of queries required: '%s%s?%v' -> '%s'\n", p.beforeHost, p.beforePath, p.beforeQuery, p.afterURL)
			continue
		}
		if matchCount == maxMatchCount {
			log.Printf("-- this rule is ignored because there is rule with higher priority: '%s%s?%v' -> '%s'\n", p.beforeHost, p.beforePath, p.beforeQuery, p.afterURL)
		} else if matchCount > maxMatchCount {
			maxMatchCount = matchCount
			dest = p.afterURL
			log.Printf("-- update dest: '%s' (matched total: %d)\n", dest, matchCount)
		}
	}

	if dest == "" {
		return "", fmt.Errorf("not found: %s", key)
	}
	return dest, nil
}

func parseCSV(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

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

		// if records has more than 2 columns, ignore after the 3rd column
		if err := redirectRules.AddRedirectRule(records[0], records[1]); err != nil {
			log.Printf("invalid format: %s (line:%d)\n", err.Error(), i)
			continue
		}
		validLineNum++
	}

	log.Printf("%d configurations loaded from CSV", validLineNum)
	return nil
}

func redirect(w http.ResponseWriter, r *http.Request) {
	log.Printf("request: '%s%s?%s'\n", r.Host, r.URL.Path, r.URL.RawQuery)
	// rebuild request URL
	dest, err := redirectRules.GetRedirectLocation(r)
	if err != nil {
		http.NotFound(w, r)
		log.Printf("not found: %s %s %s", r.URL.Host, r.URL.Path, r.URL.RawQuery)
		return
	}

	http.Redirect(w, r, dest, 301)
	log.Printf("redirected: '%s%s?%s' -> '%s'", r.Host, r.URL.Path, r.URL.RawQuery, dest)
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

	redirectRules = NewRedirectRules()
	if err := parseCSV(*path); err != nil {
		log.Fatal("parseCSV: ", err)
	}
	log.Printf("%d redirect parameters applied.", len(redirectRules.Rules))

	http.HandleFunc("/", redirect)
	log.Printf("Listening on :%d\n", *port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
