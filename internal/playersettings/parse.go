package playersettings

import (
	"regexp"
	"strings"

	"github.com/gofrs/uuid/v5"
	"github.com/quadrubo/golib/resourcename"
)

const (
	namePattern   = "players/*/settings/*"
	parentPattern = "players/*"

	maxPathLength = 255
)

var (
	playerIDRegex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	pathRegex     = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}(\.[a-z][a-z0-9_]{0,62}){0,15}$`)
)

func ValidPath(path string) bool {
	return len(path) <= maxPathLength && pathRegex.MatchString(path)
}

func FormatName(playerID uuid.UUID, path string) string {
	return FormatParent(playerID) + "/settings/" + path
}

func FormatParent(playerID uuid.UUID) string {
	return "players/" + playerID.String()
}

// ParseName returns the player id and setting path of a setting resource name.
func ParseName(name string) (uuid.UUID, string, error) {
	segments := strings.SplitN(name, "/", 4)

	if len(segments) != 4 || segments[0] != "players" || segments[2] != "settings" || !ValidPath(segments[3]) {
		return uuid.Nil, "", &resourcename.Error{Name: name, Pattern: namePattern}
	}

	playerID, ok := parsePlayerID(segments[1])

	if !ok {
		return uuid.Nil, "", &resourcename.Error{Name: name, Pattern: namePattern}
	}

	return playerID, segments[3], nil
}

func ParseParent(parent string) (uuid.UUID, error) {
	id, ok := strings.CutPrefix(parent, "players/")

	if !ok {
		return uuid.Nil, &resourcename.Error{Name: parent, Pattern: parentPattern}
	}

	playerID, ok := parsePlayerID(id)

	if !ok {
		return uuid.Nil, &resourcename.Error{Name: parent, Pattern: parentPattern}
	}

	return playerID, nil
}

func parsePlayerID(id string) (uuid.UUID, bool) {
	if !playerIDRegex.MatchString(id) {
		return uuid.Nil, false
	}

	playerID, err := uuid.FromString(id)

	return playerID, err == nil
}

func ancestors(path string) []string {
	var result []string

	for i := range len(path) {
		if path[i] == '.' {
			result = append(result, path[:i])
		}
	}

	return result
}
