package skanidlistcache

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/prebid/prebid-server/cache/skanidlistcache/cache"
	"github.com/prebid/prebid-server/cache/skanidlistcache/model"
)

// client ...
type client struct {
	httpClient  *http.Client
	cacheClient *cache.Cache
	url         string
}

// NewClient ...
func NewClient(httpClient *http.Client, url string) *client {
	cache := cache.New(1*time.Hour, 1)

	return &client{
		httpClient:  httpClient,
		cacheClient: cache,
		url:         url,
	}
}

// Load ...
func (c client) Fetch() (model.SKANIDList, error) {
	// Try to fetch from the cache first
	skanIDList, found, err := c.fetchFromCache()
	if err != nil {
		return model.SKANIDList{}, err
	}
	if found {
		return skanIDList, nil
	}

	// If not found in the cache, try to fetch from Bidder's servers
	skanIDList, _, err = c.fetchFromBidder()
	if err != nil {
		return model.SKANIDList{}, err
	}

	// Set the found skanidlist to the cache
	c.cacheClient.Set(c.url, skanIDList)

	return skanIDList, nil
}

func (c client) fetchFromCache() (model.SKANIDList, bool, error) {
	v, found := c.cacheClient.Get(c.url)
	if !found {
		return model.SKANIDList{}, false, nil
	}

	skanIDList, ok := v.(model.SKANIDList)
	if !ok {
		return model.SKANIDList{}, true, errors.New(fmt.Sprintf("error converting cache item to model.SKANIDList for: %s", c.url))
	}

	return skanIDList, true, nil
}

func (c client) fetchFromBidder() (model.SKANIDList, bool, error) {
	req, err := http.NewRequest("GET", c.url, nil)
	if err != nil {
		return model.SKANIDList{}, false, errors.New(fmt.Sprintf("error making request for bidder's servers for: %s - %v", c.url, err))
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return model.SKANIDList{}, false, errors.New(fmt.Sprintf("error fetching skanidlist from bidder's servers for: %s - %v", c.url, err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return model.SKANIDList{}, false, errors.New(fmt.Sprintf("error statuscode (%d) received from bidder's servers for: %s - %v", resp.StatusCode, c.url, err))
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return model.SKANIDList{}, true, errors.New(fmt.Sprintf("error reading skanidlist response body for: %s - %v", c.url, err))
	}

	var skanIDList model.SKANIDList
	err = json.Unmarshal(data, &skanIDList)
	if err != nil {
		return model.SKANIDList{}, true, errors.New(fmt.Sprintf("error unmarshaling response to skanidlist for: %s - %v", c.url, err))
	}

	return skanIDList, true, nil
}
