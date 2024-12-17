package highlight

import (
	"fmt"
	"slices"
	"strings"

	"github.com/tree-sitter/go-tree-sitter"
)

const (
	captureInjectionLanguage         = "injection.language"
	captureInjectionContent          = "injection.content"
	propertyInjectionCombined        = "injection.combined"
	propertyInjectionSelf            = "injection.self"
	propertyInjectionParent          = "injection.parent"
	propertyInjectionIncludeChildren = "injection.include-children"

	captureLocalScope           = "local.scope"
	captureLocalDefinition      = "local.definition"
	captureLocalDefinitionValue = "local.definition-value"
	captureLocalReference       = "local.reference"
	propertyLocal               = "local"
	propertyLocalScopeInherits  = "local.scope-inherits"
)

// StandardCaptureNames is a list of common capture names used in tree-sitter queries.
// This list is opinionated and may not align with the capture names used in a particular tree-sitter grammar.
var StandardCaptureNames = []string{
	"attribute",
	"boolean",
	"carriage-return",
	"comment",
	"comment.documentation",
	"constant",
	"constant.builtin",
	"constructor",
	"constructor.builtin",
	"embedded",
	"error",
	"escape",
	"function",
	"function.builtin",
	"keyword",
	"markup",
	"markup.bold",
	"markup.heading",
	"markup.italic",
	"markup.link",
	"markup.link.url",
	"markup.list",
	"markup.list.checked",
	"markup.list.numbered",
	"markup.list.unchecked",
	"markup.list.unnumbered",
	"markup.quote",
	"markup.raw",
	"markup.raw.block",
	"markup.raw.inline",
	"markup.strikethrough",
	"module",
	"number",
	"operator",
	"property",
	"property.builtin",
	"punctuation",
	"punctuation.bracket",
	"punctuation.delimiter",
	"punctuation.special",
	"string",
	"string.escape",
	"string.regexp",
	"string.special",
	"string.special.symbol",
	"tag",
	"type",
	"type.builtin",
	"variable",
	"variable.builtin",
	"variable.member",
	"variable.parameter",
}

// NewConfiguration creates a new highlight configuration from a [tree_sitter.Language] and a set of queries.
func NewConfiguration(language *tree_sitter.Language, languageName string, highlightsQuery []byte, injectionQuery []byte, localsQuery []byte) (*Configuration, error) {
	querySource := injectionQuery
	localsQueryOffset := uint(len(querySource))
	querySource = append(querySource, localsQuery...)
	highlightsQueryOffset := uint(len(querySource))
	querySource = append(querySource, highlightsQuery...)

	query, err := tree_sitter.NewQuery(language, string(querySource))
	if err != nil {
		return nil, fmt.Errorf("error creating query: %w", err)
	}

	localsPatternIndex := uint(0)
	highlightsPatternIndex := uint(0)
	for i := range query.PatternCount() {
		patternOffset := query.StartByteForPattern(i)
		if patternOffset < highlightsQueryOffset {
			if patternOffset < highlightsQueryOffset {
				highlightsPatternIndex++
			}
			if patternOffset < localsQueryOffset {
				localsPatternIndex++
			}
		}
	}

	combinedInjectionsQuery, err := tree_sitter.NewQuery(language, string(injectionQuery))
	if err != nil {
		return nil, fmt.Errorf("error creating combined injections query: %w", err)
	}

	var hasCombinedQueries bool
	for i := range localsPatternIndex {
		settings := combinedInjectionsQuery.PropertySettings(i)
		if slices.ContainsFunc(settings, func(setting tree_sitter.QueryProperty) bool {
			return setting.Key == propertyInjectionCombined
		}) {
			hasCombinedQueries = true
			query.DisablePattern(i)
		} else {
			combinedInjectionsQuery.DisablePattern(i)
		}
	}
	if !hasCombinedQueries {
		combinedInjectionsQuery = nil
	}

	nonLocalVariablePatterns := make([]bool, 0)
	for i := range query.PatternCount() {
		predicates := query.PropertyPredicates(i)
		if slices.ContainsFunc(predicates, func(predicate tree_sitter.PropertyPredicate) bool {
			return !predicate.Positive && predicate.Property.Key == propertyLocal
		}) {
			nonLocalVariablePatterns = append(nonLocalVariablePatterns, true)
		}
	}

	var (
		injectionContentCaptureIndex  *uint
		injectionLanguageCaptureIndex *uint
		localDefCaptureIndex          *uint
		localDefValueCaptureIndex     *uint
		localRefCaptureIndex          *uint
		localScopeCaptureIndex        *uint
	)

	for i, captureName := range query.CaptureNames() {
		ui := uint(i)
		switch captureName {
		case captureInjectionContent:
			injectionContentCaptureIndex = &ui
		case captureInjectionLanguage:
			injectionLanguageCaptureIndex = &ui
		case captureLocalDefinition:
			localDefCaptureIndex = &ui
		case captureLocalDefinitionValue:
			localDefValueCaptureIndex = &ui
		case captureLocalReference:
			localRefCaptureIndex = &ui
		case captureLocalScope:
			localScopeCaptureIndex = &ui
		}
	}

	highlightIndices := make([]*Highlight, len(query.CaptureNames()))
	return &Configuration{
		Language:                      language,
		LanguageName:                  languageName,
		Query:                         query,
		combinedInjectionsQuery:       combinedInjectionsQuery,
		localsPatternIndex:            localsPatternIndex,
		highlightsPatternIndex:        highlightsPatternIndex,
		highlightIndices:              highlightIndices,
		nonLocalVariablePatterns:      nonLocalVariablePatterns,
		injectionContentCaptureIndex:  injectionContentCaptureIndex,
		injectionLanguageCaptureIndex: injectionLanguageCaptureIndex,
		localScopeCaptureIndex:        localScopeCaptureIndex,
		localDefCaptureIndex:          localDefCaptureIndex,
		localDefValueCaptureIndex:     localDefValueCaptureIndex,
		localRefCaptureIndex:          localRefCaptureIndex,
	}, nil
}

