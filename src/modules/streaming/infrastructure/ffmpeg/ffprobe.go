package ffmpeg

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/c1r5/open-streaming/src/shared/config"
)

type VideoInfo struct {
	Duration   float64
	Width      int
	Height     int
	VideoCodec string
	AudioCodec string
}

var (
	probeCache sync.Map
)

type ffprobeOutput struct {
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
	Streams []struct {
		CodecType string `json:"codec_type"`
		CodecName string `json:"codec_name"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
	} `json:"streams"`
}

func Probe(streamURL string) (*VideoInfo, error) {
	if cached, ok := probeCache.Load(streamURL); ok {
		return cached.(*VideoInfo), nil
	}

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
		streamURL,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe for %s failed: %w", streamURL, err)
	}

	var probe ffprobeOutput
	if err := json.Unmarshal(output, &probe); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	info := &VideoInfo{}

	if probe.Format.Duration != "" {
		info.Duration, _ = strconv.ParseFloat(probe.Format.Duration, 64)
	}

	for _, s := range probe.Streams {
		switch s.CodecType {
		case "video":
			if info.VideoCodec == "" {
				info.VideoCodec = s.CodecName
				info.Width = s.Width
				info.Height = s.Height
			}
		case "audio":
			if info.AudioCodec == "" {
				info.AudioCodec = s.CodecName
			}
		}
	}

	if info.Duration <= 0 {
		return nil, fmt.Errorf("could not determine video duration")
	}

	probeCache.Store(streamURL, info)
	return info, nil
}
