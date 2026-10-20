// Command webfetch fetches web pages from a cameo script, one workflow per
// page. cameo does no I/O, so the host gives the script a Web module whose
// fetch starts the request on a goroutine and returns a promise at once; the
// host settles the promise from outside the script when the response is
// in. Each workflow awaits its own fetch, so every request is in flight
// while the script waits, and await returns each page or raises
// Web::FetchError.
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

class Flow
  # One workflow: fetch the page, then measure it. await parks this
  # workflow until the host settles the promise; the others run meanwhile.
  def run(url)
    page = await Web.fetch(url)
    "#{url}: #{page.size} bytes, #{page.lines.size} lines"
  rescue Web::FetchError => e
    "#{url}: #{e.message}"
  end
end

def fetch_all(urls)
  flow = Flow.new
  promises = urls.map { |url| async flow, :run, url }   # every workflow starts now
  await promises                                        # their lines, in order
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
