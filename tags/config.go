package tags

import (
	"fmt"
	"regexp"
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
		captureMap                  = make(map[uint]namedCaptureItem)
		syntaxTypeNames             []string
		docCaptureIndex             *uint
		nameCaptureIndex            *uint
		scopeCaptureIndex           *uint
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
		case "scope":
			scopeCaptureIndex = &ui
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

			syntaxTypeID := slices.IndexFunc(syntaxTypeNames, func(syntaxTypeName string) bool {
				return syntaxTypeName == kind
			})
			if syntaxTypeID == -1 {
				syntaxTypeNames = append(syntaxTypeNames, kind)
				syntaxTypeID = len(syntaxTypeNames) - 1
			}

			captureMap[ui] = namedCaptureItem{
				SyntaxTypeID: uint(syntaxTypeID),
				IsDefinition: isDefinition,
			}
		}
	}

	var patternInfo []patternInfoItem
	for patternIndex := range query.PatternCount() {
		var info patternInfoItem

		for _, predicate := range query.PropertyPredicates(patternIndex) {
			if !predicate.Positive && predicate.Property.Key == "local" {
				info.NameMustBeNonLocal = true
			}
		}
		info.LocalScopeInherits = true
		for _, property := range query.PropertySettings(patternIndex) {
			if property.Key == "local.scope-inherits" && (property.Value == nil || *property.Value == "false") {
				info.LocalScopeInherits = false
			}
		}

		if docCaptureIndex != nil {
			for _, predicate := range query.GeneralPredicates(patternIndex) {
				if len(predicate.Args) < 2 {
					continue
				}
				if firstArg := predicate.Args[0]; firstArg.CaptureId != nil && *firstArg.CaptureId == *docCaptureIndex {
					if predicate.Operator == "select-adjacent!" && predicate.Args[1].CaptureId != nil {
						info.DocsAdjacentCapture = &*predicate.Args[1].CaptureId
					} else if predicate.Operator == "strip!" && predicate.Args[1].String != nil {
						regex := regexp.MustCompile(*predicate.Args[1].String)
						info.DocStripRegex = regex
					}
				}
			}
		}
		patternInfo = append(patternInfo, info)
	}

	return &Configuration{
		Language:                    language,
		Query:                       query,
		syntaxTypeNames:             syntaxTypeNames,
		captureMap:                  captureMap,
		tagsPatternIndex:            tagsPatternIndex,
		docCaptureIndex:             docCaptureIndex,
		nameCaptureIndex:            nameCaptureIndex,
		scopeCaptureIndex:           scopeCaptureIndex,
		ignoreCaptureIndex:          ignoreCaptureIndex,
		localScopeCaptureIndex:      localScopeCaptureIndex,
		localDefinitionCaptureIndex: localDefinitionCaptureIndex,
		patternInfo:                 patternInfo,
	}, nil
}

type Configuration struct {
	Language                    *tree_sitter.Language
	Query                       *tree_sitter.Query
	syntaxTypeNames             []string
	captureMap                  map[uint]namedCaptureItem
	tagsPatternIndex            uint
	docCaptureIndex             *uint
	nameCaptureIndex            *uint
	scopeCaptureIndex           *uint
	ignoreCaptureIndex          *uint
	localScopeCaptureIndex      *uint
	localDefinitionCaptureIndex *uint
	patternInfo                 []patternInfoItem
}

func (c Configuration) SyntaxTypeNames() []string {
	return c.syntaxTypeNames
}

func (c Configuration) SyntaxTypeName(id uint) string {
	return c.syntaxTypeNames[id]
}

type namedCaptureItem struct {
	SyntaxTypeID uint
	IsDefinition bool
}

type patternInfoItem struct {
	DocsAdjacentCapture *uint
	LocalScopeInherits  bool
	NameMustBeNonLocal  bool
	DocStripRegex       *regexp.Regexp
}
