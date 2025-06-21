package ciconfig

import "regexp"

type GenericCiStep struct {
	Name                 string
	Image                string
	Commands             []string
	Environment          map[string]string
	Dependencies         []regexp.Regexp
	ResolvedDependencies []string

	// Circle-only configs
	IsCheckoutStep        bool
	WorkspacePersistPaths []string
	NoWorkspace           bool
	NeedsDocker           bool
	CacheRestoreKeys      []string
	CacheSaveKey          string
	CacheSavePaths        []string
}
