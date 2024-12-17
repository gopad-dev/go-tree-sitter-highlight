package html

import (
	"fmt"
	"iter"
	"log"
	"slices"

	"go.gopad.dev/go-tree-sitter-highlight/tags"
)

type ResolvedTag struct {
	// Tag is the definition or reference tag.
	Tag tags.Tag
	// Refs are the references to this tag if this is a definition.
	Refs []tags.Tag
	// Def is the definition tag if this is a reference.
	Def *tags.Tag
	// ID is the unique identifier of the tag.
	ID string
}

// ResolveRefs resolves references to definitions.
func (r *Renderer) ResolveRefs(tagsIter iter.Seq2[tags.Tag, error], source []byte, syntaxTypeNames []string) ([]ResolvedTag, error) {
	var resolved []ResolvedTag
	var definitionIDs []string

	for tag, err := range tagsIter {
		if err != nil {
			return nil, err
		}
		log.Printf("tag: %#v", tag)

		id := r.Options.TagIDCallback(tag, source, syntaxTypeNames)
		if tag.IsDefinition {
			var i int
			defID := id
			for slices.Contains(definitionIDs, defID) {
				i++
				defID = fmt.Sprintf("%s~%d", id, i)
			}
			definitionIDs = append(definitionIDs, defID)
			id = defID
		}

		resolved = append(resolved, ResolvedTag{
			Tag: tag,
			ID:  id,
		})
	}

	for i, tag := range resolved {
		if tag.Tag.IsDefinition {
			continue
		}

		defIndex := findDefForRef(tag.Tag, resolved, source, syntaxTypeNames)
		if defIndex == -1 {
			continue
		}

		resolved[defIndex].Refs = append(resolved[defIndex].Refs, tag.Tag)
		resolved[i].ID = resolved[defIndex].ID
		resolved[i].Def = &resolved[defIndex].Tag
	}

	return resolved, nil
}

func findDefForRef(ref tags.Tag, allTags []ResolvedTag, source []byte, syntaxTypeNames []string) int {
	for i, tag := range allTags {
		if !tag.Tag.IsDefinition {
			continue
		}

		if tag.Tag.Name(source) == ref.Name(source) {
			if ref.SyntaxTypeID == 0 || tag.Tag.SyntaxTypeID == ref.SyntaxTypeID {
				return i
			}

			syntaxName := syntaxTypeNames[ref.SyntaxTypeID]
			names, ok := similarSyntaxTypeNames[syntaxName]
			if ok && slices.Contains(names, syntaxTypeNames[tag.Tag.SyntaxTypeID]) {
				return i
			}
		}
	}

	return -1
}
