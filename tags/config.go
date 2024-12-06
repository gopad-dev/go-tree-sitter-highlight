package tags

import (
	"fmt"
	"slices"
	"strings"

	"github.com/tree-sitter/go-tree-sitter"
)

func NewConfiguration(language *tree_sitter.Language, tagsQuery []byte, localsQuery []byte) (*Configuration, error) {
	querySource := localsQuery
	tagsQueryOffset := uint(len(querySource))
	querySource = append(querySource, tagsQuery...)

	query, err := tree_sitter.NewQuery(language, string(querySource))
	if err != nil {
		return nil, err
	}

	tagsPatternIndex := uint(0)
	for i := range query.PatternCount() {
		patternOffset := query.StartByteForPattern(i)
		if patternOffset < tagsQueryOffset {
			tagsPatternIndex++
		}
	}

	var (
		docCaptureIndex             *uint
		nameCaptureIndex            *uint
		ignoreCaptureIndex          *uint
		localScopeCaptureIndex      *uint
		localDefinitionCaptureIndex *uint
	)
	for i, captureName := range query.CaptureNames() {
		ui := uint(i)

		switch captureName {
		case "doc":
			docCaptureIndex = &ui
		case "name":
			nameCaptureIndex = &ui
		case "ignore":
			ignoreCaptureIndex = &ui
		case "local.scope":
			localScopeCaptureIndex = &ui
		case "local.definition":
			localDefinitionCaptureIndex = &ui
		case "local.reference", "":
			continue
		default:
			var isDefinition bool
			var kind string
			if strings.HasPrefix(captureName, "definition.") {
				isDefinition = true
				kind = strings.TrimPrefix(captureName, "definition.")
			} else if strings.HasPrefix(captureName, "reference.") {
				kind = strings.TrimPrefix(captureName, "reference.")
			} else {
				return nil, fmt.Errorf("unexpected capture name: %s", captureName)
			}

		}
	}

	return &Configuration{}, nil
}

type Configuration struct {
	Language                    *tree_sitter.Language
	Query                       *tree_sitter.Query
	TagsPatternIndex            uint
	DocCaptureIndex             *uint
	NameCaptureIndex            *uint
	IgnoreCaptureIndex          *uint
	LocalScopeCaptureIndex      *uint
	LocalDefinitionCaptureIndex *uint
}
