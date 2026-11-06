package app

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// dailyHistory and dailySongsOn load the days of daily songs past and the
// songs of one; tests replace them.
var (
	dailyHistory = netease.DailyHistory
	dailySongsOn = netease.DailySongsOn
)

// pastDaily is what the app has of the daily songs past: the days NetEase
// lets the listener look back on, a subscriber's last five, and the songs of
// those chosen.
type pastDaily struct {
	dates   []string // the latest first, as "2006-01-02"
	loaded  bool
	loading bool
	day     int // the day the daily page shows: 0 for today, i for dates[i-1]
	built   int // the day the page was last built of, to start another at its top
	days    map[string]*pastDay
}

// pastDay is the daily songs of a day past, as far as they have loaded.
type pastDay struct {
	songs   []netease.Track
	loading bool
	err     string
}

func (a *app) past() *pastDaily {
	if a.pastDaily == nil {
		a.pastDaily = &pastDaily{days: map[string]*pastDay{}}
	}
	return a.pastDaily
}

// loadPastDates loads the days the listener may look back on, once signed
// in, unless they have, or again.
func (a *app) loadPastDates(again ...bool) {
	h := a.past()
	if a.profile == nil || h.loading || h.loaded && len(again) == 0 {
		return
	}
	h.loading = true
	go func() {
		dates, err := dailyHistory()
		a.post(func() {
			h.loading = false
			if err != nil {
				return
			}
			// The day chosen stays chosen, though a day has passed.
			chosen := ""
			if h.day > 0 && h.day <= len(h.dates) {
				chosen = h.dates[h.day-1]
			}
			h.dates, h.loaded, h.day = dates, true, 0
			for i, date := range dates {
				if date == chosen {
					h.day = i + 1
				}
			}
		})
	}()
}

// choosePastDay shows the daily songs of day: 0 for today's, i for those of
// dates[i-1], loading them unless they have.
func (a *app) choosePastDay(day int) {
	h := a.past()
	if day == h.day {
		return
	}
	h.day = day
	if day > 0 {
		a.loadPastDay(h.dates[day-1])
	}
}

func (a *app) loadPastDay(date string) {
	h := a.past()
	d := h.days[date]
	if d == nil {
		d = &pastDay{}
		h.days[date] = d
	}
	if d.songs != nil || d.loading {
		return
	}
	d.loading, d.err = true, ""
	go func() {
		songs, err := dailySongsOn(date)
		a.post(func() {
			d.loading = false
			if err != nil {
				d.err = "历史日推加载失败"
				return
			}
			d.songs = songs
		})
	}()
}

// shownDay is the day of the daily songs the page shows, and the songs of a
// day past, nil for today's.
func (a *app) shownDay(today time.Time) (time.Time, *pastDay) {
	h := a.past()
	// Another day's songs start at their top. Not as the day is chosen:
	// its control is in the list, which builds then.
	if h.built != h.day {
		a.trackList, a.pageScroll, h.built = ui.ListState{}, ui.ScrollState{}, h.day
	}
	if h.day == 0 || h.day > len(h.dates) {
		return today, nil
	}
	date := h.dates[h.day-1]
	day, err := time.ParseInLocation("2006-01-02", date, beijing)
	if err != nil {
		day = today
	}
	d := h.days[date]
	if d == nil {
		d = &pastDay{}
	}
	return day, d
}

// pastDays lets the listener choose the day of the daily songs shown, today
// or one past, once there are days to look back on.
func (a *app) pastDays(c *ui.Context) {
	h := a.past()
	if len(h.dates) == 0 {
		return
	}
	labels := []string{"今天"}
	for _, date := range h.dates {
		label := date
		if t, err := time.Parse("2006-01-02", date); err == nil {
			label = fmt.Sprintf("%d月%d日", t.Month(), t.Day())
		}
		labels = append(labels, label)
	}
	day := h.day
	ui.Box(c).Margin(10, 0, 0, 0).Children(func() {
		if segmented(c.Key("past-days"), &day, labels...) {
			a.choosePastDay(day)
		}
	})
}
