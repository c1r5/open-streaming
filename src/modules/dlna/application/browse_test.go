package application

import (
	"strconv"
	"testing"

	"github.com/anacrolix/dms/upnpav"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/persistence"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

func TestBrowse_seriesPathAndMoviesStaySeparate(t *testing.T) {
	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&persistence.TorrentModel{}); err != nil {
		t.Fatal(err)
	}

	const season = "Series/Big Bang - A Teoria 2006 - 1ª Temporada Completa (720p) Acesse o ORIGINAL WWW.BLUDV.TV"
	episodeOne := persistence.TorrentModel{
		Hash:        "pack",
		FileIndex:   3,
		TorrentName: season + "/The.Big.Bang.Theory.S01E01.720p.BluRay.x264.DUAL-WWW.BLUDV.TV.mkv",
		Size:        100,
	}
	episodeTwo := persistence.TorrentModel{
		Hash:        "pack",
		FileIndex:   7,
		TorrentName: season + "/The.Big.Bang.Theory.S01E02.720p.BluRay.x264.DUAL-WWW.BLUDV.TV.mkv",
		Size:        500,
	}
	loose := persistence.TorrentModel{
		Hash:        "loose",
		FileIndex:   1,
		TorrentName: "Other Show/Episode.S02E03.mkv",
		Size:        40,
	}
	bolt := persistence.TorrentModel{Hash: "bolt", FileIndex: 0, TorrentName: "bolt_supercao.mkv", Size: 10}
	for _, row := range []*persistence.TorrentModel{&episodeTwo, &bolt, &episodeOne, &loose} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	service := NewService(db, "http://192.168.1.109:3000")
	root, err := service.BrowseDirectChildren("/", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(root) != 2 || containerTitle(t, root[0]) != "Movies" || containerTitle(t, root[1]) != "Series" {
		t.Fatalf("root %#v", titles(root))
	}
	if containerCount(t, root[1]) != 2 {
		t.Fatalf("series child count %d", containerCount(t, root[1]))
	}

	movies, err := service.BrowseDirectChildren("/Movies", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(movies) != 1 || itemTitle(t, movies[0]) != "bolt_supercao.mkv" || itemURL(t, movies[0]) != "http://192.168.1.109:3000/stream/watch/"+idOf(bolt.ID) {
		t.Fatalf("movies %#v", movies)
	}

	series, err := service.BrowseDirectChildren("/Series", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(series) != 2 || containerTitle(t, series[0]) != "Big Bang - A Teoria 2006 - 1ª Temporada Completa (720p) Acesse o ORIGINAL WWW.BLUDV.TV" || containerTitle(t, series[1]) != "Other Show" {
		t.Fatalf("series folders %#v", titles(series))
	}

	seasonPath := "/Series/Big Bang - A Teoria 2006 - 1ª Temporada Completa (720p) Acesse o ORIGINAL WWW.BLUDV.TV"
	episodes, err := service.BrowseDirectChildren(seasonPath, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(episodes) != 2 {
		t.Fatalf("episodes %d", len(episodes))
	}
	if itemTitle(t, episodes[0]) != "The.Big.Bang.Theory.S01E01.720p.BluRay.x264.DUAL-WWW.BLUDV.TV.mkv" {
		t.Fatalf("first episode %q", itemTitle(t, episodes[0]))
	}
	if itemURL(t, episodes[0]) != "http://192.168.1.109:3000/stream/watch/"+idOf(episodeOne.ID) {
		t.Fatalf("watch url %q", itemURL(t, episodes[0]))
	}
	if itemID(t, episodes[0]) != seasonPath+"/"+itemTitle(t, episodes[0])+"#"+idOf(episodeOne.ID) {
		t.Fatalf("object id %q", itemID(t, episodes[0]))
	}

	meta, err := service.BrowseMetadata(seasonPath, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if containerCount(t, meta) != 2 || containerTitle(t, meta) != "Big Bang - A Teoria 2006 - 1ª Temporada Completa (720p) Acesse o ORIGINAL WWW.BLUDV.TV" {
		t.Fatalf("season metadata %#v", meta)
	}
}

func idOf(id uint) string { return strconv.FormatUint(uint64(id), 10) }

func containerTitle(t *testing.T, value interface{}) string {
	t.Helper()
	container, ok := value.(upnpav.Container)
	if !ok {
		t.Fatalf("not a container: %T", value)
	}
	return container.Title
}

func containerCount(t *testing.T, value interface{}) int {
	t.Helper()
	container, ok := value.(upnpav.Container)
	if !ok {
		t.Fatalf("not a container: %T", value)
	}
	return container.ChildCount
}

func itemTitle(t *testing.T, value interface{}) string {
	t.Helper()
	return asItem(t, value).Title
}

func itemURL(t *testing.T, value interface{}) string {
	t.Helper()
	item := asItem(t, value)
	if len(item.Res) != 1 {
		t.Fatalf("resources %#v", item.Res)
	}
	return item.Res[0].URL
}

func itemID(t *testing.T, value interface{}) string {
	t.Helper()
	return asItem(t, value).ID
}

func asItem(t *testing.T, value interface{}) upnpav.Item {
	t.Helper()
	item, ok := value.(upnpav.Item)
	if !ok {
		t.Fatalf("not an item: %T", value)
	}
	return item
}

func titles(values []interface{}) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		switch object := value.(type) {
		case upnpav.Container:
			out = append(out, object.Title)
		case upnpav.Item:
			out = append(out, object.Title)
		}
	}
	return out
}
