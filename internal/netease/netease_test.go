package netease

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPlayableFollowsPrivilege(t *testing.T) {
	tracks := []Track{{ID: 1, Fee: 1}, {ID: 2, Fee: 8}, {ID: 3, Fee: 8}, {ID: 4, Fee: 1}}
	if tracks[0].Playable() || !tracks[1].Playable() {
		t.Fatal("without privileges, VIP songs do not play and free ones do")
	}
	withPrivileges(tracks, []Privilege{
		{ID: 1, Pl: 320000},           // VIP song, the listener subscribes
		{ID: 2, Pl: 0},                // nothing to play
		{ID: 3, St: -200, Pl: 128000}, // taken down
	})
	want := []bool{true, false, false, false}
	for i, tr := range tracks {
		if tr.Playable() != want[i] {
			t.Errorf("track %d: playable %v", tr.ID, tr.Playable())
		}
	}
}

// SongURL asks again when NetEase answers -460, which it does now and then,
// and says so if it keeps on: it answered ErrUnavailable, for a song that
// cannot play, which the player skipped.
func TestSongURLRisk(t *testing.T) {
	waits := riskWaits
	riskWaits = []time.Duration{time.Millisecond, time.Millisecond}
	defer func() { riskWaits, songURLPost = waits, eapiPost }()
	risky := 2
	songURLPost = func(_ string, _ map[string]any, out any) error {
		body := `{"code":200,"data":[{"url":"http://m7.music.126.net/a.mp3","type":"mp3","level":"exhigh","size":3}]}`
		if risky > 0 {
			risky--
			body = `{"code":-460,"message":"检测到您的网络环境存在风险，请稍后再试"}`
		}
		return json.Unmarshal([]byte(body), out)
	}
	st, err := SongURL(1, "exhigh")
	if err != nil || st.URL != "https://m7.music.126.net/a.mp3" {
		t.Fatalf("%+v, %v", st, err)
	}

	risky = 3
	if _, err := SongURL(1, "exhigh"); err == nil || err == ErrUnavailable {
		t.Fatalf("after -460 throughout: %v", err)
	}
}
