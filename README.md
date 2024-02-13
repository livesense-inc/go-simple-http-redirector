# go-simple-http-redirector

Redirect HTTP requests to specific URI.
This process supports multiple domains. If you want it to work with multiple domains, please point DNS or the reverse proxies to the same process.

## Configurations

Use CSV file.

```csv
before,after
```

The `before` and `after` strings should include the scheme.

See [example](./configs/examples.csv).
