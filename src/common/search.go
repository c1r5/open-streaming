package common

type TorrentSearchResult struct {
	TorrentName string   `json:"torrent_name"`
	Year        int      `json:"year"`
	Hash        string   `json:"hash"`
	FileIndex   int      `json:"file_index"`
	IMDB        string   `json:"imdb"`
	Trackers    []string `json:"trackers,omitempty"`
}

type ITorrentSearch interface {
	Search(query string) ([]*TorrentSearchResult, error)
}

type ErrTorrentNotFound struct {}

func (e ErrTorrentNotFound) Error() string {
	return "torrent not found"
}