type Configuration struct {
	Language                      *tree_sitter.Language
	LanguageName                  string
	Query                         *tree_sitter.Query
	combinedInjectionsQuery       *tree_sitter.Query
	localsPatternIndex            uint
	highlightsPatternIndex        uint
	highlightIndices              []*Highlight
	nonLocalVariablePatterns      []bool
	injectionContentCaptureIndex  *uint
	injectionLanguageCaptureIndex *uint
	localScopeCaptureIndex        *uint
	localDefCaptureIndex          *uint
	localDefValueCaptureIndex     *uint
	localRefCaptureIndex          *uint
}

// Copy returns a copy of the configuration.
func (c Configuration) Copy() Configuration {
	return Configuration{
		Language:                      c.Language,
		LanguageName:                  c.LanguageName,
		Query:                         c.Query,
		combinedInjectionsQuery:       c.combinedInjectionsQuery,
		localsPatternIndex:            c.localsPatternIndex,
		highlightsPatternIndex:        c.highlightsPatternIndex,
		highlightIndices:              c.highlightIndices,
		nonLocalVariablePatterns:      c.nonLocalVariablePatterns,
		injectionContentCaptureIndex:  c.injectionContentCaptureIndex,
		injectionLanguageCaptureIndex: c.injectionLanguageCaptureIndex,
		localScopeCaptureIndex:        c.localScopeCaptureIndex,
		localDefCaptureIndex:          c.localDefCaptureIndex,
		localDefValueCaptureIndex:     c.localDefValueCaptureIndex,
		localRefCaptureIndex:          c.localRefCaptureIndex,
	}
}

// Names gets a slice containing all the highlight names used in the configuration.
func (c Configuration) Names() []string {
	return c.Query.CaptureNames()
}

// Configure sets the list of recognized highlight names.
//
// Tree-sitter syntax-highlighting queries specify highlights in the form of dot-separated
// highlight names like `punctuation.bracket` and `function.method.builtin`. Consumers of
// these queries can choose to recognize highlights with different levels of specificity.
// For example, the string `function.builtin` will match against `function`
// and `function.builtin.constructor`, but will not match `function.method`.
//
// When highlighting, results are returned as `Highlight` values, which contain the index
// of the matched highlight this list of highlight names.
func (c *Configuration) Configure(recognizedNames []string) {
	highlightIndices := make([]*Highlight, len(c.Query.CaptureNames()))
	for i, captureName := range c.Query.CaptureNames() {
		for {
			j := slices.Index(recognizedNames, captureName)
			if j != -1 {
				index := Highlight(j)
				highlightIndices[i] = &index
				break
			}

			lastDot := strings.LastIndex(captureName, ".")
			if lastDot == -1 {
				break
			}
			captureName = captureName[:lastDot]
		}
	}
	c.highlightIndices = highlightIndices
}

// NonconformantCaptureNames returns the list of this configuration's capture names that are neither present in the
// list of predefined 'canonical' names nor start with an underscore (denoting 'private'
// captures used as part of capture internals).
func (c Configuration) NonconformantCaptureNames(captureNames []string) []string {
	if len(captureNames) == 0 {
		captureNames = StandardCaptureNames
	}

	var nonconformantNames []string
	for _, name := range c.Names() {
		if !(strings.HasPrefix(name, "_") || slices.Contains(captureNames, name)) {
			nonconformantNames = append(nonconformantNames, name)
		}
	}

	return nonconformantNames
}
