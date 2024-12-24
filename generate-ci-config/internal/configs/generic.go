package configs

import "regexp"

type GenericCiStep struct {
	Name                 string
	Image                string
	Commands             []string
	Environment          map[string]string
	Dependencies         []regexp.Regexp
	ResolvedDependencies []string

	// used for Circle only
	IsCheckoutStep bool
	NeedsDocker    bool
	SkipPersist    bool
}
