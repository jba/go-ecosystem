// Package proxy supports queries on the Go module proxy and index.
// It always sets the Disable-Module-Fetch header to true,
// and it always throttles requests to a configured QPS.

package proxy

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"jba/work/lib/errs"
	"jba/work/lib/httputil"
	"jba/work/lib/jiter"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/mod/module"
	"golang.org/x/time/rate"
)

var URL string = "https://proxy.golang.org"

func init() {
	if u := os.Getenv("GOPROXY"); u != "" {
		URL = u
	}
}

const (
	defaultMaxQPS = 50
	defaultBurst  = 10
)

var (
	ncalls  atomic.Int64
	mu      sync.Mutex
	maxQPS  int
	limiter *rate.Limiter
	start   time.Time
)

func SetMaxQPS(qps int) {
	mu.Lock()
	defer mu.Unlock()
	maxQPS = qps
	limiter = rate.NewLimiter(rate.Every(time.Second/time.Duration(qps)), defaultBurst)
}

func init() {
	SetMaxQPS(defaultMaxQPS)
}

var Debug = false

func QPS() float64 {
	mu.Lock()
	defer mu.Unlock()
	return float64(ncalls.Load()) / time.Since(start).Seconds()
}

func ResetQPS() {
	ncalls.Store(0)
	mu.Lock()
	start = time.Time{}
	mu.Unlock()
}

////////////////////////////////////////////////////////////////

type InfoEntry struct {
	Version string
	Time    string
	Origin  Origin
}

type Origin struct {
	VCS  string
	URL  string
	Ref  string
	Hash string
}

func Info(ctx context.Context, path, version string) (_ *InfoEntry, err error) {
	defer errs.Wrap(&err, "proxy.Info(%q, %q)", path, version)
	url, err := proxyVersionURL(path, version, ".info")
	if err != nil {
		return nil, err
	}
	return fetchInfoEntry(ctx, url)
}

func Latest(ctx context.Context, path string) (_ string, err error) {
	defer errs.Wrap(&err, "proxy.Latest(%q)", path)
	url, err := proxyPathURL(path)
	if err != nil {
		return "", err
	}
	url += "/@latest"
	entry, err := fetchInfoEntry(ctx, url)
	if err != nil {
		return "", err
	}
	return entry.Version, nil
}

func fetchInfoEntry(ctx context.Context, url string) (*InfoEntry, error) {
	data, err := fetchCached(ctx, url)
	if err != nil {
		return nil, err
	}
	var res InfoEntry
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func Mod(ctx context.Context, path, version string) (_ []byte, err error) {
	defer errs.Wrap(&err, "proxy.Mod(%q, %q)", path, version)
	url, err := proxyVersionURL(path, version, ".mod")
	if err != nil {
		return nil, err
	}
	return fetchCached(ctx, url)
}

func List(ctx context.Context, path string) (_ []string, err error) {
	defer errs.Wrap(&err, "proxy.List(%q)", path)
	url, err := proxyPathURL(path)
	if err != nil {
		return nil, err
	}
	url += "/@v/list"
	data, err := fetchCached(ctx, url)
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(data)), nil
}

func Zip(ctx context.Context, path, version string) (_ *zip.Reader, err error) {
	defer errs.Wrap(&err, "proxy.Zip(%q, %q)", path, version)

	data, err := ZipData(ctx, path, version)
	if err != nil {
		return nil, err
	}
	return zip.NewReader(bytes.NewReader(data), int64(len(data)))
}

func ZipData(ctx context.Context, path, version string) ([]byte, error) {
	url, err := proxyVersionURL(path, version, ".zip")
	if err != nil {
		return nil, err
	}
	return fetch(ctx, url)
}

func proxyPathURL(modPath string) (string, error) {
	epath, err := module.EscapePath(modPath)
	if err != nil {
		return "", err
	}
	return URL + "/" + epath, nil
}

func proxyVersionURL(modPath, version, suffix string) (string, error) {
	u, err := proxyPathURL(modPath)
	if err != nil {
		return "", err
	}
	v, err := module.EscapeVersion(version)
	if err != nil {
		return "", err
	}
	return u + "/@v/" + v + suffix, nil
}

