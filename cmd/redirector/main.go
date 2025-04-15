package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"

	"go.uber.org/automaxprocs/maxprocs"
)

var version, gitcommit string
var logger *slog.Logger

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

	logger.Debug(fmt.Sprintf("Add RedirectRule: %s + %v -> %s", key, rule.beforeQuery, rule.afterURL))
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
			logger.Debug(fmt.Sprintf("-- default dest found: '%s'", p.afterURL))
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
					logger.Debug(fmt.Sprintf("-- query match: '%s: %s (matched: %d)'", reqQueryKey, v, matchCount))
					break
				}
			}
		}

		if matchCount == 0 {
			continue
		}
		if matchCount < len(p.beforeQuery) {
			logger.Debug(fmt.Sprintf("-- this rule is ignored because it does not meet the number of queries required: '%s%s?%v' -> '%s'", p.beforeHost, p.beforePath, p.beforeQuery, p.afterURL))
			continue
		}
		if matchCount == maxMatchCount {
			logger.Debug(fmt.Sprintf("-- this rule is ignored because there is rule with higher priority: '%s%s?%v' -> '%s'", p.beforeHost, p.beforePath, p.beforeQuery, p.afterURL))
		} else if matchCount > maxMatchCount {
			maxMatchCount = matchCount
			dest = p.afterURL
			logger.Debug(fmt.Sprintf("-- update dest: '%s' (matched total: %d)", dest, matchCount))
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
	defer func() {
		if cerr := f.Close(); cerr != nil {
			logger.Error(fmt.Sprintf("Error closing file: %s, error: %v", path, cerr))
		}
	}()

	validLineNum := 0
	r := csv.NewReader(f)
	for i := 1; ; i++ {
		records, err := r.Read()
		if err == io.EOF {
			break
		}
		if len(records) < 2 {
			logger.Error(fmt.Sprintf("invalid row format: %v(line:%d)", records, i))
			continue
		}

		// if records has more than 2 columns, ignore after the 3rd column
		if err := redirectRules.AddRedirectRule(records[0], records[1]); err != nil {
			logger.Error(fmt.Sprintf("invalid format: %s (line:%d)", err.Error(), i))
			continue
		}
		validLineNum++
	}

	logger.Info(fmt.Sprintf("%d configurations loaded from CSV", validLineNum))
	return nil
}

func redirect(w http.ResponseWriter, r *http.Request) {
	logger.Debug(fmt.Sprintf("request: '%s%s?%s'", r.Host, r.URL.Path, r.URL.RawQuery))

	var logMsg string
	var logStatusCode int
	var logURL string
	if r.URL.RawQuery == "" {
		logURL = r.URL.Path
	} else {
		logURL = fmt.Sprintf("%s?%s", r.URL.Path, r.URL.RawQuery)
	}

	dest, err := redirectRules.GetRedirectLocation(r)
	if err != nil || dest == "" {
		http.NotFound(w, r)
		logMsg = "not found"
		logStatusCode = http.StatusNotFound
	} else {
		http.Redirect(w, r, dest, http.StatusMovedPermanently)
		logMsg = "redirected"
		logStatusCode = http.StatusMovedPermanently
	}

	logger.Info(
		logMsg,
		"method", r.Method,
		"status_code", logStatusCode,
		"host", r.Host,
		"url", logURL,
		"location", dest,
		"remote_addr", r.RemoteAddr,
		"x_forwarded_for", r.Header.Get("X-Forwarded-For"),
		"x_forwarded_proto", r.Header.Get("X-Forwarded-Proto"),
		"referer", r.Referer(),
		"user_agent", r.UserAgent(),
	)
}

func health(w http.ResponseWriter, r *http.Request) {
	// return 200 OK
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		logger.Error(fmt.Sprintf("Response to client returns error: %s", err))
	}

	var logURL string
	if r.URL.RawQuery == "" {
		logURL = r.URL.Path
	} else {
		logURL = fmt.Sprintf("%s?%s", r.URL.Path, r.URL.RawQuery)
	}
	logger.Debug(
		"health check OK",
		"method", r.Method,
		"status_code", http.StatusOK,
		"host", r.Host,
		"url", logURL,
		"location", "",
		"remote_addr", r.RemoteAddr,
		"x_forwarded_for", r.Header.Get("X-Forwarded-For"),
		"x_forwarded_proto", r.Header.Get("X-Forwarded-Proto"),
		"referer", r.Referer(),
		"user_agent", r.UserAgent(),
	)
}

func main() {
	logLevel := new(slog.LevelVar)
	ops := slog.HandlerOptions{
		Level: logLevel,
	}
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &ops))

	// Set GOMAXPROCS to the number of CPUs available
	if _, err := maxprocs.Set(maxprocs.Logger(func(format string, v ...interface{}) {
		logger.Info(fmt.Sprintf(format, v...))
	})); err != nil {
		logger.Warn("Setting GOMAXPROCS failed", "error", err)
	}

	versionFlag := flag.Bool("version", false, "Show version")
	csvPath := flag.String("csv", "", "Redirect list CSV file path")
	port := flag.Int("port", 8080, "Listening TCP port number")
	logLevelString := flag.String("loglevel", "info", "Log level (debug, info, warn, error)")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("%s (rev:%s)\n", version, gitcommit)
		os.Exit(0)
	}

	switch *logLevelString {
	case "debug":
		logLevel.Set(slog.LevelDebug)
	case "info":
		logLevel.Set(slog.LevelInfo)
	case "warn":
		logLevel.Set(slog.LevelWarn)
	case "error":
		logLevel.Set(slog.LevelError)
	default:
		logger.Error(fmt.Sprintf("invalid log level: %s", *logLevelString))
		flag.Usage()
		os.Exit(1)
	}
	logger.Info(fmt.Sprintf("redirector version: %s (rev:%s)", version, gitcommit))

	redirectRules = NewRedirectRules()
	if *csvPath == "" {
		logger.Error("csv option is required")
		flag.Usage()
		os.Exit(1)
	}
	if err := parseCSV(*csvPath); err != nil {
		logger.Error(fmt.Sprintf("parseCSV: %s", err))
		os.Exit(1)
	}
	logger.Info(fmt.Sprintf("%d redirect parameters applied.", len(redirectRules.Rules)))

	http.HandleFunc("/health", health)
	http.HandleFunc("/", redirect)

	logger.Info(fmt.Sprintf("Listening on :%d", *port))
	err := http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)
	if err != nil {
		logger.Error(fmt.Sprintf("ListenAndServe: %s", err))
		os.Exit(1)
	}
}
