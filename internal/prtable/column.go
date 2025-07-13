package prtable

import (
	"encoding/json"
	"fmt"
	"iter"
	"maps"
	"math"
	"slices"
	"strings"
)

// Column identifies colums in the output table
type Column int

const (
	checksColumn Column = iota
	mergeableColumn
	approvedColumn
	draftColumn
	titleColumn
	urlColumn
	authorColumn
	repositoryColumn
	changeColumn
	stateColumn
	commentsColumn
	updatedAtColumn
)

var (
	columnIndex_name = map[Column]string{
		checksColumn:     "checks",
		mergeableColumn:  "mergable",
		approvedColumn:   "approved",
		draftColumn:      "draft",
		titleColumn:      "title",
		urlColumn:        "url",
		authorColumn:     "author",
		repositoryColumn: "repository",
		changeColumn:     "change",
		stateColumn:      "state",
		commentsColumn:   "comments",
		updatedAtColumn:  "updatedAt",
	}
	column_value = map[string]Column{
		"checks":     checksColumn,
		"mergeable":  mergeableColumn,
		"approved":   approvedColumn,
		"draft":      draftColumn,
		"title":      titleColumn,
		"url":        urlColumn,
		"author":     authorColumn,
		"repository": repositoryColumn,
		"change":     changeColumn,
		"state":      stateColumn,
		"comments":   commentsColumn,
		"updatedAt":  updatedAtColumn,
	}
	columnIndex_title = map[Column]string{
		checksColumn:     "C",
		mergeableColumn:  "M",
		approvedColumn:   "A",
		draftColumn:      "D",
		titleColumn:      "Title",
		urlColumn:        "Url",
		authorColumn:     "Author",
		repositoryColumn: "Repository",
		changeColumn:     "Change",
		stateColumn:      "State",
		commentsColumn:   "Comments",
		updatedAtColumn:  "UpdatedAt",
	}
	columnIndex_minWidth = map[Column]int{
		checksColumn:     2,
		mergeableColumn:  2,
		approvedColumn:   2,
		draftColumn:      2,
		titleColumn:      5,
		urlColumn:        5,
		authorColumn:     6,
		repositoryColumn: 10,
		changeColumn:     6,
		stateColumn:      5,
		commentsColumn:   5,
		updatedAtColumn:  10,
	}
	columnIndex_maxWidth = map[Column]int{
		checksColumn:     2,
		mergeableColumn:  2,
		approvedColumn:   2,
		draftColumn:      2,
		titleColumn:      math.MaxInt,
		urlColumn:        math.MaxInt,
		authorColumn:     math.MaxInt,
		repositoryColumn: math.MaxInt,
		changeColumn:     math.MaxInt,
		stateColumn:      math.MaxInt,
		commentsColumn:   math.MaxInt,
		updatedAtColumn:  math.MaxInt,
	}
	defaultDefaultColumns = []Column{
		checksColumn,
		mergeableColumn,
		approvedColumn,
		titleColumn,
		authorColumn,
		repositoryColumn,
		changeColumn,
		updatedAtColumn,
	}
	defaultWideColumns = []Column{
		checksColumn,
		mergeableColumn,
		approvedColumn,
		draftColumn,
		titleColumn,
		urlColumn,
		authorColumn,
		repositoryColumn,
		changeColumn,
		stateColumn,
		commentsColumn,
		updatedAtColumn,
	}
)

func (i Column) String() string {
	return columnIndex_name[i]
}

func Map[V, U any](mapFn func(V) U, s iter.Seq[V]) iter.Seq[U] {
	return func(yield func(U) bool) {
		for item := range s {
			if !yield(mapFn(item)) {
				return
			}
		}
	}
}

func parseColumnIndex(s string) (Column, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	value, present := column_value[s]
	if !present {
		names := Map(Column.String, maps.Keys(columnIndex_name))
		validColumns := strings.Join(slices.Sorted(names), ", ")
		return 0, fmt.Errorf("unknown column: %s (must be one of %s)", s, validColumns)
	}
	return value, nil
}

func (i *Column) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.String())
}

func (i *Column) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}
	*i, err = parseColumnIndex(s)
	return err
}