func fetch(ctx context.Context, url string) ([]byte, error) {
	mu.Lock()
	lim := limiter
	if start.IsZero() {
		start = time.Now()
	}
	mu.Unlock()
	if err := lim.Wait(ctx); err != nil {
		return nil, err
	}
	if Debug {
		log.Printf("proxy: Get %s", url)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	// Setting this header to true prevents the proxy from fetching uncached
	// modules.
	req.Header.Set("Disable-Module-Fetch", "true")
	req.Header.Set("User-Agent", "jba work")
	ncalls.Add(1)
	return httputil.DoReadBody(req)
}

var cacheEnabled = false
var cacheDir = filepath.Join(os.TempDir(), "goproxy-cache")

func init() {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		log.Fatal(err)
	}
}

var cacheTTL = 24 * time.Hour

func fetchCached(ctx context.Context, surl string) ([]byte, error) {
	filename := filepath.Join(cacheDir, url.PathEscape(surl))
	if cacheEnabled {
		finfo, fetchErr := os.Stat(filename)
		if fetchErr == nil && time.Since(finfo.ModTime()) < cacheTTL {
			data, err := os.ReadFile(filename)
			if err != nil {
				return nil, err
			}
			if len(data) < 4 {
				return nil, errors.New("contents too short")
			}
			status, err := strconv.Atoi(string(data[:3]))
			if err != nil {
				return nil, err
			}
			if status == 200 {
				return data[4:], nil
			}
			return nil, &httputil.HTTPError{Status: status}
		}
	}
	var fileBytes []byte
	bytes, fetchErr := fetch(ctx, surl)
	if fetchErr != nil {
		var herr *httputil.HTTPError
		if errors.As(fetchErr, &herr) {
			fileBytes = []byte(fmt.Sprintf("%d\n", herr.Status))
		} else {
			return nil, fetchErr
		}
	} else {
		fileBytes = append([]byte("200\n"), bytes...)
	}
	if cacheEnabled {
		if err := os.WriteFile(filename, fileBytes, 0o644); err != nil {
			return nil, err
		}
	}
	return bytes, fetchErr
}

////////////////////////////////////////////////////////////////

type IndexEntry struct {
	Path      string
	Version   string
	Timestamp string
}

// ReadIndex reads entries from index.golang.org.
//
// since should either be the empty string or a value returned in the
// Timestamp field of a previously read IndexEntry.
//
// The limit is passed on to the index unless it is zero.
func ReadIndex(ctx context.Context, since string, limit int) ([]*IndexEntry, error) {
	mu.Lock()
	lim := limiter
	if start.IsZero() {
		start = time.Now()
	}
	mu.Unlock()
	if err := lim.Wait(ctx); err != nil {
		return nil, err
	}
	url := "https://index.golang.org/index"
	var params []string
	if since != "" {
		params = append(params, "since="+since)
	}
	if limit > 0 {
		params = append(params, fmt.Sprintf("limit=%d", limit))
	}
	if len(params) > 0 {
		url += "?" + strings.Join(params, "&")
	}
	if Debug {
		log.Printf("index: GET %s", url)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	ncalls.Add(1)
	body, err := httputil.DoReadBody(req)
	if err != nil {
		return nil, err
	}
	var entries []*IndexEntry
	dec := json.NewDecoder(bytes.NewReader(body))
	// The module index returns a stream of JSON objects formatted with newline
	// as the delimiter.
	for dec.More() {
		var e IndexEntry
		if err := dec.Decode(&e); err != nil {
			return nil, fmt.Errorf("decoding JSON: %v", err)
		}
		entries = append(entries, &e)
	}
	return entries, nil
}

// IndexEntries returns an iterator over index entries since the given time, which should be the
// empty string or a value from an [IndexEntry].
// It never returns the same entry twice, even if they have the same timestamp.
func IndexEntries(ctx context.Context, since string) (iter.Seq[*IndexEntry], func() error) {
	var es jiter.ErrorState
	return func(yield func(*IndexEntry) bool) {
		defer es.Done()
		prevs := map[IndexEntry]bool{} // previously seen entries at since.
		for {
			entries, err := ReadIndex(ctx, since, 0)
			if err != nil {
				es.Set(err)
				return
			}
			n := 0
			for _, e := range entries {
				if prevs[*e] {
					continue
				}
				if !yield(e) {
					return
				}
				n++
			}
			if n == 0 {
				return
			}
			since = entries[len(entries)-1].Timestamp
			// Remember entries we've returned at this timestamp so we don't repeat them.
			clear(prevs)
			for i := len(entries) - 1; i >= 0; i-- {
				if entries[i].Timestamp != since {
					break
				}
				prevs[*entries[i]] = true
			}
		}
	}, es.Func()
}
