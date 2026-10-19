// Command webfetch fetches web pages from a cameo script. cameo does no
// I/O, so the host gives the script a Web module whose fetch starts the
// request on a goroutine and returns a promise at once. The script starts
// every fetch before it awaits any, so the requests run at the same time,
// and await returns each page or raises Web::FetchError.
//
//	go run ./examples/webfetch https://example.com https://example.org/missing
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/cameo-lang/cameo/src/cameo"
)

const script = `
include Cameo

def fetch_all(urls)
  promises = urls.map { |url| Web.fetch(url) }   # every request starts now
  urls.zip(promises).map do |url, promise|
    "#{url}: #{(await promise).size} bytes"
  rescue Web::FetchError => e
    "#{url}: #{e.message}"
  end
end
`

// fetchAll runs the script's fetch_all over urls with client.
func fetchAll(client *http.Client, urls []string) ([]any, error) {
	prog, err := cameo.Load("webfetch.cameo", script, os.Stdout, os.Stderr)
	if err != nil {
		return nil, err
	}
	web := prog.Module("Web")
	fetchError := web.Exception("FetchError")
	web.Def("fetch", func(args []any, kwargs map[string]any) (any, error) {
		url, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("Web.fetch takes a URL, not %v", args[0])
		}
		promise := prog.Promise()
		go func() {
			resp, err := client.Get(url)
			if err != nil {
				promise.Fail(fetchError.New("%v", err))
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				promise.Fail(fetchError.New("%s", resp.Status))
				return
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				promise.Fail(fetchError.New("%v", err))
				return
			}
			promise.Resolve(string(body))
		}()
		return promise, nil
	})
	lines, err := prog.Call(nil, "fetch_all", urls)
	if err != nil {
		return nil, err
	}
	return lines.([]any), nil
}

func main() {
	lines, err := fetchAll(http.DefaultClient, os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, line := range lines {
		fmt.Println(line)
	}
}
