package assetrepo

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
)

var repoIdPattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

// Repos returns the configured repos sorted by id.
func (c Config) Repos() ([]AssetRepo, error) {
	repos := make([]AssetRepo, 0, len(c.RepoConfigs))

	for _, id := range slices.Sorted(maps.Keys(c.RepoConfigs)) {
		if !repoIdPattern.MatchString(id) {
			return nil, fmt.Errorf("asset_repo: repo id %q has to match %s", id, repoIdPattern)
		}

		repo := c.RepoConfigs[id]
		repo.Id = id
		repos = append(repos, repo)
	}

	return repos, nil
}
