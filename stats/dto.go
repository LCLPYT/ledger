package stats

// Ref addresses a registered definition version.
type Ref struct {
	Key     string `json:"key"`
	Version string `json:"version"`
}

type Manifest struct {
	Definitions []DefinitionInput `json:"definitions"`
}

type DefinitionInput struct {
	Kind     string      `json:"kind"`
	Key      string      `json:"key"`
	Name     string      `json:"name"`
	Group    string      `json:"group"`
	Version  string      `json:"version"`
	GameType string      `json:"game_type"`
	Stats    []StatInput `json:"stats"`
}

type StatInput struct {
	Key            string `json:"key"`
	Name           string `json:"name"`
	Scope          string `json:"scope"`
	Type           string `json:"type"`
	Unit           string `json:"unit"`
	HigherIsBetter *bool  `json:"higher_is_better"`
	Display        *bool  `json:"display"`
}

type RegisterResult struct {
	Definitions []DefinitionResult `json:"definitions"`
	Created     int                `json:"created"`
	Unchanged   int                `json:"unchanged"`
}

type DefinitionResult struct {
	Kind      string `json:"kind"`
	Key       string `json:"key"`
	Version   string `json:"version"`
	VersionID int64  `json:"version_id"`
	Status    string `json:"status"`
}

const (
	StatusCreated   = "created"
	StatusUnchanged = "unchanged"
)
