package highlight

import (
	"context"
	"iter"

	"github.com/tree-sitter/go-tree-sitter"
)

// Highlight represents the index of a capture name.
type Highlight uint

const DefaultHighlight = Highlight(^uint(0))

// Event is an interface that represents a highlight event.
// Possible implementations are:
// - [EventLayerStart]
// - [EventLayerEnd]
// - [EventCaptureStart]
// - [EventCaptureEnd]
// - [EventSource]
type Event interface {
	highlightEvent()
}

// EventSource is emitted when a source code range is highlighted.
type EventSource struct {
	StartByte uint
	EndByte   uint
}

func (EventSource) highlightEvent() {}

// EventLayerStart is emitted when a language injection starts.
type EventLayerStart struct {
	// LanguageName is the name of the language that is being injected.
	LanguageName string
	// Range is the range of the source code that is being injected.
	Range tree_sitter.Range
}

func (EventLayerStart) highlightEvent() {}

// EventLayerEnd is emitted when a language injection ends.
type EventLayerEnd struct{}

func (EventLayerEnd) highlightEvent() {}

type EventFoldStart struct {
	Range tree_sitter.Range
}

func (EventFoldStart) highlightEvent() {}

type EventFoldEnd struct{}

func (EventFoldEnd) highlightEvent() {}

// EventCaptureStart is emitted when a highlight region starts.
type EventCaptureStart struct {
	// Highlight is the capture name of the highlight.
	Highlight Highlight
}

func (EventCaptureStart) highlightEvent() {}

// EventCaptureEnd is emitted when a highlight region ends.
type EventCaptureEnd struct{}

func (EventCaptureEnd) highlightEvent() {}

// InjectionCallback is called when a language injection is found to load the configuration for the injected language.
type InjectionCallback func(languageName string) *Configuration

// New returns a new highlighter. The highlighter is not thread-safe and should not be shared between goroutines,
// but it can be reused to highlight multiple source code snippets.
func New() *Highlighter {
	return &Highlighter{
		Parser: tree_sitter.NewParser(),
	}
}

// Highlighter is a syntax highlighter that uses tree-sitter to parse source code and apply syntax highlighting. It is not thread-safe.
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

// Highlight highlights the given source code using the given configuration. The source code is expected to be UTF-8 encoded.
// The function returns an [iter.Seq2[Event, error]] that yields the highlight events or an error.
func (h *Highlighter) Highlight(ctx context.Context, cfg Configuration, source []byte, injectionCallback InjectionCallback) iter.Seq2[Event, error] {
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

	i := &iterator{
		Ctx:                ctx,
		Source:             source,
		LanguageName:       cfg.LanguageName,
		ByteOffset:         0,
		Highlighter:        h,
		InjectionCallback:  injectionCallback,
		Layers:             layers,
		NextEvents:         nil,
		LastHighlightRange: nil,
	}
	i.sortLayers()

	return func(yield func(Event, error) bool) {
		for {
			event, err := i.next()
			if err != nil {
				yield(nil, err)

				// error we are done
				return
			}

			if event == nil {
				// we're done if there are no more events
				return
			}

			// yield the event
			if !yield(event, nil) {
				// if the consumer returns false we can stop
				return
			}
		}
	}
}

