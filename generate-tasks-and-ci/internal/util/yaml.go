package util

import "go.yaml.in/yaml/v4"

var YAMLOutputOptions = []yaml.Option{
	yaml.WithIndent(2),
	yaml.WithLineWidth(-1),
	yaml.WithCompactSeqIndent(false),
	yaml.WithFlowSimpleCollections(false),
	yaml.WithExplicitStart(true),
	yaml.WithQuotePreference(yaml.QuoteDouble),
}
