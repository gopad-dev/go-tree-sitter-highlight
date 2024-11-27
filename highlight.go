package highlight

import (
	"context"
	"fmt"
	"iter"
	"slices"
	"strings"

	"github.com/tree-sitter/go-tree-sitter"
)

const (
	captureInjectionCombined        = "injection.combined"
	captureInjectionLanguage        = "injection.language"
	captureInjectionSelf            = "injection.self"
	captureInjectionParent          = "injection.parent"
	captureInjectionIncludeChildren = "injection.include-children"
	captureLocal                    = "local"
	captureLocalScopeInherits       = "local.scope-inherits"
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

type Highlight uint

type Event interface {
	highlightEvent()
}

type EventSource struct {
	Start uint
	End   uint
}

func (EventSource) highlightEvent() {}

type EventStart struct {
	Highlight    Highlight
	LanguageName string
}

func (EventStart) highlightEvent() {}

type EventEnd struct{}

func (EventEnd) highlightEvent() {}

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
			return setting.Key == captureInjectionCombined
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
			return !predicate.Positive && predicate.Property.Key == captureLocal
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
		case "injection.content":
			injectionContentCaptureIndex = &ui
		case "injection.language":
			injectionLanguageCaptureIndex = &ui
		case "local.definition":
			localDefCaptureIndex = &ui
		case "local.definition-value":
			localDefValueCaptureIndex = &ui
		case "local.reference":
			localRefCaptureIndex = &ui
		case "local.scope":
			localScopeCaptureIndex = &ui
		}
	}

	highlightIndices := make([]*Highlight, len(query.CaptureNames()))
	return &Configuration{
		Language:                      language,
		LanguageName:                  languageName,
		Query:                         query,
		CombinedInjectionsQuery:       combinedInjectionsQuery,
		LocalsPatternIndex:            localsPatternIndex,
		HighlightsPatternIndex:        highlightsPatternIndex,
		HighlightIndices:              highlightIndices,
		NonLocalVariablePatterns:      nonLocalVariablePatterns,
		InjectionContentCaptureIndex:  injectionContentCaptureIndex,
		InjectionLanguageCaptureIndex: injectionLanguageCaptureIndex,
		LocalScopeCaptureIndex:        localScopeCaptureIndex,
		LocalDefCaptureIndex:          localDefCaptureIndex,
		LocalDefValueCaptureIndex:     localDefValueCaptureIndex,
		LocalRefCaptureIndex:          localRefCaptureIndex,
	}, nil
}

type Configuration struct {
	Language                      *tree_sitter.Language
	LanguageName                  string
	Query                         *tree_sitter.Query
	CombinedInjectionsQuery       *tree_sitter.Query
	LocalsPatternIndex            uint
	HighlightsPatternIndex        uint
	HighlightIndices              []*Highlight
	NonLocalVariablePatterns      []bool
	InjectionContentCaptureIndex  *uint
	InjectionLanguageCaptureIndex *uint
	LocalScopeCaptureIndex        *uint
	LocalDefCaptureIndex          *uint
	LocalDefValueCaptureIndex     *uint
	LocalRefCaptureIndex          *uint
}

// Get a slice containing all of the highlight names used in the configuration.
func (c *Configuration) Names() []string {
	return c.Query.CaptureNames()
}

// Set the list of recognized highlight names.
//
// Tree-sitter syntax-highlighting queries specify highlights in the form of dot-separated
// highlight names like `punctuation.bracket` and `function.method.builtin`. Consumers of
// these queries can choose to recognize highlights with different levels of specificity.
// For example, the string `function.builtin` will match against `function.method.builtin`
// and `function.builtin.constructor`, but will not match `function.method`.
//
// When highlighting, results are returned as `Highlight` values, which contain the index
// of the matched highlight this list of highlight names.
func (c *Configuration) Configure(recognizedNames []string) {
	highlightIndices := make([]*Highlight, len(c.Query.CaptureNames()))
	for i, captureName := range c.Query.CaptureNames() {
		captureParts := strings.Split(captureName, ".")

		var bestIndex *Highlight
		var bestMatchLen int
		for j, recognizedName := range recognizedNames {
			var matchLen int
			matches := true
			for _, part := range strings.Split(recognizedName, ".") {
				matchLen++
				if !slices.Contains(captureParts, part) {
					matches = false
					break
				}
			}
			if matches && matchLen > bestMatchLen {
				index := Highlight(j)
				bestIndex = &index
				bestMatchLen = matchLen
			}
		}
		highlightIndices[i] = bestIndex
	}
	c.HighlightIndices = highlightIndices
}

