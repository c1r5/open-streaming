package domain

import "testing"

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
