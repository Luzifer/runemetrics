package main

import (
	"fmt"
	"math"
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

	eventsPage     = 0
	lastUpdate     = map[string]time.Time{}
	playerData     *playerInfo
	selectedMetric = 0

	inputPrompt string
	inputBuffer string

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

	for {
		select {

		case evt := <-ui.PollEvents():
			switch evt.ID {

			case "1", "2", "3", "4", "5", "6", "7", "8", "9", "0":
				if inputPrompt != "" {
					inputBuffer = inputBuffer + evt.ID
				}
				updateUI(playerData, nil)

			case "q", "<C-c>":
				return

			case "t":
				if inputPrompt != "" {
					continue
				}
				inputPrompt = "Enter target level"
				inputBuffer = ""
				updateUI(playerData, nil)

			case "<C-r>":
				updateTicker.Reset(0)

			case "<Down>":
				selectedMetric++
				if selectedMetric >= len(playerData.SkillValues) {
					selectedMetric = len(playerData.SkillValues) - 1
				}
				updateUI(playerData, nil)

			case "<Enter>":
				if inputPrompt == "" {
					continue
				}

				var tlvl int

				inputPrompt = ""
				if inputBuffer != "" {
					tlvl, err = strconv.Atoi(inputBuffer)
				}

				if err == nil {
					if tlvl < playerData.SkillValues[selectedMetric].Level {
						tlvl = 0
					}
					playerData.SkillValues[selectedMetric].TargetLevel = tlvl
				}

				updateUI(playerData, err)

			case "<Escape>":
				inputPrompt = ""
				updateUI(playerData, nil)

			case "<PageDown>":
				eventsPage++
				updateUI(playerData, nil)

			case "<PageUp>":
				eventsPage--
				updateUI(playerData, nil)

			case "<Resize>":
				ui.Clear()
				updateUI(playerData, nil)

			case "<Up>":
				selectedMetric--
				if selectedMetric < 0 {
					selectedMetric = 0
				}
				updateUI(playerData, nil)

			}

		case <-updateTicker.C:
			if playerData, err = getPlayerInfo(player, 20); err != nil {
				log.WithError(err).Error("Unable to fetch metrics")
			}

			if err := updateUI(playerData, err); err != nil {
				log.WithError(err).Error("Unable to update UI")
				return
			}
			updateTicker.Reset(time.Until(cron.Next(time.Now())))

			if err := playerInfoCache.storeCache(); err != nil {
				log.WithError(err).Error("Unable to write cache")
			}

		}
	}
}

func updateUI(playerData *playerInfo, err error) error {
	termWidth, termHeight := ui.TerminalDimensions()

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
		status.Text = fmt.Sprintf("Error: %s", err.Error())
		status.BorderStyle.Fg = ui.ColorRed

		if playerData == nil {
			return nil
		}
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
	//levelTable.TextAlignment = ui.AlignRight
	levelTable.RowStyles[0] = ui.Style{Fg: ui.ColorWhite, Modifier: ui.ModifierBold}
	levelTable.SetRect(0, 6, termWidth, 6+2+len(playerData.SkillValues)+1)
	levelTable.RowSeparator = false

	levelTable.ColumnWidths = []int{termWidth - 2 - 6 - 6 - 8 - 11 - 13 - 9, 6, 8, 11, 13, 9}

	levelTable.Rows = [][]string{{
		"  Skill",
		fmt.Sprintf("%*s", 6, "Level"),
		fmt.Sprintf("%*s", 8, "Level %"),
		fmt.Sprintf("%*s", 11, "Current XP"),
		fmt.Sprintf("%*s", 13, "XP remaining"),
		fmt.Sprintf("%*s", 9, "To Level"),
	}}
	for i, s := range playerData.SkillValues {
		var (
			name       = s.ID.String()
			remaining  = strconv.FormatInt(s.ID.Info().XPToNextLevel(s.XP/10), 10)
			percentage = strconv.FormatFloat(s.ID.Info().LevelPercentage(s.XP/10), 'f', 1, 64)
			target     = strconv.Itoa(s.Level + 1)

			rowStyle = ui.Style{Fg: ui.ColorWhite}
		)

		if i == selectedMetric {
			name = "> " + name
		} else {
			name = "  " + name
		}

		if s.TargetLevel > 0 {
			remaining = strconv.FormatInt(s.ID.Info().XPToTargetLevel(s.TargetLevel, s.XP/10), 10)
			percentage = strconv.FormatFloat(s.ID.Info().TargetPercentage(s.TargetLevel, s.XP/10), 'f', 1, 64)
			target = strconv.Itoa(s.TargetLevel)
			rowStyle.Fg = ui.ColorYellow
		}

		levelTable.Rows = append(levelTable.Rows, []string{
			name,
			fmt.Sprintf("%*s", 6, strconv.Itoa(s.Level)),
			fmt.Sprintf("%*s", 8, percentage),
			fmt.Sprintf("%*s", 11, strconv.FormatInt(s.XP/10, 10)),
			fmt.Sprintf("%*s", 13, remaining),
			fmt.Sprintf("%*s", 9, target),
		})

		if time.Since(s.Updated) < cfg.MarkerTime {
			rowStyle.Fg = ui.ColorGreen
		}

		levelTable.RowStyles[i+1] = rowStyle
	}
	ui.Render(levelTable)

	// Latest events
	events := widgets.NewTable()
	events.RowSeparator = false
	events.ColumnWidths = []int{12, termWidth - 3 - 12}
	events.SetRect(0, 6+2+len(playerData.SkillValues)+1, termWidth, termHeight-3)

	eventsPerPage := termHeight - 3 - (6 + 2 + len(playerData.SkillValues) + 1)
	eventPages := int(math.Ceil(float64(len(playerData.Activities)) / float64(eventsPerPage)))

	if eventsPage < 0 {
		eventsPage = 0
	}

	if eventsPage >= eventPages {
		eventsPage = eventPages - 1
	}

	events.Title = fmt.Sprintf("Event Log (%d / %d)", eventsPage+1, eventPages)

	for i, logEntry := range playerData.Activities[eventsPage*eventsPerPage:] {
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

	// Input box
	if inputPrompt != "" {
		input := widgets.NewParagraph()
		input.Title = inputPrompt
		input.Text = inputBuffer + "_"
		inputTop := int(math.Floor(float64(termHeight-3)) / 2)
		inputMargin := int(math.Floor(float64(termWidth) / 4))
		input.SetRect(inputMargin, inputTop, termWidth-inputMargin, inputTop+3)
		ui.Render(input)
	}

	return nil
}