// Return the list of this configuration's capture names that are neither present in the
// list of predefined 'canonical' names nor start with an underscore (denoting 'private'
// captures used as part of capture internals).
func (c *Configuration) NonconformantCaptureNames(captureNames []string) []string {
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

type localDef struct {
	Name      string
	Range     tree_sitter.Range
	Highlight *Highlight
}

type localScope struct {
	Inherits  bool
	Range     tree_sitter.Range
	LocalDefs []localDef
}

type InjectionCallback func(name string) *Configuration

type iterRange struct {
	Start uint
	End   uint
	Depth int
}

type highlightIter struct {
	Ctx                context.Context
	Source             []byte
	LanguageName       string
	ByteOffset         uint
	Highlighter        *Highlighter
	InjectionCallback  InjectionCallback
	Layers             []*iterLayer
	NextEvent          Event
	LastHighlightRange *iterRange
}

func (h *highlightIter) emitEvent(offset uint, event Event) (Event, error) {
	var result Event
	if h.ByteOffset < offset {
		result = EventSource{
			Start: h.ByteOffset,
			End:   offset,
		}
		h.ByteOffset = offset
		h.NextEvent = event
	} else {
		result = event
	}
	h.sortLayers()
	return result, nil
}

func (h *highlightIter) next() (Event, error) {
main:
	for {
		if h.NextEvent != nil {
			event := h.NextEvent
			h.NextEvent = nil
			return event, nil
		}

		// check for cancellation
		select {
		case <-h.Ctx.Done():
			return nil, h.Ctx.Err()
		default:
		}

		// If none of the layers have any more highlight boundaries, terminate.
		if len(h.Layers) == 0 {
			if h.ByteOffset < uint(len(h.Source)) {
				event := EventSource{
					Start: h.ByteOffset,
					End:   uint(len(h.Source)),
				}
				h.ByteOffset = uint(len(h.Source))
				return event, nil
			}
			return nil, nil
		}

		// Get the next capture from whichever layer has the earliest highlight boundary.
		var r tree_sitter.Range
		layer := h.Layers[0]
		if len(layer.Captures) > 0 {
			nextQueryCapture := layer.Captures[0]
			nextCapture := nextQueryCapture.Match.Captures[nextQueryCapture.Index]
			r = nextCapture.Node.Range()

			// If any previous highlight ends before this node starts, then before
			// processing this capture, emit the source code up until the end of the
			// previous highlight, and an end event for that highlight.
			if len(layer.HighlightEndStack) > 0 {
				endByte := layer.HighlightEndStack[len(layer.HighlightEndStack)-1]
				if endByte <= r.StartByte {
					layer.HighlightEndStack = layer.HighlightEndStack[:len(layer.HighlightEndStack)-1]
					return h.emitEvent(endByte, EventEnd{})
				}
			}
		} else {
			// If there are no more captures, then emit any remaining highlight end events.
			// And if there are none of those, then just advance to the end of the document.
			if len(layer.HighlightEndStack) > 0 {
				endByte := layer.HighlightEndStack[len(layer.HighlightEndStack)-1]
				layer.HighlightEndStack = layer.HighlightEndStack[:len(layer.HighlightEndStack)-1]
				return h.emitEvent(endByte, EventEnd{})
			}
			return h.emitEvent(uint(len(h.Source)), nil)
		}

		queryCapture := layer.Captures[0]
		layer.Captures = layer.Captures[1:]
		capture := queryCapture.Match.Captures[queryCapture.Index]

		// If this capture represents an injection, then process the injection.
		if queryCapture.Match.PatternIndex < layer.Config.LocalsPatternIndex {
			languageName, contentNode, includeChildren := injectionForMatch(layer.Config, h.LanguageName, queryCapture.Match, h.Source)

			// Explicitly remove this match so that none of its other captures will remain
			// in the stream of captures.
			queryCapture.Match.Remove()

			// If a language is found with the given name, then add a new language layer
			// to the highlighted document.
			if languageName != "" && contentNode != nil {
				newConfig := h.InjectionCallback(languageName)
				if newConfig != nil {
					ranges := intersectRanges(layer.Ranges, []tree_sitter.Node{*contentNode}, includeChildren)
					if len(ranges) > 0 {
						newLayers, err := newIterLayers(h.Ctx, h.Source, h.LanguageName, h.Highlighter, h.InjectionCallback, *newConfig, layer.Depth+1, ranges)
						if err != nil {
							return nil, err
						}
						for _, newLayer := range newLayers {
							h.insertLayer(newLayer)
						}
					}
				}
			}

			h.sortLayers()
			continue main
		}

		// Remove from the local scope stack any local scopes that have already ended.
		for r.StartByte > layer.ScopeStack[len(layer.ScopeStack)-1].Range.EndByte {
			layer.ScopeStack = layer.ScopeStack[:len(layer.ScopeStack)-1]
		}

		// If this capture is for tracking local variables, then process the
		// local variable info.
		var referenceHighlight *Highlight
		var definitionHighlight *Highlight
		for queryCapture.Match.PatternIndex < layer.Config.HighlightsPatternIndex {
			// If the node represents a local scope, push a new local scope onto
			// the scope stack.
			if layer.Config.LocalScopeCaptureIndex != nil && uint(capture.Index) == *layer.Config.LocalScopeCaptureIndex {
				definitionHighlight = nil
				scope := localScope{
					Inherits:  true,
					Range:     r,
					LocalDefs: nil,
				}
				for _, prop := range layer.Config.Query.PropertySettings(queryCapture.Match.PatternIndex) {
					if prop.Key == captureLocalScopeInherits {
						scope.Inherits = *prop.Value == "true"
					}
				}
				layer.ScopeStack = append(layer.ScopeStack, scope)
			} else if layer.Config.LocalDefCaptureIndex != nil && uint(capture.Index) == *layer.Config.LocalDefCaptureIndex {
				// If the node represents a definition, add a new definition to the
				// local scope at the top of the scope stack.
				referenceHighlight = nil
				definitionHighlight = nil
				scope := layer.ScopeStack[len(layer.ScopeStack)-1]

				var valueRange tree_sitter.Range
				for _, matchCapture := range queryCapture.Match.Captures {
					if layer.Config.LocalDefValueCaptureIndex != nil && uint(matchCapture.Index) == *layer.Config.LocalDefValueCaptureIndex {
						valueRange = matchCapture.Node.Range()
					}
				}

				if len(h.Source) > int(r.StartByte) && len(h.Source) > int(valueRange.EndByte) {
					name := string(h.Source[r.StartByte:r.EndByte])

					scope.LocalDefs = append(scope.LocalDefs, localDef{
						Name:      name,
						Range:     r,
						Highlight: nil,
					})
					definitionHighlight = scope.LocalDefs[len(scope.LocalDefs)-1].Highlight
				}
			} else if layer.Config.LocalRefCaptureIndex != nil && uint(capture.Index) == *layer.Config.LocalRefCaptureIndex && definitionHighlight == nil {
				// If the node represents a reference, then try to find the corresponding
				// definition in the scope stack.
				definitionHighlight = nil
				if len(h.Source) > int(r.StartByte) && len(h.Source) > int(r.EndByte) {
					name := string(h.Source[r.StartByte:r.EndByte])
					for _, scope := range slices.Backward(layer.ScopeStack) {
						var highlight *Highlight
						for _, def := range slices.Backward(scope.LocalDefs) {
							if def.Name == name && r.StartByte >= def.Range.EndByte {
								highlight = def.Highlight
							}
						}
						if highlight != nil {
							referenceHighlight = highlight
							break
						}
						if !scope.Inherits {
							break
						}
					}
				}
			}

			// Continue processing any additional matches for the same node.
			if len(layer.Captures) > 0 {
				nextQueryCapture := layer.Captures[0]
				nextCapture := nextQueryCapture.Match.Captures[nextQueryCapture.Index]
				if nextCapture.Node.Equals(capture.Node) {
					capture = nextCapture
					queryCapture = nextQueryCapture
					layer.Captures = layer.Captures[1:]
					continue
				}
			}

			h.sortLayers()
			continue main
		}

		// Otherwise, this capture must represent a highlight.
		// If this exact range has already been highlighted by an earlier pattern, or by
		// a different layer, then skip over this one.
		if h.LastHighlightRange != nil {
			lastRange := *h.LastHighlightRange
			if r.StartByte == lastRange.Start && r.EndByte == lastRange.End && layer.Depth < lastRange.Depth {
				h.sortLayers()
				continue main
			}
		}

		// Once a highlighting pattern is found for the current node, keep iterating over
		// any later highlighting patterns that also match this node and set the match to it.
		// Captures for a given node are ordered by pattern index, so these subsequent
		// captures are guaranteed to be for highlighting, not injections or
		// local variables.
		for len(layer.Captures) > 0 {
			nextQueryCapture := layer.Captures[0]
			nextCapture := nextQueryCapture.Match.Captures[nextQueryCapture.Index]
			if nextCapture.Node.Equals(capture.Node) {
				followingQueryCapture := nextQueryCapture
				layer.Captures = layer.Captures[1:]
				// If the current node was found to be a local variable, then ignore
				// the following match if it's a highlighting pattern that is disabled
				// for local variables.
				if definitionHighlight != nil || referenceHighlight != nil && layer.Config.NonLocalVariablePatterns[followingQueryCapture.Match.PatternIndex] {
					continue
				}

				queryCapture.Match.Remove()
				capture = nextCapture
				queryCapture = nextQueryCapture
			} else {
				break
			}
		}

		currentHighlight := layer.Config.HighlightIndices[uint(capture.Index)]

		// If this node represents a local definition, then store the current
		// highlight value on the local scope entry representing this node.
		if definitionHighlight != nil {
			definitionHighlight = currentHighlight
		}

		// Emit a scope start event and push the node's end position to the stack.
		highlight := referenceHighlight
		if highlight == nil {
			highlight = currentHighlight
		}
		if highlight != nil {
			h.LastHighlightRange = &iterRange{
				Start: r.StartByte,
				End:   r.EndByte,
				Depth: layer.Depth,
			}
			layer.HighlightEndStack = append(layer.HighlightEndStack, r.EndByte)
			return h.emitEvent(r.StartByte, EventStart{
				Highlight:    *highlight,
				LanguageName: layer.Config.LanguageName,
			})
		}

		h.sortLayers()
	}
}

func rotateLeft[T any](s []T, mid int) []T {
	return append(s[mid:], s[:mid]...)
}

func (h *highlightIter) sortLayers() {
	for len(h.Layers) > 0 {
		sortKey := h.Layers[0].sortKey()
		if sortKey != nil {
			var i int
			for i+1 < len(h.Layers) {
				nextOffset := h.Layers[i+1].sortKey()
				if nextOffset != nil {
					if nextOffset.Less(*sortKey) {
						i++
						continue
					}
				}
				break
			}
			if i > 0 {
				h.Layers = rotateLeft(h.Layers[:i], 1)
			}
			break
		}
		layer := h.Layers[0]
		h.Layers = h.Layers[1:]
		h.Highlighter.pushCursor(layer.Cursor)
	}
}

func (h *highlightIter) insertLayer(layer *iterLayer) {
	sortKey := layer.sortKey()
	if sortKey != nil {
		i := 1
		for i < len(h.Layers) {
			sortKeyI := h.Layers[i].sortKey()
			if sortKeyI != nil {
				if sortKeyI.Greater(*sortKey) {
					h.Layers = slices.Insert(h.Layers, i, layer)
					return
				}
				i++
			} else {
				h.Layers = slices.Delete(h.Layers, i, i+1)
			}
		}
		h.Layers = append(h.Layers, layer)
	}
}

type highlightQueueItem struct {
	config Configuration
	depth  int
	ranges []tree_sitter.Range
}

type injectionItem struct {
	languageName    string
	nodes           []tree_sitter.Node
	includeChildren bool
}

func newIterLayers(
	ctx context.Context,
	source []byte,
	parentName string,
	highlighter *Highlighter,
	injectionCallback InjectionCallback,
	config Configuration,
	depth int,
	ranges []tree_sitter.Range,
) ([]*iterLayer, error) {
	var result []*iterLayer
	var queue []highlightQueueItem
	for {
		if err := highlighter.Parser.SetIncludedRanges(ranges); err == nil {
			if err = highlighter.Parser.SetLanguage(config.Language); err != nil {
				return nil, fmt.Errorf("error setting language: %w", err)
			}
			tree := highlighter.Parser.ParseCtx(ctx, source, nil)

			cursor := highlighter.popCursor()

			// Process combined injections.
			if config.CombinedInjectionsQuery != nil {
				injectionsByPatternIndex := make([]injectionItem, config.CombinedInjectionsQuery.PatternCount())

				matches := cursor.Matches(config.CombinedInjectionsQuery, tree.RootNode(), source)
				for {
					match := matches.Next()
					if match == nil {
						break
					}

					languageName, contentNode, includeChildren := injectionForMatch(config, parentName, *match, source)

					if languageName == "" {
						injectionsByPatternIndex[match.PatternIndex].languageName = languageName
					}
					if contentNode != nil {
						injectionsByPatternIndex[match.PatternIndex].nodes = append(injectionsByPatternIndex[match.PatternIndex].nodes, *contentNode)
					}
					injectionsByPatternIndex[match.PatternIndex].includeChildren = includeChildren
				}

				for _, injection := range injectionsByPatternIndex {
					if injection.languageName != "" && len(injection.nodes) > 0 {
						nextConfig := injectionCallback(injection.languageName)
						if nextConfig != nil {
							nextRanges := intersectRanges(ranges, injection.nodes, injection.includeChildren)
							if len(nextRanges) > 0 {
								queue = append(queue, highlightQueueItem{
									config: *nextConfig,
									depth:  depth + 1,
									ranges: nextRanges,
								})
							}
						}

					}
				}

			}

			captures := make([]_queryCapture, 0)
			queryCaptures := cursor.Captures(config.Query, tree.RootNode(), source)
			for {
				match, i := queryCaptures.Next()
				if match == nil {
					break
				}

				match.Captures = slices.Clone(match.Captures)
				captures = append(captures, _queryCapture{
					Match: *match,
					Index: i,
				})
			}

			result = append(result, &iterLayer{
				Tree:              tree,
				Cursor:            cursor,
				Config:            config,
				HighlightEndStack: nil,
				ScopeStack: []localScope{
					{
						Inherits: false,
						Range: tree_sitter.Range{
							StartByte: 0,
							StartPoint: tree_sitter.Point{
								Row:    0,
								Column: 0,
							},
							EndByte: ^uint(0),
							EndPoint: tree_sitter.Point{
								Row:    ^uint(0),
								Column: ^uint(0),
							},
						},
						LocalDefs: nil,
					},
				},
				Captures: captures,
				Ranges:   ranges,
				Depth:    depth,
			})
		}

		if len(queue) == 0 {
			break
		}

		var next highlightQueueItem
		next, queue = queue[0], append(queue, queue[1:]...)

		config = next.config
		depth = next.depth
		ranges = next.ranges
	}

	return result, nil
}

func intersectRanges(parentRanges []tree_sitter.Range, nodes []tree_sitter.Node, includesChildren bool) []tree_sitter.Range {
	cursor := nodes[0].Walk()
	results := make([]tree_sitter.Range, 0)
	if len(parentRanges) == 0 {
		panic("parentRanges must not be empty")
	}
	parentRange := parentRanges[0]
	parentRanges = parentRanges[1:]

	for _, node := range nodes {
		precedingRange := tree_sitter.Range{
			StartByte: 0,
			StartPoint: tree_sitter.Point{
				Row:    0,
				Column: 0,
			},
			EndByte:  node.StartByte(),
			EndPoint: node.StartPosition(),
		}
		followingRange := tree_sitter.Range{
			StartByte:  node.EndByte(),
			StartPoint: node.EndPosition(),
			EndByte:    ^uint(0),
			EndPoint: tree_sitter.Point{
				Row:    ^uint(0),
				Column: ^uint(0),
			},
		}

		excludedRanges := make([]tree_sitter.Range, 0)
		cursor.Reset(node)
		cursor.GotoFirstChild()
		for range node.ChildCount() {
			child := cursor.Node()
			cursor.GotoNextSibling()
			if !includesChildren {
				excludedRanges = append(excludedRanges, tree_sitter.Range{
					StartByte:  child.StartByte(),
					StartPoint: child.StartPosition(),
					EndByte:    child.EndByte(),
					EndPoint:   child.EndPosition(),
				})
			}
		}
		excludedRanges = append(excludedRanges, followingRange)

		for _, excludedRange := range excludedRanges {
			r := tree_sitter.Range{
				StartByte:  precedingRange.EndByte,
				StartPoint: precedingRange.EndPoint,
				EndByte:    excludedRange.StartByte,
				EndPoint:   excludedRange.StartPoint,
			}
			precedingRange = excludedRange

			if r.EndByte < parentRange.StartByte {
				continue
			}

			for parentRange.StartByte <= r.EndByte {
				if parentRange.EndByte > r.StartByte {
					if r.StartByte < parentRange.StartByte {
						r.StartByte = parentRange.StartByte
						r.StartPoint = parentRange.StartPoint
					}

					if parentRange.EndByte < r.EndByte {
						if r.StartByte < parentRange.EndByte {
							results = append(results, tree_sitter.Range{
								StartByte:  r.StartByte,
								StartPoint: r.StartPoint,
								EndByte:    parentRange.EndByte,
								EndPoint:   parentRange.EndPoint,
							})
						}
						r.StartByte = parentRange.EndByte
						r.StartPoint = parentRange.EndPoint
					} else {
						if r.StartByte < r.EndByte {
							results = append(results, r)
						}
						break
					}
				}

				if len(parentRanges) > 0 {
					parentRange = parentRanges[0]
					parentRanges = parentRanges[1:]
				} else {
					return results
				}
			}
		}
	}

	return results
}

func injectionForMatch(config Configuration, parentName string, match tree_sitter.QueryMatch, source []byte) (string, *tree_sitter.Node, bool) {
	if config.InjectionContentCaptureIndex == nil || config.InjectionLanguageCaptureIndex == nil {
		return "", nil, false
	}

	contentCaptureIndex := *config.InjectionContentCaptureIndex
	languageCaptureIndex := *config.InjectionLanguageCaptureIndex

	var languageName string
	var contentNode *tree_sitter.Node

	for _, capture := range match.Captures {
		index := uint(capture.Index)
		if index == languageCaptureIndex {
			languageName = capture.Node.Utf8Text(source)
		} else if index == contentCaptureIndex {
			contentNode = &capture.Node
		}
	}

	var includeChildren bool
	for _, property := range config.Query.PropertySettings(match.PatternIndex) {
		switch property.Key {
		case captureInjectionLanguage:
			if languageName == "" {
				languageName = *property.Value
			}
		case captureInjectionSelf:
			if languageName == "" {
				languageName = config.LanguageName
			}
		case captureInjectionParent:
			if languageName == "" {
				languageName = parentName
			}
		case captureInjectionIncludeChildren:
			includeChildren = true
		}
	}

	return languageName, contentNode, includeChildren
}

type _queryCapture struct {
	Match tree_sitter.QueryMatch
	Index uint
}

type iterLayer struct {
	Tree              *tree_sitter.Tree
	Cursor            *tree_sitter.QueryCursor
	Config            Configuration
	HighlightEndStack []uint
	ScopeStack        []localScope
	Captures          []_queryCapture
	Ranges            []tree_sitter.Range
	Depth             int
}

type sortKeyResult struct {
	offset uint
	start  bool
	depth  int
}

func (k sortKeyResult) Less(other sortKeyResult) bool {
	if k.offset < other.offset {
		return true
	}
	if k.offset > other.offset {
		return false
	}
	if !k.start && other.start {
		return true
	}
	if k.start && !other.start {
		return false
	}
	return k.depth < other.depth
}

func (k sortKeyResult) Greater(other sortKeyResult) bool {
	if k.offset > other.offset {
		return true
	}
	if k.offset < other.offset {
		return false
	}
	if k.start && !other.start {
		return true
	}
	if !k.start && other.start {
		return false
	}
	return k.depth > other.depth
}

func (h *iterLayer) sortKey() *sortKeyResult {
	depth := -h.Depth

	var nextStart *uint
	if len(h.Captures) > 0 {
		capture := h.Captures[0]
		startByte := capture.Match.Captures[capture.Index].Node.StartByte()
		nextStart = &startByte
	}

	var nextEnd *uint
	if len(h.HighlightEndStack) > 0 {
		endByte := h.HighlightEndStack[len(h.HighlightEndStack)-1]
		nextEnd = &endByte
	}

	switch {
	case nextStart != nil && nextEnd != nil:
		if *nextStart < *nextEnd {
			return &sortKeyResult{
				offset: *nextStart,
				start:  true,
				depth:  depth,
			}
		} else {
			return &sortKeyResult{
				offset: *nextEnd,
				start:  false,
				depth:  depth,
			}
		}
	case nextStart != nil:
		return &sortKeyResult{
			offset: *nextStart,
			start:  true,
			depth:  depth,
		}
	case nextEnd != nil:
		return &sortKeyResult{
			offset: *nextEnd,
			start:  false,
			depth:  depth,
		}
	default:
		return nil
	}
}

func New() *Highlighter {
	return &Highlighter{
		Parser: tree_sitter.NewParser(),
	}
}

type Highlighter struct {
	Parser  *tree_sitter.Parser
	cursors []*tree_sitter.QueryCursor
}

func (h *Highlighter) pushCursor(cursor *tree_sitter.QueryCursor) {
	h.cursors = append(h.cursors, cursor)
}

func (h *Highlighter) popCursor() *tree_sitter.QueryCursor {
	if len(h.cursors) == 0 {
		return tree_sitter.NewQueryCursor()
	}

	cursor := h.cursors[len(h.cursors)-1]
	h.cursors = h.cursors[:len(h.cursors)-1]
	return cursor
}

func (h *Highlighter) Highlight(
	ctx context.Context,
	cfg Configuration,
	source []byte,
	injectionCallback InjectionCallback,
) iter.Seq2[Event, error] {
	layers, err := newIterLayers(ctx, source, "", h, injectionCallback, cfg, 0, []tree_sitter.Range{
		{
			StartByte: 0,
			EndByte:   ^uint(0),
			StartPoint: tree_sitter.Point{
				Row:    0,
				Column: 0,
			},
			EndPoint: tree_sitter.Point{
				Row:    ^uint(0),
				Column: ^uint(0),
			},
		},
	})
	if err != nil {
		return func(yield func(Event, error) bool) {
			yield(nil, err)
		}
	}

	hIter := &highlightIter{
		Ctx:                ctx,
		Source:             source,
		LanguageName:       cfg.LanguageName,
		ByteOffset:         0,
		Highlighter:        h,
		InjectionCallback:  injectionCallback,
		Layers:             layers,
		NextEvent:          nil,
		LastHighlightRange: nil,
	}
	hIter.sortLayers()

	return func(yield func(Event, error) bool) {
		for {
			event, err := hIter.next()
			// error we are done
			if err != nil {
				yield(nil, err)
				return
			}

			// we're done if there are no more events
			if event == nil {
				return
			}

			// yield the event
			if !yield(event, nil) {
				return
			}
		}
	}
}
