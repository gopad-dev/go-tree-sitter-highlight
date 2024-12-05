package highlight

import (
	"context"
	"slices"

	"github.com/tree-sitter/go-tree-sitter"
)

type highlightRange struct {
	start uint
	end   uint
	depth uint
}

type iterator struct {
	Ctx                context.Context
	Source             []byte
	LanguageName       string
	ByteOffset         uint
	Highlighter        *Highlighter
	InjectionCallback  InjectionCallback
	Layers             []*iterLayer
	NextEvents         []Event
	LastHighlightRange *highlightRange
	LayerStack         []*iterLayer
}

func (h *iterator) emitEvents(offset uint, events ...Event) (Event, error) {
	var result Event
	if h.ByteOffset < offset {
		result = EventSource{
			StartByte: h.ByteOffset,
			EndByte:   offset,
		}
		h.ByteOffset = offset
		h.NextEvents = append(h.NextEvents, events...)
	} else {
		if len(events) > 1 {
			h.NextEvents = append(h.NextEvents, events[1:]...)
		}
		result = events[0]
	}
	h.sortLayers()
	return result, nil
}

func (h *iterator) next() (Event, error) {
main:
	for {
		if len(h.NextEvents) > 0 {
			event := h.NextEvents[0]
			h.NextEvents = h.NextEvents[1:]
			return event, nil
		}

		// check for cancellation
		select {
		case <-h.Ctx.Done():
			return nil, h.Ctx.Err()
		default:
		}

		// Get the next capture from whichever layer has the earliest highlight boundary.
		var layer *iterLayer
		if len(h.Layers) > 0 {
			layer = h.Layers[0]
		}

		// Check if this layer is new and emit a layer start event if it is.
		if layer != nil {
			if layer.New {
				layer.New = false
				h.NextEvents = append(h.NextEvents, EventLayerStart{
					LanguageName: layer.Config.LanguageName,
					Range:        layer.Ranges[0], // TODO: handle multiple ranges
				})
			}
		}

		// If none of the layers have any more highlight boundaries, terminate.
		if layer == nil {
			if h.ByteOffset < uint(len(h.Source)) {
				event := EventSource{
					StartByte: h.ByteOffset,
					EndByte:   uint(len(h.Source)),
				}
				h.ByteOffset = uint(len(h.Source))
				return event, nil
			}

			if len(h.LayerStack) > 0 {
				h.LayerStack = h.LayerStack[:len(h.LayerStack)-1]
				return h.emitEvents(h.ByteOffset, EventLayerEnd{})
			}

			return nil, nil
		}

		if len(h.LayerStack) == 0 || h.LayerStack[len(h.LayerStack)-1] != layer {
			h.LayerStack = append(h.LayerStack, layer)
		}

		var nextCaptureRange tree_sitter.Range
		if nextMatch, captureIndex, ok := layer.Captures.Peek(); ok {
			nextCapture := nextMatch.Captures[captureIndex]
			nextCaptureRange = nextCapture.Node.Range()

			// If any previous highlight ends before this node starts, then before
			// processing this capture, emit the source code up until the end of the
			// previous highlight, and an end event for that highlight.
			if len(layer.HighlightEndStack) > 0 {
				endByte := layer.HighlightEndStack[len(layer.HighlightEndStack)-1]
				if endByte <= nextCaptureRange.StartByte {
					layer.HighlightEndStack = layer.HighlightEndStack[:len(layer.HighlightEndStack)-1]
					return h.emitEvents(endByte, EventCaptureEnd{})
				}
			}
		} else {
			// If there are no more captures, then emit any remaining highlight end events.
			// And if there are none of those, then just advance to the end of the document.
			if len(layer.HighlightEndStack) > 0 {
				endByte := layer.HighlightEndStack[len(layer.HighlightEndStack)-1]
				layer.HighlightEndStack = layer.HighlightEndStack[:len(layer.HighlightEndStack)-1]
				return h.emitEvents(endByte, EventCaptureEnd{})
			}
			return h.emitEvents(uint(len(h.Source)), nil)
		}

		match, captureIndex, _ := layer.Captures.Next()
		capture := match.Captures[captureIndex]

		// Remove from the layer stack any layers that have already ended.
		for len(h.LayerStack) > 0 && h.LayerStack[len(h.LayerStack)-1].Ranges[0].EndByte <= nextCaptureRange.StartByte {
			h.LayerStack = h.LayerStack[:len(h.LayerStack)-1]
			h.NextEvents = append(h.NextEvents, EventLayerEnd{})
		}

		// If this capture represents an injection, then process the injection.
		if match.PatternIndex < layer.Config.LocalsPatternIndex {
			languageName, contentNode, includeChildren := injectionForMatch(layer.Config, h.LanguageName, layer.Config.Query, match, h.Source)

			// Explicitly remove this match so that none of its other captures will remain
			// in the stream of captures.
			match.Remove()

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
		for nextCaptureRange.StartByte > layer.ScopeStack[len(layer.ScopeStack)-1].Range.EndByte {
			layer.ScopeStack = layer.ScopeStack[:len(layer.ScopeStack)-1]
		}

		// If this capture is for tracking local variables, then process the
		// local variable info.
		var referenceHighlight *Highlight
		var definitionHighlight *Highlight
		for match.PatternIndex < layer.Config.FoldsPatternIndex {
			// If the node represents a local scope, push a new local scope onto
			// the scope stack.
			if layer.Config.LocalScopeCaptureIndex != nil && uint(capture.Index) == *layer.Config.LocalScopeCaptureIndex {
				definitionHighlight = nil
				scope := localScope{
					Inherits:  true,
					Range:     nextCaptureRange,
					LocalDefs: nil,
				}
				for _, prop := range layer.Config.Query.PropertySettings(match.PatternIndex) {
					if prop.Key == propertyLocalScopeInherits {
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
				for _, matchCapture := range match.Captures {
					if layer.Config.LocalDefValueCaptureIndex != nil && uint(matchCapture.Index) == *layer.Config.LocalDefValueCaptureIndex {
						valueRange = matchCapture.Node.Range()
					}
				}

				if len(h.Source) > int(nextCaptureRange.StartByte) && len(h.Source) > int(valueRange.EndByte) {
					name := string(h.Source[nextCaptureRange.StartByte:nextCaptureRange.EndByte])

					scope.LocalDefs = append(scope.LocalDefs, localDef{
						Name:      name,
						Range:     nextCaptureRange,
						Highlight: nil,
					})
					definitionHighlight = scope.LocalDefs[len(scope.LocalDefs)-1].Highlight
				}
			} else if layer.Config.LocalRefCaptureIndex != nil && uint(capture.Index) == *layer.Config.LocalRefCaptureIndex && definitionHighlight == nil {
				// If the node represents a reference, then try to find the corresponding
				// definition in the scope stack.
				definitionHighlight = nil
				if len(h.Source) > int(nextCaptureRange.StartByte) && len(h.Source) > int(nextCaptureRange.EndByte) {
					name := string(h.Source[nextCaptureRange.StartByte:nextCaptureRange.EndByte])
					for _, scope := range slices.Backward(layer.ScopeStack) {
						var highlight *Highlight
						for _, def := range slices.Backward(scope.LocalDefs) {
							if def.Name == name && nextCaptureRange.StartByte >= def.Range.EndByte {
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
			if nextMatch, nextCaptureIndex, ok := layer.Captures.Peek(); ok {
				nextCapture := nextMatch.Captures[nextCaptureIndex]
				if nextCapture.Node.Equals(capture.Node) {
					capture = nextCapture
					match, _, _ = layer.Captures.Next()
					continue
				}
			}

			h.sortLayers()
			continue main
		}

		// If this capture is for tracking folds, then process the fold info.
		for match.PatternIndex < layer.Config.HighlightsPatternIndex {
			if layer.Config.FoldCaptureIndex != nil && uint(capture.Index) == *layer.Config.FoldCaptureIndex {
				fold := codeFold{
					Range: nextCaptureRange,
				}

				layer.FoldStack = append(layer.FoldStack, fold)
			}

			// Continue processing any additional matches for the same node.
			if nextMatch, nextCaptureIndex, ok := layer.Captures.Peek(); ok {
				nextCapture := nextMatch.Captures[nextCaptureIndex]
				if nextCapture.Node.Equals(capture.Node) {
					capture = nextCapture
					match, _, _ = layer.Captures.Next()
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
			if nextCaptureRange.StartByte == lastRange.start && nextCaptureRange.EndByte == lastRange.end && layer.Depth < lastRange.depth {
				h.sortLayers()
				continue main
			}
		}

		// Once a highlighting pattern is found for the current node, keep iterating over
		// any later highlighting patterns that also match this node and set the match to it.
		// Captures for a given node are ordered by pattern index, so these subsequent
		// captures are guaranteed to be for highlighting, not injections or
		// local variables.
		for {
			nextMatch, nextCaptureIndex, ok := layer.Captures.Peek()
			if !ok {
				break
			}

			nextCapture := nextMatch.Captures[nextCaptureIndex]
			if nextCapture.Node.Equals(capture.Node) {
				followingMatch, _, _ := layer.Captures.Next()
				// If the current node was found to be a local variable, then ignore
				// the following match if it's a highlighting pattern that is disabled
				// for local variables.
				if definitionHighlight != nil || referenceHighlight != nil && layer.Config.NonLocalVariablePatterns[followingMatch.PatternIndex] {
					continue
				}

				match.Remove()
				capture = nextCapture
				match = followingMatch
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
			h.LastHighlightRange = &highlightRange{
				start: nextCaptureRange.StartByte,
				end:   nextCaptureRange.EndByte,
				depth: layer.Depth,
			}
			layer.HighlightEndStack = append(layer.HighlightEndStack, nextCaptureRange.EndByte)
			return h.emitEvents(nextCaptureRange.StartByte, EventCaptureStart{
				Highlight: *highlight,
			})
		}

		h.sortLayers()
	}
}

func (h *iterator) sortLayers() {
	for len(h.Layers) > 0 {
		key := h.Layers[0].sortKey()
		if key != nil {
			var i int
			for i+1 < len(h.Layers) {
				nextOffsetKey := h.Layers[i+1].sortKey()
				if nextOffsetKey != nil {
					if nextOffsetKey.GreaterThan(*key) {
						i += 1
						continue
					}
				}
				break
			}
			if i > 0 {
				h.Layers = append(rotateLeft(h.Layers[:i+1], 1), h.Layers[i+1:]...)
			}
			break
		}
		layer := h.Layers[0]
		h.Layers = h.Layers[1:]
		h.Highlighter.pushCursor(layer.Cursor)
	}
}

func (h *iterator) insertLayer(layer *iterLayer) {
	key := layer.sortKey()
	if key != nil {
		i := 1
		for i < len(h.Layers) {
			keyI := h.Layers[i].sortKey()
			if keyI != nil {
				if keyI.LessThan(*key) {
					h.Layers = slices.Insert(h.Layers, i, layer)
					return
				}
				i += 1
			} else {
				h.Layers = slices.Delete(h.Layers, i, i+1)
			}
		}
		h.Layers = append(h.Layers, layer)
	}
}

func rotateLeft[T any](s []T, i int) []T {
	return append(s[i:], s[:i]...)
}
