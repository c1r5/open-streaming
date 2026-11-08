package ffmpeg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// MuxTrack is one audio or subtitle file copied into a Matroska file.
type MuxTrack struct {
	Path     string
	Kind     string
	Language string
	Title    string
	Default  bool
}

type countedStreams struct {
	videoCodec string
	audio      int
	subtitle   int
}

type placedTrack struct {
	kind     string
	index    int
	def      bool
	language string
	title    string
}

// Mux copies source streams and the given tracks into output.
// output is removed when ffmpeg exits with an error.
func Mux(ctx context.Context, source string, tracks []MuxTrack, output string) error {
	counts, err := countStreams(ctx, source)
	if err != nil {
		return err
	}
	if counts.videoCodec == "" {
		return fmt.Errorf("ffmpeg mux: source has no video stream")
	}

	args := []string{"-y", "-i", source}
	for _, track := range tracks {
		args = append(args, "-i", track.Path)
	}
	args = append(args, "-map", "0")

	audioN := counts.audio
	subN := counts.subtitle
	placed := make([]placedTrack, 0, len(tracks))
	for i, track := range tracks {
		input := i + 1
		switch track.Kind {
		case "audio":
			args = append(args, "-map", fmt.Sprintf("%d:a:0", input))
			placed = append(placed, placedTrack{"a", audioN, track.Default, track.Language, track.Title})
			audioN++
		case "subtitle":
			args = append(args, "-map", fmt.Sprintf("%d:s:0", input))
			placed = append(placed, placedTrack{"s", subN, track.Default, track.Language, track.Title})
			subN++
		default:
			return fmt.Errorf("ffmpeg mux: unknown track kind %q", track.Kind)
		}
	}
	args = append(args, "-c", "copy")
	for _, track := range placed {
		spec := fmt.Sprintf("s:%s:%d", track.kind, track.index)
		if track.language != "" {
			args = append(args, "-metadata:"+spec, "language="+track.language)
		}
		if track.title != "" {
			args = append(args, "-metadata:"+spec, "title="+track.title)
		}
	}
	for _, kind := range []string{"a", "s"} {
		chosen := -1
		for _, track := range placed {
			if track.kind == kind && track.def {
				chosen = track.index
			}
		}
		if chosen < 0 {
			continue
		}
		total := audioN
		if kind == "s" {
			total = subN
		}
		for i := 0; i < total; i++ {
			value := "0"
			if i == chosen {
				value = "default"
			}
			args = append(args, fmt.Sprintf("-disposition:%s:%d", kind, i), value)
		}
	}
	args = append(args, "-f", "matroska", output)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		os.Remove(output)
		return fmt.Errorf("ffmpeg mux: %s", lastLine(stderr.String()))
	}
	return nil
}

func countStreams(ctx context.Context, path string) (countedStreams, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		path,
	)
	output, err := cmd.Output()
	if err != nil {
		return countedStreams{}, fmt.Errorf("ffprobe for %s failed: %w", path, err)
	}
	var probe struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(output, &probe); err != nil {
		return countedStreams{}, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}
	var counts countedStreams
	for _, stream := range probe.Streams {
		switch stream.CodecType {
		case "video":
			if counts.videoCodec == "" {
				counts.videoCodec = stream.CodecName
			}
		case "audio":
			counts.audio++
		case "subtitle":
			counts.subtitle++
		}
	}
	return counts, nil
}

func lastLine(text string) string {
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line
		}
	}
	return "ffmpeg failed"
}
