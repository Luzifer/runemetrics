package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/gorhill/cronexpr"
	log "github.com/sirupsen/logrus"

	"github.com/Luzifer/rconfig/v2"
)

const (
	updateKeyTotalXP = "total_xp"
	updateKeyGeneral = "general"
	updateKeyFeed    = "feed"
)

var (
	cfg = struct {
		MarkerTime     time.Duration `flag:"marker-time" default:"30m" description:"How long to highlight new entries"`
		Update         string        `flag:"update" default:"* * * * *" description:"When to fetch metrics (cron syntax)"`
		LogLevel       string        `flag:"log-level" default:"info" description:"Log level (debug, info, warn, error, fatal)"`
		VersionAndExit bool          `flag:"version" default:"false" description:"Prints current version and exits"`
	}{}

	lastUpdate = map[string]time.Time{}

	version = "dev"
)

func init() {
	if err := rconfig.ParseAndValidate(&cfg); err != nil {
		log.Fatalf("Unable to parse commandline options: %s", err)
	}

	if cfg.VersionAndExit {
		fmt.Printf("git-changerelease %s\n", version)
		os.Exit(0)
	}

	if l, err := log.ParseLevel(cfg.LogLevel); err != nil {
		log.WithError(err).Fatal("Unable to parse log level")
	} else {
		log.SetLevel(l)
	}
}

func main() {
	var err error

	if len(rconfig.Args()) != 2 {
		log.Fatal("Usage: runemetrics <player>")
	}

	if playerInfoCache, err = loadPlayerInfoCache(); err != nil {
		log.WithError(err).Fatal("Unable to load cache")
	}

	if err = ui.Init(); err != nil {
		log.WithError(err).Fatal("Unable to initialize termui")
	}
	defer ui.Close()

	var (
		cron         = cronexpr.MustParse(cfg.Update)
		player       = rconfig.Args()[1]
		updateTicker = time.NewTimer(0)
	)

	updateUI(player)

	for {
		select {

		case evt := <-ui.PollEvents():
			switch evt.ID {

			case "q", "<C-c>":
				return

			case "<C-r>":
				updateTicker.Reset(0)

			case "<Resize>":
				ui.Clear()
				updateUI(player)

			}

		case <-updateTicker.C:
			if err := updateUI(player); err != nil {
				log.WithError(err).Error("Unable to update metrics")
				return
			}
			updateTicker.Reset(time.Until(cron.Next(time.Now())))

			if err := playerInfoCache.storeCache(); err != nil {
				log.WithError(err).Error("Unable to write cache")
			}

		}
	}
}

func updateUI(player string) error {
	termWidth, termHeight := ui.TerminalDimensions()

	playerData, err := getPlayerInfo(player, 20)

	// Status-bar
	status := widgets.NewParagraph()
	status.Title = "Status"
	status.Text = fmt.Sprintf("Last Refresh: %s | XP Change: %s | Feed Change: %s",
		lastUpdate[updateKeyGeneral].Format("15:04:05"),
		lastUpdate[updateKeyTotalXP].Format("15:04:05"),
		lastUpdate[updateKeyFeed].Format("15:04:05"),
	)
	status.SetRect(0, termHeight-3, termWidth, termHeight)
	defer ui.Render(status)

	if err != nil {
		status.Text = fmt.Sprintf("Unable to get player info: %s", err.Error())
		status.BorderStyle.Fg = ui.ColorRed
		return nil
	}

	// Header
	hdrText := widgets.NewParagraph()
	hdrText.Title = "Player"
	hdrText.Text = playerData.Name
	hdrText.SetRect(0, 0, termWidth, 3)
	ui.Render(hdrText)

	// General stats
	combatLevel := widgets.NewParagraph()
	combatLevel.Title = "Combat Level"
	combatLevel.Text = strconv.Itoa(playerData.CombatLevel)

	totalXP := widgets.NewParagraph()
	totalXP.Title = "Total XP"
	totalXP.Text = strconv.FormatInt(playerData.TotalXP, 10)

	totalLevel := widgets.NewParagraph()
	totalLevel.Title = "Total Level"
	totalLevel.Text = strconv.FormatInt(playerData.TotalSkill, 10)

	rank := widgets.NewParagraph()
	rank.Title = "Rank"
	rank.Text = strconv.FormatInt(playerData.NumericRank(), 10)

	statsGrid := ui.NewGrid()
	statsGrid.SetRect(0, 3, termWidth, 6)
	statsGrid.Set(
		ui.NewRow(1.0,
			ui.NewCol(1.0/4, combatLevel),
			ui.NewCol(1.0/4, totalXP),
			ui.NewCol(1.0/4, totalLevel),
			ui.NewCol(1.0/4, rank),
		),
	)
	ui.Render(statsGrid)

	// Levels
	levelTable := widgets.NewTable()
	levelTable.Title = "Levels"
	levelTable.TextAlignment = ui.AlignRight
	levelTable.RowStyles[0] = ui.Style{Fg: ui.ColorWhite, Modifier: ui.ModifierBold}
	levelTable.SetRect(0, 6, termWidth, 6+2+len(playerData.SkillValues)+1)
	levelTable.RowSeparator = false
	levelTable.Rows = [][]string{{"Skill", "Level", "Level %", "XP", "XP remaining"}}
	for i, s := range playerData.SkillValues {
		var (
			remaining  = strconv.FormatInt(s.ID.Info().XPToNextLevel(s.XP/10), 10)
			percentage = strconv.FormatFloat(s.ID.Info().LevelPercentage(s.XP/10), 'f', 1, 64)
		)

		if s.TargetLevel > 0 {
			remaining = strconv.FormatInt(s.ID.Info().XPToTargetLevel(s.TargetLevel, s.XP/10), 10)
			percentage = strconv.FormatFloat(s.ID.Info().TargetPercentage(s.TargetLevel, s.XP/10), 'f', 1, 64)
			levelTable.RowStyles[i+1] = ui.Style{Fg: ui.ColorYellow}
		}

		levelTable.Rows = append(levelTable.Rows, []string{
			s.ID.String(),
			strconv.Itoa(s.Level),
			percentage,
			strconv.FormatInt(s.XP/10, 10),
			remaining,
		})

		if time.Since(s.Updated) < cfg.MarkerTime {
			levelTable.RowStyles[i+1] = ui.Style{Fg: ui.ColorGreen}
		}
	}
	ui.Render(levelTable)

	// Latest events
	events := widgets.NewTable()
	events.Title = "Event Log"
	events.RowSeparator = false
	events.ColumnWidths = []int{12, termWidth - 3 - 12}
	events.SetRect(0, 6+2+len(playerData.SkillValues)+1, termWidth, termHeight-3)
	for i, logEntry := range playerData.Activities {
		date, _ := logEntry.GetParsedDate()
		events.Rows = append(
			events.Rows,
			[]string{
				date.Local().Format("01/02 15:04"),
				strings.Replace(logEntry.Details, "  ", " ", -1),
			},
		)

		if time.Since(date) < cfg.MarkerTime {
			events.RowStyles[i] = ui.Style{Fg: ui.ColorGreen}
		}
	}
	ui.Render(events)

	return nil
}
