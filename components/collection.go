package components

import "github.com/dmundt/go-mvu/html"

// Map converts items to components in order. Rendering stays with the returned
// Sequence, preserving normal escaping and error propagation.
func Map[T any](items []T, component func(T) html.Component) Sequence {
	children := make(Sequence, len(items))
	for index, item := range items {
		children[index] = component(item)
	}
	return children
}

// TextCell renders escaped text in a data cell.
func TextCell(value string) TableCell {
	return TableCell{Body: Text(value)}
}

// RowHeader renders escaped text in a scoped row header.
func RowHeader(value string) TableCell {
	return TableCell{Header: true, Scope: ScopeRow, Body: Text(value)}
}
