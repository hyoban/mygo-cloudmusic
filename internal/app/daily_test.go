package app

import (
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// The daily songs count as new from 6:00 in Beijing, wherever the listener
// is, so a home page loaded before then loads again.
func TestDailyRenewed(t *testing.T) {
	at := func(day, hour, min int) time.Time { return time.Date(2026, 10, day, hour, min, 0, 0, beijing) }
	for _, c := range []struct {
		loaded, now time.Time
		want        bool
	}{
		{at(26, 23, 0), at(27, 5, 59), false},
		{at(26, 23, 0), at(27, 6, 0), true},
		{at(27, 5, 59), at(27, 6, 1), true},
		{at(27, 6, 30), at(27, 23, 59), false},
		{at(27, 6, 30), at(28, 5, 0), false},
		{at(27, 6, 30), at(29, 1, 0), true},
	} {
		if got := dailyRenewed(c.loaded, c.now); got != c.want {
			t.Errorf("loaded %v, now %v: renewed %v", c.loaded, c.now, got)
		}
	}

	// Midnight where the listener is isn't NetEase's new day: 0:30 in
	// Los Angeles is 15:30 in Beijing, long after its 6:00.
	la, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Skip(err)
	}
	loaded := time.Date(2026, 10, 26, 23, 0, 0, 0, la)
	if dailyRenewed(loaded, time.Date(2026, 10, 27, 0, 30, 0, 0, la)) {
		t.Error("renewed at midnight in Los Angeles")
	}
	if !dailyRenewed(loaded, time.Date(2026, 10, 27, 15, 0, 0, 0, la)) {
		t.Error("not renewed past 6:00 in Beijing")
	}
}

// Choosing a day past on the daily page asks for its songs, and 播放全部
// plays them rather than today's.
func TestPastDailyDay(t *testing.T) {
	asked := make(chan string, 2)
	dailySongsOn = func(date string) ([]netease.Track, error) {
		asked <- date
		return nil, errors.New("offline")
	}
	defer func() { dailySongsOn = netease.DailySongsOn }()
	a := newTestApp()
	a.settings = defaultSettings()
	a.profile = &netease.Profile{UserID: 1}
	a.daily = []netease.Track{{ID: 1, Name: "今天的歌", Duration: 200000}}
	h := a.past()
	h.dates, h.loaded = []string{"2026-11-05", "2026-11-04"}, true
	a.open(route{page: pageDaily})
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()

	if err := tt.Click("11月4日"); err != nil {
		t.Fatal(err)
	}
	if date := wait(t, asked, "the day's songs"); date != "2026-11-04" {
		t.Fatalf("asked for %s", date)
	}
	h.days["2026-11-04"] = &pastDay{songs: []netease.Track{{ID: 2, Name: "那天的歌", Duration: 200000}}}
	tt.Frame()
	if err := tt.Click("播放全部"); err != nil {
		t.Fatal(err)
	}
	if cur := a.pl.Current(); cur == nil || cur.ID != 2 {
		t.Fatalf("playing %+v", cur)
	}
}
