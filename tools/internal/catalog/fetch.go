package catalog

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

var fetchClient = &http.Client{Timeout: 5 * time.Minute}

const fetchAttempts = 4

// fetchBackoff is the wait before the first retry. It doubles after each one.
var fetchBackoff = time.Second

// Fetch returns the body of url. It retries connection errors, 5xx and 429
// responses with backoff. Any other status fails at once.
func Fetch(url string) ([]byte, error) {
	var err error
	for attempt, wait := 0, fetchBackoff; ; attempt, wait = attempt+1, wait*2 {
		var retry bool
		var data []byte
		if data, retry, err = fetchOnce(url); err == nil {
			return data, nil
		}
		if !retry || attempt+1 == fetchAttempts {
			return nil, err
		}
		time.Sleep(wait)
	}
}

func fetchOnce(url string) (data []byte, retry bool, err error) {
	resp, err := fetchClient.Get(url)
	if err != nil {
		return nil, true, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		retry = resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return nil, retry, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	data, err = io.ReadAll(resp.Body)
	return data, err != nil, err
}
