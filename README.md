# go-simple-http-redirector

Redirect HTTP requests to specific URI.

Redirect rules can be configured via CSV file. It suited for cases where a large number of redirect rules are required.

This process supports redirect rules that determine the domains as well as paths. If you want it to work with multiple domains, please point DNS or the reverse proxies to the same process.

## Configurations

Use CSV file.

```csv
https://before/hoge,https://after/yo
https://before/hoge?a=1,https://after/yo?z=1
```

The `before` and `after` strings should include the scheme.

See [example](./configs/examples.csv).

## About Redirect Rules

- Return a location where the set of GET queries in the request exactly matches the set of queries in the redirect rule.
  - Order of GET queries is not considered.
- A redirect rule with no GET query specified becomes the default rule for that PATH.
  - If the GET query of the request does not match any of the redirect rules, the location of the default rule is returned.
  - If the default rule is not defined, the request does not match any of the redirect rules, status 404 returned.
- Redirect rules are domain-aware.
- If there are duplicate redirect rules, the first defined rule is returned.
- Extra GET queries are removed on redirect.
