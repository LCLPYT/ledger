package assetrepo

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/goccy/go-yaml"
)

var repoIdPattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

type Conf struct {
	Repos []AssetRepo `yaml:"repos"`
}

func ReadConfig(path string) (Conf, error) {
	content, err := os.ReadFile(path)

	if err != nil {
		return Conf{}, fmt.Errorf("asset_repo_conf: failed to open config file %s: %w", path, err)
	}

	cfg := Conf{}

	err = yaml.Unmarshal(content, &cfg)

	if err != nil {
		return Conf{}, fmt.Errorf("asset_repo_conf: failed to parse asset repo config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return Conf{}, fmt.Errorf("asset_repo_conf: invalid asset repo config %s: %w", path, err)
	}

	dir := filepath.Dir(path)

	for i, repo := range cfg.Repos {
		if !filepath.IsAbs(repo.Path) {
			cfg.Repos[i].Path = filepath.Join(dir, repo.Path)
		}
	}

	return cfg, nil
}

func (c Conf) Validate() error {
	seen := make(map[string]struct{}, len(c.Repos))

	for i, repo := range c.Repos {
		if !repoIdPattern.MatchString(repo.Id) {
			return fmt.Errorf("repo #%d: id %q has to match %s", i, repo.Id, repoIdPattern)
		}

		if _, ok := seen[repo.Id]; ok {
			return fmt.Errorf("repo #%d: duplicate id %q", i, repo.Id)
		}

		seen[repo.Id] = struct{}{}

		if repo.Path == "" {
			return fmt.Errorf("repo %s: path is required", repo.Id)
		}
	}

	return nil
}
