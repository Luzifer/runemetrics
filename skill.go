package main

type skillInfo struct {
	id         uint
	name       string
	color      string
	maxLevel   int
	skillCurve string // ???
}

func (s skillInfo) LevelFromXP(xp int64) int {
	levelTree := levels
	if s.skillCurve != "" {
		levelTree = masterLevels
	}

	for i := 1; i <= len(levelTree); i++ {
		if levelTree[i] > xp {
			return i - 1
		}
	}

	return 1
}

func (s skillInfo) LevelXP(level int) int64 {
	levelTree := levels
	if s.skillCurve != "" {
		levelTree = masterLevels
	}

	return levelTree[level]
}

func (s skillInfo) LevelPercentage(xp int64) float64 {
	var (
		level  = s.LevelFromXP(xp)
		xpCurr = float64(s.LevelXP(level))
		xpNext = float64(s.LevelXP(level + 1))
	)

	return (float64(xp) - xpCurr) / (xpNext - xpCurr) * 100
}

func (s skillInfo) TargetPercentage(level int, xp int64) float64 {
	var xpNext = float64(s.LevelXP(level))
	return float64(xp) / xpNext * 100
}

func (s skillInfo) XPToNextLevel(xp int64) int64 {
	level := s.LevelFromXP(xp)
	return s.LevelXP(level+1) - xp
}

func (s skillInfo) XPToTargetLevel(level int, xp int64) int64 {
	return s.LevelXP(level) - xp
}

var skillList = []skillInfo{
	{
		id:    0,
		name:  "Attack",
		color: "#981414",
	}, {
		id:    1,
		name:  "Defence",
		color: "#147e98",
	}, {
		id:    2,
		name:  "Strength",
		color: "#13b787",
	}, {
		id:    3,
		name:  "Constitution",
		color: "#AACEDA",
	}, {
		id:    4,
		name:  "Ranged",
		color: "#13b751",
	}, {
		id:    5,
		name:  "Prayer",
		color: "#6dbff2",
	}, {
		id:    6,
		name:  "Magic",
		color: "#c3e3dc",
	}, {
		id:    7,
		name:  "Cooking",
		color: "#553285",
	}, {
		id:    8,
		name:  "Woodcutting",
		color: "#7e4f35",
	}, {
		id:    9,
		name:  "Fletching",
		color: "#149893",
	}, {
		id:    10,
		name:  "Fishing",
		color: "#3e70b9",
	}, {
		id:    11,
		name:  "Firemaking",
		color: "#f75f28",
	}, {
		id:    12,
		name:  "Crafting",
		color: "#b6952c",
	}, {
		id:    13,
		name:  "Smithing",
		color: "#65887e",
	}, {
		id:    14,
		name:  "Mining",
		color: "#56495e",
	}, {
		id:    15,
		name:  "Herblore",
		color: "#12453a",
	}, {
		id:    16,
		name:  "Agility",
		color: "#284A95",
	}, {
		id:    17,
		name:  "Thieving",
		color: "#36175e",
	}, {
		id:    18,
		name:  "Slayer",
		color: "#48412f",
	}, {
		id:    19,
		name:  "Farming",
		color: "#1f7d54",
	}, {
		id:    20,
		name:  "Runecrafting",
		color: "#d7eba3",
	}, {
		id:    21,
		name:  "Hunter",
		color: "#c38b4e",
	}, {
		id:    22,
		name:  "Construction",
		color: "#a8babc",
	}, {
		id:    23,
		name:  "Summoning",
		color: "#DEA1B0",
	}, {
		id:       24,
		name:     "Dungeoneering",
		color:    "#723920",
		maxLevel: 120,
	}, {
		id:    25,
		name:  "Divination",
		color: "#943fba",
	}, {
		id:         26,
		name:       "Invention",
		color:      "#f7b528",
		skillCurve: "master",
	},
}

type skillID uint

func (s skillID) String() string {
	for _, se := range skillList {
		if se.id == uint(s) {
			return se.name
		}
	}

	return ""
}

func (s skillID) Info() skillInfo {
	for _, se := range skillList {
		if se.id == uint(s) {
			return se
		}
	}

	return skillInfo{}
}
