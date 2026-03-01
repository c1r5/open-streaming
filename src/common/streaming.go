package common

type IStreamingService interface {
	GetStream(id uint) (*TorrentFile, error)
}

type IStreamingController interface {
	WatchStream() HandlerFunc
	GetStream() HandlerFunc
}
