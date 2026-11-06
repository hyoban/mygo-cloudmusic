package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// Refreshing a page loaded asks NetEase for it again, once while that is on
// its way, and keeps showing what it had; under Now Playing it asks nothing.
func TestRefreshLoadsAgain(t *testing.T) {
	asked := make(chan int64, 4)
	hold := make(chan struct{})
	albumDetail = func(id int64) (*netease.AlbumProfile, error) {
		asked <- id
		<-hold
		return &netease.AlbumProfile{}, nil
	}
	defer func() { close(hold); albumDetail = netease.AlbumDetail }()
	a := newTestApp()
	a.settings = defaultSettings()
	al := &netease.AlbumProfile{Album: netease.Album{ID: 8, Name: "迷跡波"}}
	a.albums[8] = &albumData{album: al}
	a.open(route{page: pageAlbum, id: 8})
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()

	a.nowPlaying = true
	a.refresh()
	select {
	case <-asked:
		t.Fatal("refreshed under Now Playing")
	case <-time.After(50 * time.Millisecond):
	}

	a.nowPlaying = false
	a.refresh()
	a.refresh()
	if id := wait(t, asked, "the album"); id != 8 {
		t.Fatalf("asked for album %d", id)
	}
	select {
	case <-asked:
		t.Fatal("asked twice")
	case <-time.After(50 * time.Millisecond):
	}
	tt.Frame()
	if _, ok := tt.Find("迷跡波"); !ok {
		t.Fatal("the album stopped showing while it refreshed")
	}
}
