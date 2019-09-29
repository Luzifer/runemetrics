package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

var (
	knownTotalXP int64
	knownFeed    time.Time

	playerInfoCache *playerInfo
)

type activity struct {
	Date    string `json:"date"`
	Details string `json:"details"`
	Text    string `json:"text"`
}

func (a activity) GetParsedDate() (time.Time, error) {
	loc, err := time.LoadLocation("Europe/London")
	if err != nil {
		return time.Time{}, errors.Wrap(err, "Unable to load London time information")
	}

	return time.ParseInLocation("02-Jan-2006 15:04", a.Date, loc)
}

type skill struct {
	ID    skillID `json:"id"`
	Level int     `json:"level"`
	Rank  int64   `json:"rank"`
	XP    int64   `json:"xp"`

	TargetLevel int
	Updated     time.Time
}

type playerInfo struct {
	Activities       []activity `json:"activities"`
	CombatLevel      int        `json:"combatlevel"`
	LoggedIn         bool       `json:"loggedIn,string"`
	Magic            int64      `json:"magic"`
	Melee            int64      `json:"melee"`
	Name             string     `json:"name"`
	QuestsComplete   int        `json:"questscomplete"`
	QuestsNotStarted int        `json:"questsnotstarted"`
	QuestsStarted    int        `json:"questsstarted"`
	Ranged           int64      `json:"ranged"`
	Rank             string     `json:"rank"`
	SkillValues      []skill    `json:"skillvalues"`
	TotalSkill       int64      `json:"totalskill"`
	TotalXP          int64      `json:"totalxp"`
}

func (p playerInfo) NumericRank() int64 {
	v, _ := strconv.ParseInt(strings.Replace(p.Rank, ",", "", -1), 10, 64)
	return v
}

func (p playerInfo) GetSkill(s skillID) skill {
	for _, sk := range p.SkillValues {
		if sk.ID == s {
			return sk
		}
	}
	return skill{}
}

func (p playerInfo) storeCache() error {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return errors.Wrap(err, "Unable to retrieve user cache dir")
	}

	cacheDir = path.Join(cacheDir, "luzifer", "runemetrics")
	if err = os.MkdirAll(cacheDir, 0755); err != nil {
		return errors.Wrap(err, "Unable to create cache dir")
	}

	cacheFile := path.Join(cacheDir, "metrics.json")
	f, err := os.Create(cacheFile)
	if err != nil {
		return errors.Wrap(err, "Unable to create cache file")
	}
	defer f.Close()

	return errors.Wrap(json.NewEncoder(f).Encode(p), "Unable to marshal into cache file")
}

func getPlayerInfo(name string, activities int) (*playerInfo, error) {
	if name == "" {
		return nil, errors.New("Player name must not be empty")
	}

	params := url.Values{
		"user":       []string{name},
		"activities": []string{strconv.Itoa(activities)},
	}
	uri := "https://apps.runescape.com/runemetrics/profile/profile?" + params.Encode()

	resp, err := http.Get(uri)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to query profile data")
	}
	defer resp.Body.Close()

	out := &playerInfo{}
	if err = json.NewDecoder(resp.Body).Decode(out); err != nil {
		return nil, errors.Wrap(err, "Unable to decode profile data")
	}

	if playerInfoCache != nil {
		for i, nSk := range out.SkillValues {
			oSk := playerInfoCache.GetSkill(nSk.ID)

			if oSk.TargetLevel > nSk.Level {
				out.SkillValues[i].TargetLevel = oSk.TargetLevel
			}

			if oSk.XP == nSk.XP {
				out.SkillValues[i].Updated = oSk.Updated
				continue
			}

			out.SkillValues[i].Updated = time.Now()
		}

		var (
			lastActivity = out.Activities[len(out.Activities)-1]
			skip         = true
		)

		for _, a := range playerInfoCache.Activities {
			// Times are no good match: they might be duplicated, we search
			// last message which should never duplicate.
			if a.Details == lastActivity.Details {
				skip = false
				continue
			}

			if skip {
				continue
			}

			out.Activities = append(out.Activities, a)
		}
	}

	if knownTotalXP != out.TotalXP {
		knownTotalXP = out.TotalXP
		lastUpdate[updateKeyTotalXP] = time.Now()
	}

	if d, _ := out.Activities[0].GetParsedDate(); !d.Equal(knownFeed) {
		knownFeed = d
		lastUpdate[updateKeyFeed] = time.Now()
	}

	lastUpdate[updateKeyGeneral] = time.Now()

	playerInfoCache = out

	return out, nil
}

func loadPlayerInfoCache() (*playerInfo, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, errors.Wrap(err, "Unable to retrieve user cache dir")
	}

	cacheFile := path.Join(cacheDir, "luzifer", "runemetrics", "metrics.json")
	if _, err := os.Stat(cacheFile); err != nil {
		if os.IsNotExist(err) {
			// Empty cache
			return nil, nil
		}
		return nil, errors.Wrap(err, "Unable to stat cache file")
	}

	f, err := os.Open(cacheFile)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to open cache file")
	}
	defer f.Close()

	p := &playerInfo{}
	return p, errors.Wrap(json.NewDecoder(f).Decode(p), "Unable to unmarshal cache file")
}
