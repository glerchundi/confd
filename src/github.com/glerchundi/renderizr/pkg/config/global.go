package config

import (
	"time"
)

type GlobalConfig struct {
	Prefix         string
	Templates      []string
	Onetime        bool
	Watch          bool
	ResyncInterval time.Duration
	NoOp           bool
	KeepStageFile  bool
}

func NewGlobalConfig() *GlobalConfig {
	return &GlobalConfig{
		Prefix:         "/",
		Templates:      nil,
		Onetime:        false,
		Watch:          false,
		ResyncInterval: 60 * time.Second,
		NoOp:           false,
		KeepStageFile:  false,
	}
}