// Compute the ranges that should be included when parsing an injection.
// This takes into account three things:
//   - `parent_ranges` - The ranges must all fall within the *current* layer's ranges.
//   - `nodes` - Every injection takes place within a set of nodes. The injection ranges are the
//     ranges of those nodes.
//   - `includes_children` - For some injections, the content nodes' children should be excluded
//     from the nested document, so that only the content nodes' *own* content is reparsed. For
//     other injections, the content nodes' entire ranges should be reparsed, including the ranges
//     of their children.
func intersectRanges(parentRanges []tree_sitter.Range, nodes []tree_sitter.Node, includesChildren bool) []tree_sitter.Range {
	return []tree_sitter.Range{
		nodes[0].Range(),
	}

	// TODO: investigate why this is not working, ported from: https://github.com/tree-sitter/tree-sitter/blob/e445532a1fea3b1dda93cee61c534f5b9acc9c16/highlight/src/lib.rs#L638 (and probably wrong lol)
	// if len(parentRanges) == 0 {
	//	panic("Layers should only be constructed with non-empty ranges")
	// }
	//
	// parentRange := parentRanges[0]
	// parentRanges = parentRanges[1:]
	//
	// cursor := nodes[0].Walk()
	// defer cursor.Close()
	//
	// var results []tree_sitter.Range
	// for _, node := range nodes {
	//	precedingRange := tree_sitter.Range{
	//		StartByte: 0,
	//		StartPoint: tree_sitter.Point{
	//			Row:    0,
	//			Column: 0,
	//		},
	//		EndByte:  node.StartByte(),
	//		EndPoint: node.StartPosition(),
	//	}
	//	followingRange := tree_sitter.Range{
	//		StartByte:  node.EndByte(),
	//		StartPoint: node.EndPosition(),
	//		EndByte:    ^uint(0),
	//		EndPoint: tree_sitter.Point{
	//			Row:    ^uint(0),
	//			Column: ^uint(0),
	//		},
	//	}
	//
	//	var excludedRanges []tree_sitter.Range
	//	for _, child := range node.Children(cursor) {
	//		if !includesChildren {
	//			excludedRanges = append(excludedRanges, child.Range())
	//		}
	//	}
	//	excludedRanges = append(excludedRanges, followingRange)
	//
	//	for _, excludedRange := range excludedRanges {
	//		r := tree_sitter.Range{
	//			StartByte:  precedingRange.EndByte,
	//			StartPoint: precedingRange.EndPoint,
	//			EndByte:    excludedRange.StartByte,
	//			EndPoint:   excludedRange.StartPoint,
	//		}
	//		precedingRange = excludedRange
	//
	//		if r.EndByte < parentRange.StartByte {
	//			continue
	//		}
	//
	//		for parentRange.StartByte <= r.EndByte {
	//			if parentRange.EndByte > r.StartByte {
	//				if r.StartByte < parentRange.StartByte {
	//					r.StartByte = parentRange.StartByte
	//					r.StartPoint = parentRange.StartPoint
	//				}
	//
	//				if parentRange.EndByte < r.EndByte {
	//					if r.StartByte < parentRange.EndByte {
	//						results = append(results, tree_sitter.Range{
	//							StartByte:  r.StartByte,
	//							StartPoint: r.StartPoint,
	//							EndByte:    parentRange.EndByte,
	//							EndPoint:   parentRange.EndPoint,
	//						})
	//					}
	//					r.StartByte = parentRange.EndByte
	//					r.StartPoint = parentRange.EndPoint
	//				} else {
	//					if r.StartByte < r.EndByte {
	//						results = append(results, r)
	//					}
	//					break
	//				}
	//			}
	//
	//			if len(parentRanges) > 0 {
	//				parentRange = parentRanges[0]
	//				parentRanges = parentRanges[1:]
	//			} else {
	//				return results
	//			}
	//		}
	//	}
	// }
	//
	// return results
}

func injectionForMatch(config Configuration, parentName string, query *tree_sitter.Query, match tree_sitter.QueryMatch, source []byte) (string, *tree_sitter.Node, bool) {
	var (
		languageName    string
		contentNode     *tree_sitter.Node
		includeChildren bool
	)

	for _, capture := range match.Captures {
		index := uint(capture.Index)
		if config.InjectionLanguageCaptureIndex != nil && index == *config.InjectionLanguageCaptureIndex {
			languageName = capture.Node.Utf8Text(source)
		} else if config.InjectionContentCaptureIndex != nil && index == *config.InjectionContentCaptureIndex {
			contentNode = &capture.Node
		}
	}

	for _, property := range query.PropertySettings(match.PatternIndex) {
		switch property.Key {
		case captureInjectionLanguage:
			if languageName == "" {
				languageName = *property.Value
			}
		case propertyInjectionSelf:
			if languageName == "" {
				languageName = config.LanguageName
			}
		case propertyInjectionParent:
			if languageName == "" {
				languageName = parentName
			}
		case propertyInjectionIncludeChildren:
			includeChildren = true
		}
	}

	return languageName, contentNode, includeChildren
}
