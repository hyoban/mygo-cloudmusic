package app

import (
	"errors"
	"time"

	"netease-music-mygo/internal/netease"
)

// playlistDetail loads a playlist from NetEase; tests replace it.
var playlistDetail = netease.PlaylistDetail

// refresh loads what the page open shows anew, as ⌘R asks. What it shows
// stays until what replaces it has come, and stays if that fails. A page
// that has nothing to show, having failed to load, loads as its 重试
// would. Now Playing, over the page, hides it, so nothing loads then.
func (a *app) refresh() {
	if a.nowPlaying {
		return
	}
	r := a.route
	switch r.page {
	case pageHome, pageDaily, pageCharts:
		if a.homeLoading {
			return
		}
		if len(a.featured) == 0 && len(a.charts) == 0 {
			a.loadHome()
			return
		}
		signedIn, at := a.profile != nil, time.Now()
		a.refetch(route{page: pageHome}, func() (func(), error) {
			featured, err1 := netease.FeaturedPlaylists(30)
			charts, err2 := netease.Charts()
			daily, _ := netease.DailySongs()
			var rec []netease.Playlist
			if signedIn {
				rec, _ = netease.RecommendedPlaylists()
			}
			if err1 != nil && err2 != nil {
				return nil, errors.Join(err1, err2)
			}
			return func() {
				a.featured, a.charts, a.daily, a.recPlaylists = featured, charts, daily, rec
				a.homeAt = at
			}, nil
		})

	case pagePlaylist:
		if a.playlists[r.id] == nil {
			a.loadPlaylist(r.id, false)
			return
		}
		a.refetch(r, func() (func(), error) {
			pl, err := playlistDetail(r.id)
			return func() { a.playlists[r.id] = pl }, err
		})

	case pageAlbum:
		d := a.albums[r.id]
		if d.album == nil {
			a.loadAlbum(r.id)
			return
		}
		a.refetch(r, func() (func(), error) {
			al, err := albumDetail(r.id)
			return func() { d.album = al }, err
		})

	case pageArtist:
		d := a.artist(r.id)
		if d.profile == nil {
			a.loadArtist(r.id)
			return
		}
		// Who they are, with their most popular songs, and the tab shown.
		tab, uid := a.artistTab, d.profile.AccountID
		a.refetch(r, func() (func(), error) {
			ar, err := artistDetail(r.id)
			if err != nil {
				return nil, err
			}
			switch tab {
			case artistLatest:
				songs, err := artistLatestSongs(r.id, latestCount)
				return func() { d.profile, d.latest = ar, songs }, err
			case artistAlbums:
				albums, more, err := albumsBy(r.id, albumsPage, 0)
				return func() {
					d.profile, d.albums = ar, albums
					d.albumsPage = paged{loaded: true, more: more && len(albums) > 0}
				}, err
			case artistEvents:
				if uid == 0 {
					break
				}
				events, last, more, err := accountEvents(uid, eventsPage, -1)
				return func() {
					d.profile, d.events, d.eventsBefore = ar, events, last
					d.eventsPage = paged{loaded: true, more: more && len(events) > 0}
				}, err
			}
			return func() { d.profile = ar }, nil
		})

	case pageUser:
		d := a.users[r.id]
		if d.user == nil {
			a.loadUser(r.id)
			return
		}
		if a.profile != nil && a.profile.UserID == r.id {
			a.loadSubscriptions(d)
		}
		a.refetch(r, func() (func(), error) {
			u, err := userDetail(r.id)
			if err != nil {
				return nil, err
			}
			made, _, err := userPlaylists(r.id)
			return func() { d.user, d.playlists = u, publicPlaylists(made, r.id) }, err
		})

	case pagePodcast:
		d := a.podcasts[r.id]
		if d.podcast == nil || !d.page.loaded {
			a.loadPodcast(r.id)
			return
		}
		a.refetch(r, func() (func(), error) {
			pod, err := podcastDetail(r.id)
			if err != nil {
				return nil, err
			}
			programs, more, err := podcastPrograms(r.id, programsPage, 0)
			return func() {
				d.podcast, d.programs = pod, programs
				d.page = paged{loaded: true, more: more && len(programs) > 0}
				ids := make([]int64, len(programs))
				for i := range programs {
					ids[i] = programs[i].ID
				}
				a.askProgramLikes(ids)
			}, err
		})

	case pagePodcasts:
		t := a.podcastTab
		if !t.hotPage.loaded {
			a.loadPodcastTab()
			return
		}
		signedIn := a.profile != nil
		a.refetch(r, func() (func(), error) {
			var subs []netease.Podcast
			if signedIn {
				subs, _ = subscribedPodcasts()
			}
			picks, err1 := recommendedPodcasts()
			hot, more, err2 := hotPodcasts(programsPage, 0)
			if err1 != nil && err2 != nil {
				return nil, errors.Join(err1, err2)
			}
			return func() {
				t.subs, t.picks = subs, picks
				if err2 == nil {
					t.hot, t.hotAt = nil, 0
					t.addHot(hot)
					t.hotPage = paged{loaded: true, more: more && len(hot) > 0}
				}
			}, nil
		})

	case pageSearch:
		d := a.searches[r]
		if d == nil || !d.page.loaded {
			a.loadSearch(r, true)
			return
		}
		a.refetch(r, func() (func(), error) {
			res, err := searchResults(r.query, r.kind, searchLimit, 0)
			return func() {
				fresh := &searchData{res: res}
				fresh.page = paged{loaded: true, more: res.Len() > 0 && res.Len() < res.Total}
				a.searches[r] = fresh
			}, err
		})
	}
}

// refetch runs fetch on a goroutine, once at a time for each page, and then
// what it returns on the main thread: in place of what the page showed, or,
// if it failed, only to say so.
func (a *app) refetch(r route, fetch func() (apply func(), err error)) {
	if a.refreshing[r] {
		return
	}
	if a.refreshing == nil {
		a.refreshing = map[route]bool{}
	}
	a.refreshing[r] = true
	go func() {
		apply, err := fetch()
		a.post(func() {
			delete(a.refreshing, r)
			if err != nil {
				a.toast = "刷新失败"
				return
			}
			apply()
		})
	}()
}
