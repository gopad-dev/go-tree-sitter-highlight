package folds

import (
	"github.com/tree-sitter/go-tree-sitter"
)

func NewConfiguration(language *tree_sitter.Language, foldsQuery []byte) (*Configuration, error) {
	query, err := tree_sitter.NewQuery(language, string(foldsQuery))
	if err != nil {
		return nil, err
	}

	var foldCaptureIndex uint
	for i, captureName := range query.CaptureNames() {
		ui := uint(i)
		switch captureName {
		case "fold":
			foldCaptureIndex = ui
		}
	}

	return &Configuration{
		Language:         language,
		Query:            query,
		foldCaptureIndex: foldCaptureIndex,
	}, nil
}

type Configuration struct {
	Language         *tree_sitter.Language
	Query            *tree_sitter.Query
	foldCaptureIndex uint
}
