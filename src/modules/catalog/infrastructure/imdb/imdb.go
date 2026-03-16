package imdb

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type TitleImage struct {
	URL string `json:"url,omitempty"`
}
type Title struct {
	Id    string `json:"id,omitempty"`
	Type  string `json:"type,omitempty"`
	Title string `json:"primaryTitle,omitempty"`
	Year  int    `json:"startYear,omitempty"`
	Image *TitleImage `json:"primaryImage,omitempty"`
}

type IMDBResult struct {
	Titles []*Title `json:"titles,omitempty"`
}

type IMDB struct {
	BaseURL string
}

func CreateIMDB(baseURL string) *IMDB {
	return &IMDB{BaseURL: baseURL}
}

func (c *IMDB) Search(q string) ([]*Title, error) {
	var result *IMDBResult

	params := url.Values{}
	params.Set("query", q)
	params.Set("limit", "5")
	resp, err := c.get(fmt.Sprintf("/search/titles?%s", params.Encode()))

	if err != nil {
		return nil, err
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Titles, nil
}

func (i *IMDB) get(endpoint string) (*http.Response, error) {

	if !strings.HasPrefix(endpoint, "/") {
		endpoint = fmt.Sprintf("%s%s", "/", endpoint)
	}

	api_url := fmt.Sprintf("%s%s", i.BaseURL, endpoint)

	return http.Get(api_url)

}
