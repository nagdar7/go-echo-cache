package cache

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coocood/freecache"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestCache(t *testing.T) {
	client := getCachedServer(t, nil)
	defer client.Close()

	resp, err := http.Get(client.URL)
	assert.NoError(t, err)
	assertRequest(t, resp, http.StatusOK, "test_1")

	resp, err = http.Get(client.URL)
	assert.NoError(t, err)
	assertRequest(t, resp, http.StatusOK, "test_1")

	resp, err = http.Get(client.URL + "/foo")
	assert.NoError(t, err)
	assertRequest(t, resp, http.StatusOK, "test_2")

	resp, err = http.Get(client.URL + "/?foo=42&bar=84")
	assert.NoError(t, err)
	assertRequest(t, resp, http.StatusOK, "test_3")
}

func TestCache_ConfigTTL(t *testing.T) {
	client := getCachedServer(t, &Config{TTL: time.Second})
	defer client.Close()

	resp, err := http.Get(client.URL)
	assert.NoError(t, err)
	assertRequest(t, resp, http.StatusOK, "test_1")

	resp, err = http.Get(client.URL)
	assert.NoError(t, err)
	assertRequest(t, resp, http.StatusOK, "test_1")

	time.Sleep(time.Millisecond * 1001)
	resp, err = http.Get(client.URL)
	assert.NoError(t, err)
	assertRequest(t, resp, http.StatusOK, "test_2")
}

func TestCache_ConfigRefresh(t *testing.T) {
	var i int
	client := getCachedServer(t, &Config{Refresh: func(r *http.Request) bool {
		i++
		return i == 2
	}})

	defer client.Close()

	resp, err := http.Get(client.URL)
	assert.NoError(t, err)
	assertRequest(t, resp, http.StatusOK, "test_1")

	resp, err = http.Get(client.URL)
	assert.NoError(t, err)
	assertRequest(t, resp, http.StatusOK, "test_2")

	resp, err = http.Get(client.URL)
	assert.NoError(t, err)
	assertRequest(t, resp, http.StatusOK, "test_2")
}

func TestCache_ConfigCache(t *testing.T) {
	var i int
	client := getCachedServer(t, &Config{Cache: func(r *http.Request) bool {
		i++
		return i != 1
	}})

	defer client.Close()

	resp, err := http.Get(client.URL)
	assert.NoError(t, err)
	assertRequest(t, resp, http.StatusOK, "test_1")

	resp, err = http.Get(client.URL)
	assert.NoError(t, err)
	assertRequest(t, resp, http.StatusOK, "test_2")

	resp, err = http.Get(client.URL)
	assert.NoError(t, err)
	assertRequest(t, resp, http.StatusOK, "test_2")
}

func TestCache_ConfigIgnoreQuery(t *testing.T) {
	client := getCachedServer(t, &Config{IgnoreQuery: true})
	defer client.Close()

	resp, err := http.Get(client.URL)
	assert.NoError(t, err)
	assertRequest(t, resp, http.StatusOK, "test_1")

	resp, err = http.Get(client.URL + "/?foo=42&bar=84")
	assert.NoError(t, err)
	assertRequest(t, resp, http.StatusOK, "test_1")
}

func TestCache_Methods(t *testing.T) {
	client := getCachedServer(t, &Config{Methods: []string{"GET", "POST"}})
	defer client.Close()

	resp, err := http.Post(client.URL, "", strings.NewReader("foo"))
	assert.NoError(t, err)
	assertRequest(t, resp, http.StatusOK, "test_1")

	resp, err = http.Get(client.URL)
	assert.NoError(t, err)
	assertRequest(t, resp, http.StatusOK, "test_2")

	resp, err = http.Get(client.URL)
	assert.NoError(t, err)
	assertRequest(t, resp, http.StatusOK, "test_2")

	r, _ := http.NewRequest("PUT", client.URL, nil)
	resp, err = http.DefaultClient.Do(r)
	assert.NoError(t, err)
	assertRequest(t, resp, http.StatusOK, "test_3")

	resp, err = http.DefaultClient.Do(r)
	assert.NoError(t, err)
	assertRequest(t, resp, http.StatusOK, "test_4")
}

func TestCache_StatusCode(t *testing.T) {
	client := getCachedServerWithCode(t, &Config{StatusCode: []int{200, 404}}, http.StatusInternalServerError)
	defer client.Close()

	resp, err := http.Get(client.URL)
	assert.NoError(t, err)
	assertRequest(t, resp, http.StatusOK, "test_1")

	resp, err = http.Get(client.URL)
	assert.NoError(t, err)
	assertRequest(t, resp, http.StatusOK, "test_2")
}

func getCachedServer(t *testing.T, cfg *Config) *httptest.Server {
	return getCachedServerWithCode(t, cfg, http.StatusOK)
}

type frecacheWrapper struct {
	*freecache.Cache
}

func (f frecacheWrapper) GetNotFoundErr() error {
	return freecache.ErrNotFound
}

func getCachedServerWithCode(t *testing.T, cfg *Config, status int) *httptest.Server {
	e := echo.New()

	var i int
	h := New(cfg, frecacheWrapper{freecache.NewCache(42 * 1024 * 1024)})(func(c echo.Context) error {
		i++
		return c.String(status, fmt.Sprintf("test_%d", i))
	})

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := e.NewContext(r, w)
		assert.NoError(t, h(c))
	}))
}

func assertRequest(t *testing.T, resp *http.Response, status int, content string) {
	if status != http.StatusOK {
		return
	}

	b, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, content, string(b))
	resp.Body.Close()
}
