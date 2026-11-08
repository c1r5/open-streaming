package ffmpeg

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/c1r5/open-streaming/src/shared/config"
)

// EmbeddedMedia is the first video stream plus every audio and subtitle stream.
type EmbeddedMedia struct {
	Duration  float64
	Video     *MediaStream
	Audios    []MediaStream
	Subtitles []MediaStream
}

// MediaStream is one ffprobe stream the player panel can print.
type MediaStream struct {
	Index    int
	Codec    string
	Language string
	Title    string
	Channels int
	Width    int
	Height   int
	Default  bool
}

type mediaDocument struct {
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
	Streams []struct {
		Index       int               `json:"index"`
		CodecType   string            `json:"codec_type"`
		CodecName   string            `json:"codec_name"`
		Width       int               `json:"width"`
		Height      int               `json:"height"`
		Channels    int               `json:"channels"`
		Disposition map[string]int    `json:"disposition"`
		Tags        map[string]string `json:"tags"`
	} `json:"streams"`
}

// ProbeMedia reads container duration and embedded streams from a local file.
func ProbeMedia(path string) (*EmbeddedMedia, error) {
	cfg := config.Get().Transcode
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.ProbeTimeoutSeconds)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffprobe",
		"-fflags", "+ignidx",
		"-probesize", cfg.ProbeSize,
		"-analyzeduration", cfg.AnalyzeDuration,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe for %s failed: %w", path, err)
	}

	var doc mediaDocument
	if err := json.Unmarshal(output, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	media := &EmbeddedMedia{
		Audios:    []MediaStream{},
		Subtitles: []MediaStream{},
	}
	if doc.Format.Duration != "" {
		media.Duration, _ = strconv.ParseFloat(doc.Format.Duration, 64)
	}
	for _, stream := range doc.Streams {
		item := MediaStream{
			Index:    stream.Index,
			Codec:    stream.CodecName,
			Language: streamTag(stream.Tags, "language"),
			Title:    streamTag(stream.Tags, "title"),
			Channels: stream.Channels,
			Width:    stream.Width,
			Height:   stream.Height,
			Default:  stream.Disposition["default"] == 1,
		}
		switch stream.CodecType {
		case "video":
			if media.Video == nil {
				copy := item
				media.Video = &copy
			}
		case "audio":
			media.Audios = append(media.Audios, item)
		case "subtitle":
			media.Subtitles = append(media.Subtitles, item)
		}
	}
	return media, nil
}

func streamTag(tags map[string]string, key string) string {
	for name, value := range tags {
		if strings.EqualFold(name, key) {
			return value
		}
	}
	return ""
}
