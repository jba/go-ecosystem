// Package proxy supports queries on the Go module proxy.
// It only accesses cached modules, and throttles requests to a configured
// QPS.
package proxy

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

	"github.com/jba/go-ecosystem/internal/errs"
	"github.com/jba/go-ecosystem/internal/httputil"
)

const (
	proxyURL      = "https://proxy.golang.org/cached-only"
	defaultMaxQPS = 100
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

func Info(ctx context.Context, path, version string) (*InfoEntry, error) {
	return info(ctx, http.DefaultClient, path, version)
}

// info is the implementation of [Info], with an explicit HTTP client so tests
// can substitute a record/replay client.
func info(ctx context.Context, c *http.Client, path, version string) (_ *InfoEntry, err error) {
	debugf("Info %s %s", path, version)
	defer errs.Wrap(&err, "proxy.Info(%q, %q)", path, version)
	url, err := proxyVersionURL(path, version, ".info")
	if err != nil {
		return nil, err
	}
	return fetchInfoEntry(ctx, c, url)
}

func Latest(ctx context.Context, path string) (string, error) {
	return latest(ctx, http.DefaultClient, path)
}

// latest is the implementation of [Latest], with an explicit HTTP client so
// tests can substitute a record/replay client.
func latest(ctx context.Context, c *http.Client, path string) (_ string, err error) {
	debugf("Latest %s", path)
	defer errs.Wrap(&err, "proxy.Latest(%q)", path)
	url, err := proxyPathURL(path)
	if err != nil {
		return "", err
	}
	url += "/@latest"
	entry, err := fetchInfoEntry(ctx, c, url)
	if err != nil {
		return "", err
	}
	return entry.Version, nil
}

func fetchInfoEntry(ctx context.Context, c *http.Client, url string) (*InfoEntry, error) {
	data, err := fetchCached(ctx, c, url)
	if err != nil {
		return nil, err
	}
	var res InfoEntry
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func Mod(ctx context.Context, path, version string) ([]byte, error) {
	return mod(ctx, http.DefaultClient, path, version)
}

// mod is the implementation of [Mod], with an explicit HTTP client so tests
// can substitute a record/replay client.
func mod(ctx context.Context, c *http.Client, path, version string) (_ []byte, err error) {
	debugf("Mod %s %s", path, version)
	defer errs.Wrap(&err, "proxy.Mod(%q, %q)", path, version)
	url, err := proxyVersionURL(path, version, ".mod")
	if err != nil {
		return nil, err
	}
	return fetchCached(ctx, c, url)
}

func List(ctx context.Context, path string) ([]string, error) {
	return list(ctx, http.DefaultClient, path)
}

// list is the implementation of [List], with an explicit HTTP client so tests
// can substitute a record/replay client.
func list(ctx context.Context, c *http.Client, path string) (_ []string, err error) {
	debugf("List %s", path)
	defer errs.Wrap(&err, "proxy.List(%q)", path)
	url, err := proxyPathURL(path)
	if err != nil {
		return nil, err
	}
	url += "/@v/list"
	data, err := fetchCached(ctx, c, url)
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
	return fetch(ctx, http.DefaultClient, url)
}

func proxyPathURL(modPath string) (string, error) {
	epath, err := module.EscapePath(modPath)
	if err != nil {
		return "", err
	}
	return proxyURL + "/" + epath, nil
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

func fetch(ctx context.Context, c *http.Client, url string) ([]byte, error) {
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
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	// Setting this header to true prevents the proxy from fetching uncached
	// modules.
	req.Header.Set("Disable-Module-Fetch", "true")
	req.Header.Set("User-Agent", "jba work")
	ncalls.Add(1)
	return httputil.DoReadBody(ctx, c, req)
}

var (
	cacheEnabled = false
	cacheDir     = filepath.Join(os.TempDir(), "goproxy-cache")
)

func init() {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		log.Fatal(err)
	}
}

var cacheTTL = 24 * time.Hour

func fetchCached(ctx context.Context, c *http.Client, surl string) ([]byte, error) {
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
	bytes, fetchErr := fetch(ctx, c, surl)
	if fetchErr != nil {
		var herr *httputil.HTTPError
		if errors.As(fetchErr, &herr) {
			fileBytes = fmt.Appendf(nil, "%d\n", herr.Status)
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

func debugf(format string, args ...any) {
	if Debug {
		log.Printf("proxy: "+format, args...)
	}
}
