package tags

import (
	"bytes"
	"context"
	"slices"
	"unicode"
	"unicode/utf16"

	"github.com/tree-sitter/go-tree-sitter"

	"go.gopad.dev/go-tree-sitter-highlight/internal/peekiter"
)

const maxLineLen = 180

type tagQueueItem struct {
	Tag          Tag
	PatternIndex uint
}

type lineInfoItem struct {
	UTF8Position tree_sitter.Point
	UTF8Byte     uint
	UTF16Column  uint
	LineRange    byteRange
}

type iterator struct {
	Ctx          context.Context
	Source       []byte
	Tree         *tree_sitter.Tree
	Matches      *peekiter.QueryMatches
	Cfg          Configuration
	Scopes       []localScope
	TagQueue     []tagQueueItem
	PrevLineInfo *lineInfoItem
}

func (t *iterator) next() (*Tag, error) {
	for {
		// If there is a queued tag for an earlier node in the syntax tree, then pop
		// it off of the queue and return it.
		if len(t.TagQueue) > 0 {
			lastEntry := t.TagQueue[len(t.TagQueue)-1]
			if len(t.TagQueue) > 1 && t.TagQueue[0].Tag.NameRange.End < lastEntry.Tag.NameRange.Start {
				tag := t.TagQueue[0].Tag
				t.TagQueue = t.TagQueue[1:]
				if tag.IsIgnored() {
					continue
				}
				return &tag, nil
			}
		}

		// check for cancellation
		select {
		case <-t.Ctx.Done():
			return nil, t.Ctx.Err()
		default:
		}

		if match, ok := t.Matches.Next(); ok {
			patternInfo := t.Cfg.patternInfo[match.PatternIndex]

			if match.PatternIndex < t.Cfg.tagsPatternIndex {
				for _, capture := range match.Captures {
					index := uint(capture.Index)
					captureRangeStart, captureRangeEnd := capture.Node.ByteRange()
					if t.Cfg.localScopeCaptureIndex != nil && index == *t.Cfg.localScopeCaptureIndex {
						t.Scopes = append(t.Scopes, localScope{
							Inherits: patternInfo.LocalScopeInherits,
							Range: byteRange{
								Start: captureRangeStart,
								End:   captureRangeEnd,
							},
							LocalDefs: nil,
						})
					} else if t.Cfg.localDefinitionCaptureIndex != nil && index == *t.Cfg.localDefinitionCaptureIndex {
						if i := slices.IndexFunc(t.Scopes, func(scope localScope) bool {
							return scope.Range.Start <= captureRangeStart && captureRangeEnd >= scope.Range.End
						}); i > -1 {
							t.Scopes[i].LocalDefs = append(t.Scopes[i].LocalDefs, localDef{
								Name: t.Source[captureRangeStart:captureRangeEnd],
							})
						}
					}
				}
				continue
			}

			var (
				nameNode         *tree_sitter.Node
				docNodes         []tree_sitter.Node
				tagNode          *tree_sitter.Node
				syntaxTypeID     uint
				isDefinition     bool
				docsAdjacentNode *tree_sitter.Node
				isIgnored        bool
			)

			for _, capture := range match.Captures {
				index := uint(capture.Index)

				if t.Cfg.ignoreCaptureIndex != nil && index == *t.Cfg.ignoreCaptureIndex {
					isIgnored = true
					nameNode = &capture.Node
				}

				if docsAdjacentCapture := t.Cfg.patternInfo[match.PatternIndex].DocsAdjacentCapture; docsAdjacentCapture != nil && index == *docsAdjacentCapture {
					docsAdjacentNode = &capture.Node
				}

				if t.Cfg.nameCaptureIndex != nil && index == *t.Cfg.nameCaptureIndex {
					nameNode = &capture.Node
				} else if t.Cfg.docCaptureIndex != nil && index == *t.Cfg.docCaptureIndex {
					docNodes = append(docNodes, capture.Node)
				}

				if namedCapture, ok := t.Cfg.captureMap[uint(capture.Index)]; ok {
					tagNode = &capture.Node
					syntaxTypeID = namedCapture.SyntaxTypeID
					isDefinition = namedCapture.IsDefinition
				}
			}

			if nameNode != nil {
				nameRange := newByteRange(nameNode.ByteRange())

				var tag Tag
				if tagNode != nil {
					if nameNode.HasError() {
						continue
					}

					if patternInfo.NameMustBeNonLocal {
						var isLocal bool
						for _, scope := range slices.Backward(t.Scopes) {
							if scope.Range.Start <= nameRange.Start && scope.Range.End >= nameRange.End {
								if i := slices.IndexFunc(scope.LocalDefs, func(def localDef) bool {
									return bytes.Equal(def.Name, t.Source[nameRange.Start:nameRange.End])
								}); i > -1 {
									isLocal = true
								}
								if !scope.Inherits {
									break
								}
							}
						}
						if isLocal {
							continue
						}
					}

					// If needed, filter the doc nodes based on their ranges, selecting
					// only the slice that are adjacent to some specified node.
					var docStartIndex int
					if docsAdjacentNode != nil && len(docNodes) > 0 {
						docStartIndex = len(docNodes)
						startRow := docsAdjacentNode.StartPosition().Row
						for docStartIndex > 0 {
							docNode := docNodes[docStartIndex-1]
							prevDocEndRow := docNode.EndPosition().Row
							if prevDocEndRow+1 >= startRow {
								docStartIndex--
								startRow = docNode.StartPosition().Row
							} else {
								break
							}
						}
					}

					// Generate a doc string from all the doc nodes, applying any strip regexes.
					var docs string
					for _, docNode := range docNodes[docStartIndex:] {
						content := docNode.Utf8Text(t.Source)

						if patternInfo.DocStripRegex != nil {
							content = patternInfo.DocStripRegex.ReplaceAllString(content, "")
						}

						if docs == "" {
							docs = content
						} else {
							docs += "\n" + content
						}
					}

					rngStart, rngEnd := tagNode.ByteRange()
					tagRange := byteRange{
						Start: min(rngStart, nameRange.Start),
						End:   max(rngEnd, nameRange.End),
					}
					span := pointRange{
						Start: nameNode.StartPosition(),
						End:   nameNode.EndPosition(),
					}

					// Compute tag properties that depend on the text of the containing line. If
					// the previous tag occurred on the same line, then
					// reuse results from the previous tag.
					var prevUTF16Column uint
					prevUTF8Byte := nameRange.Start - span.Start.Column

					var lineInfo *lineInfoItem
					if t.PrevLineInfo != nil && t.PrevLineInfo.UTF8Position.Row == span.Start.Row {
						lineInfo = t.PrevLineInfo
					}

					var lineRange byteRange
					if lineInfo != nil {
						if lineInfo.UTF8Position.Column <= span.Start.Column {
							prevUTF8Byte = lineInfo.UTF8Byte
							prevUTF16Column = lineInfo.UTF16Column
						}
						lineRange = lineInfo.LineRange
					} else {
						lineRange = newLineRange(t.Source, nameRange.Start, span.Start, maxLineLen)
					}

					utf16StartColumn := prevUTF16Column + utf16Len(t.Source[prevUTF8Byte:nameRange.Start])
					utf16EndColumn := utf16StartColumn + utf16Len(t.Source[nameRange.Start:nameRange.End])
					utf16ColumnRange := byteRange{
						Start: utf16StartColumn,
						End:   utf16EndColumn,
					}

					t.PrevLineInfo = &lineInfoItem{
						UTF8Position: span.End,
						UTF8Byte:     nameRange.End,
						UTF16Column:  utf16EndColumn,
						LineRange:    lineRange,
					}
					tag = Tag{
						Range:            tagRange,
						NameRange:        nameRange,
						LineRange:        lineRange,
						Span:             span,
						UTF16ColumnRange: utf16ColumnRange,
						Docs:             docs,
						IsDefinition:     isDefinition,
						SyntaxTypeID:     syntaxTypeID,
					}
				} else if isIgnored {
					tag = newIgnoredTag(nameRange)
				} else {
					continue
				}

				// Only create one tag per node. The tag queue is sorted by node position
				// to allow for fast lookup.
				if i, ok := slices.BinarySearchFunc(t.TagQueue, tagQueueItem{
					Tag:          tag,
					PatternIndex: match.PatternIndex,
				}, func(tag1, tag2 tagQueueItem) int {
					if tag1.Tag.NameRange.Start < tag2.Tag.NameRange.Start {
						return -1
					}
					if tag1.Tag.NameRange.Start > tag2.Tag.NameRange.Start {
						return 1
					}
					return 0
				}); ok {
					existingTagItem := t.TagQueue[i]
					if existingTagItem.PatternIndex > match.PatternIndex {
						t.TagQueue[i].Tag = tag
					}
				} else {
					t.TagQueue = slices.Insert(t.TagQueue, i, tagQueueItem{
						Tag:          tag,
						PatternIndex: match.PatternIndex,
					})
				}
			}
		} else if len(t.TagQueue) > 0 {
			// If there are no more matches, then drain the queue.
			tag := t.TagQueue[0].Tag
			t.TagQueue = t.TagQueue[1:]
			return &tag, nil
		} else {
			return nil, nil
		}

	}
}

func utf16Len(s []byte) uint {
	return uint(len(utf16.Encode([]rune(string(s)))))
}

func newLineRange(source []byte, startByte uint, startPoint tree_sitter.Point, maxLineLen uint) byteRange {
	// Trim leading whitespace
	lineStartByte := startByte - startPoint.Column
	for lineStartByte < uint(len(source)) && unicode.IsSpace(rune(source[lineStartByte])) {
		lineStartByte++
	}

	maxLineLen = min(maxLineLen, uint(len(source))-lineStartByte)
	textAfterLineStart := source[lineStartByte : lineStartByte+maxLineLen]
	var lineLen uint
	if i := bytes.IndexByte(textAfterLineStart, '\n'); i > -1 {
		lineLen = uint(i)
	} else {
		lineLen = uint(len(textAfterLineStart))
	}

	lineEndByte := lineStartByte + lineLen
	for lineEndByte > lineStartByte && unicode.IsSpace(rune(source[lineEndByte-1])) {
		lineEndByte--
	}

	return byteRange{
		Start: lineStartByte,
		End:   lineEndByte,
	}
}
