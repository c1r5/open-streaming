package transcoder

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/c1r5/open-streaming/src/config"
)

var transcodeSem chan struct{}

func init() {
	transcodeSem = make(chan struct{}, config.Get().Transcode.MaxConcurrent)
}

func Transcode(streamURL string, seq int, startSec, durSec float64, outPath string) error {
	err := doTranscode(streamURL, seq, startSec, durSec, outPath)
	if err != nil {
		log.Printf("transcode seg %d failed, retrying: %v", seq, err)
		err = doTranscode(streamURL, seq, startSec, durSec, outPath)
	}
	return err
}

func doTranscode(streamURL string, seq int, startSec, durSec float64, outPath string) error {
	transcodeSem <- struct{}{}
	defer func() { <-transcodeSem }()

	timeout := time.Duration(config.Get().Transcode.TimeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cfg := config.Get().Transcode

	args := []string{
		"-fflags", "+ignidx",
		"-ss", fmt.Sprintf("%.3f", startSec),
		"-i", streamURL,
		"-t", fmt.Sprintf("%.3f", durSec),

		// Gera áudio silencioso como âncora de tempo
		"-async", "1",

		"-c:v", "libx264", "-preset", cfg.Preset, "-crf", cfg.CRF,
		"-profile:v", cfg.VideoProfile, "-level", cfg.VideoLevel, "-pix_fmt", "yuv420p",
		"-force_key_frames", "expr:eq(n,0)",
		"-c:a", "aac", "-b:a", cfg.AudioBitrate, "-ac", fmt.Sprintf("%d", cfg.AudioChannels),
		"-avoid_negative_ts", "make_zero",
		"-fflags", "+genpts",
		"-f", "mpegts",
		"-muxdelay", "0", "-muxpreload", "0",
		"-shortest",
		"-y",
		outPath,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		os.Remove(outPath)
		return fmt.Errorf("ffmpeg seg %d failed: %w", seq, err)
	}

	return nil
}
