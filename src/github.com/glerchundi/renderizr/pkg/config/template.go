package config

type TemplateConfigFile struct {
	TemplateConfig TemplateConfig `toml:"template"`
}

type TemplateConfig struct {
	Src           string
	Dest          string
	Uid           int
	Gid           int
	Mode          string
	KeepStageFile bool
	Prefix        string
	CheckCmd      string
	ReloadCmd     string
}

func NewTemplateConfig() *TemplateConfig {
	return &TemplateConfig{
		Src:           "",
		Dest:          "",
		Uid:           0,
		Gid:           0,
		Mode:          "0644",
		KeepStageFile: false,
		Prefix:        "/",
		CheckCmd:      "",
		ReloadCmd:     "",
	}
}
