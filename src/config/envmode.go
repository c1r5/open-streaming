package config

type EnvType int

const (
	PRD EnvType = iota
	DEV
)

var envTypeName = map[EnvType]string{
	PRD: "PRD",
	DEV: "DEV",
}

func (et EnvType) String() string {
	return envTypeName[et]
}
