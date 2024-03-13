# go-simple-http-redirector

[![Test](https://github.com/livesense-inc/go-simple-http-redirector/actions/workflows/test.yml/badge.svg)](https://github.com/livesense-inc/go-simple-http-redirector/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/livesense-inc/go-simple-http-redirector)](https://goreportcard.com/report/github.com/livesense-inc/go-simple-http-redirector)


HTTPリクエストを特定のURIにリダイレクトします。

リダイレクトルールはCSVファイルで設定可能で、大量のリダイレクト設定が必要なケースに向いています。

このプロセスはパスだけでなく、ドメインを区別したリダイレクトルールに対応しています。もし複数のドメインに対応させたい場合は、DNSやリバースプロキシを同じプロセスに向けるように設定してください。

## 設定

CSVファイルを利用します。

```csv
https://before/hoge,https://after/yo
https://before/hoge?a=1,https://after/yo?z=1
```

上記の `before` と `after` の文字列にはスキームを含めるようにしてください。

[設定例](./configs/examples.csv)をご覧ください。

## 起動方法

設定ファイルとなるCSVファイルを用意してください。

以下のように起動します。

```bash
redirector -csv CSVFILE
```

その他のオプションについてはヘルプを見て下さい。

```bash
$ redirector -help

Usage of redirector:
  -csv string
    	Redirect list CSV file path
  -loglevel string
    	Log level (debug, info, warn, error) (default "info")
  -port int
    	Listening TCP port number (default 8080)
  -version
    	Show version
```

## リダイレクトルールについて

- リクエストのGETクエリのセットが、リダイレクトルールのクエリのセットと完全マッチするルールを返答
  - GETクエリの順序は考慮しない
- GETクエリの指定なしのリダイレクトルールは、そのPATHにおけるデフォルトルールとなる
  - リクエストのGETクエリがリダイレクトルールとマッチしなければ、デフォルトルールを返答
  - デフォルト設定すら定義されてない場合は、status 404を返答
- リダイレクトルールはドメインを意識して動作
- リダイレクトルールに重複がある場合は、先に定義されたルールを返答
- 余分なGETクエリはリダイレクト時に除去される
