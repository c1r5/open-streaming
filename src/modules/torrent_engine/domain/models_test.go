package domain

import (
	"strings"
	"testing"
)

func TestParseLowercaseBase32Magnet(t *testing.T) {
	uri := "magnet:?xt=urn:btih:4otzjkrcktpqb3rukf4r3ntl3iwvoi2n&tr=udp%3A%2F%2Ftracker.opentrackr.org%3A1337%2Fannounce&dn=BAIXARAPIDO.COM-.WEB-DL.1080P.MKV.5.1.-DUBLADO-DUAL-AUDIO-.Moana.2026.1080p.WEB-DL.x264.DUAL.5.1"

	got, err := Parse(uri)
	if err != nil {
		t.Fatalf("Parse(%q) error = %v", uri, err)
	}

	const wantHash = "e3a794aa2254df00ee3451791db66bda2d57234d"
	if got.Hash != wantHash {
		t.Errorf("Hash = %q, want %q", got.Hash, wantHash)
	}

	const wantName = "BAIXARAPIDO.COM-.WEB-DL.1080P.MKV.5.1.-DUBLADO-DUAL-AUDIO-.Moana.2026.1080p.WEB-DL.x264.DUAL.5.1"
	if got.Name != wantName {
		t.Errorf("Name = %q, want %q", got.Name, wantName)
	}

	wantTracker := "tracker:udp://tracker.opentrackr.org:1337/announce"
	if len(got.Sources) != 1 || got.Sources[0] != wantTracker {
		t.Errorf("Sources = %#v, want [%q]", got.Sources, wantTracker)
	}
}

func TestParseDropsTrackerPrefixAndUnknownScheme(t *testing.T) {
	uri := "magnet:?xt=urn:btih:0130ea6e4522b8fbb5349b17db5da33994933ac7&tr=tracker%3Audp%3A%2F%2Fexplodie.org%3A6969&tr=lol%3A%2F%2Ftracker.openbittorrent.com%3A80%2Fannounce"

	got, err := Parse(uri)
	if err != nil {
		t.Fatalf("Parse(%q) error = %v", uri, err)
	}

	wantSource := "tracker:udp://explodie.org:6969"
	if len(got.Sources) != 1 || got.Sources[0] != wantSource {
		t.Fatalf("Sources = %#v, want [%q]", got.Sources, wantSource)
	}

	trackers := got.Trackers()
	if len(trackers) != 1 || trackers[0] != "udp://explodie.org:6969" {
		t.Fatalf("Trackers() = %#v", trackers)
	}

	magnet := got.Magnet()
	if strings.Contains(magnet, "tracker") || strings.Contains(magnet, "lol") {
		t.Fatalf("Magnet() = %q", magnet)
	}
	if !strings.Contains(magnet, "tr=udp") {
		t.Fatalf("Magnet() = %q, want a udp tracker", magnet)
	}
}
