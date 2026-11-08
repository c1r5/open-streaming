package ffmpeg

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMuxCopiesSubtitleWithLanguageTitleAndDefault(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Fatal("ffmpeg is not on PATH")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Fatal("ffprobe is not on PATH")
	}

	dir := t.TempDir()
	source := filepath.Join(dir, "in.mkv")
	subtitle := filepath.Join(dir, "in.srt")
	output := filepath.Join(dir, "out.mkv")

	write := exec.Command("ffmpeg",
		"-y",
		"-f", "lavfi", "-i", "testsrc=duration=1:size=160x120:rate=10",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1",
		"-c:v", "libx264", "-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-shortest",
		source,
	)
	if raw, err := write.CombinedOutput(); err != nil {
		t.Fatalf("source mkv: %v %s", err, raw)
	}
	if err := os.WriteFile(subtitle, []byte("1\n00:00:00,000 --> 00:00:01,000\nOi\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	before, err := probeFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := Mux(context.Background(), source, []MuxTrack{{
		Path:     subtitle,
		Kind:     "subtitle",
		Language: "por",
		Title:    "Legenda",
		Default:  true,
	}}, output); err != nil {
		t.Fatal(err)
	}

	after, err := probeFile(output)
	if err != nil {
		t.Fatal(err)
	}
	videoBefore, ok := firstCodec(before, "video")
	if !ok {
		t.Fatal("source has no video")
	}
	videoAfter, ok := firstCodec(after, "video")
	if !ok || videoAfter != videoBefore {
		t.Fatalf("video codec %q, source %q", videoAfter, videoBefore)
	}
	sub, ok := findStream(after, "subtitle")
	if !ok {
		t.Fatal("output has no subtitle")
	}
	if sub.CodecName == "" || sub.Tags["language"] != "por" || sub.Tags["title"] != "Legenda" || sub.Disposition["default"] != 1 {
		t.Fatalf("subtitle %+v", sub)
	}
}

type probeDoc struct {
	Streams []probeStream `json:"streams"`
}

type probeStream struct {
	CodecType   string            `json:"codec_type"`
	CodecName   string            `json:"codec_name"`
	Tags        map[string]string `json:"tags"`
	Disposition map[string]int    `json:"disposition"`
}

func probeFile(path string) (probeDoc, error) {
	cmd := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json", "-show_streams", path)
	raw, err := cmd.Output()
	if err != nil {
		return probeDoc{}, err
	}
	var doc probeDoc
	err = json.Unmarshal(raw, &doc)
	return doc, err
}

func firstCodec(doc probeDoc, kind string) (string, bool) {
	stream, ok := findStream(doc, kind)
	if !ok {
		return "", false
	}
	return stream.CodecName, true
}

func findStream(doc probeDoc, kind string) (probeStream, bool) {
	for _, stream := range doc.Streams {
		if stream.CodecType == kind {
			return stream, true
		}
	}
	return probeStream{}, false
}
