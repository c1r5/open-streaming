package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type TorrentStream struct {
	Title    string   `json:"title,omitempty"`
	InfoHash string   `json:"infoHash,omitempty"`
	FileIdx  int      `json:"fileIdx,omitempty"`
	Sources  []string `json:"sources,omitempty"`
}

type TorrentioSearchResult struct {
	Streams []*TorrentStream `json:"streams,omitempty"`
}

type TorrentioAPI struct {
	BaseURL   string
	UserAgent string
}

func CreateTorrentio(baseURL, userAgent string) *TorrentioAPI {
	return &TorrentioAPI{BaseURL: baseURL, UserAgent: userAgent}
}

func (t *TorrentioAPI) Search(id string) ([]*TorrentStream, error) {
	request_url, err := url.Parse(t.BaseURL)

	if err != nil {
		return nil, fmt.Errorf("torrentio_search_url_error: %s", err.Error())
	}

	request_url = request_url.JoinPath("brazuca/stream/movie", fmt.Sprintf("%s.json", id))
	request, err := http.NewRequest(http.MethodGet, request_url.String(), nil)

	if err != nil {
		return nil, fmt.Errorf("torrentio_search_request_error: %s", err.Error())
	}

	request.Header.Set("user-agent", t.UserAgent)
	response, err := http.DefaultClient.Do(request)

	var result *TorrentioSearchResult

	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("torrent_search_decode_error: %s", err.Error())
	}
	defer response.Body.Close()

	return result.Streams, nil
}
