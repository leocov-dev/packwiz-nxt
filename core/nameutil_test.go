package core

import (
	"testing"

	"github.com/bradleyjkemp/cupaloy"
)

func TestSlugifyName(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "Basic lowercase conversion",
			input: "Hello World",
		},
		{
			name:  "Remove parentheses and content inside",
			input: "Product (Special Edition)",
		},
		{
			name:  "Remove suffix after hyphen-space",
			input: "Movie - Director's Cut",
		},
		{
			name:  "Replace non-alphanumeric with hyphens",
			input: "Hello! @World#",
		},
		{
			name:  "Collapse multiple hyphens",
			input: "Hello---World",
		},
		{
			name:  "Remove leading and trailing hyphens",
			input: "-hello-world-",
		},
		{
			name:  "Empty string",
			input: "",
		},
		{
			name:  "Only special characters",
			input: "!@#$%^&*()",
		},
		{
			name:  "Combination of all transformations",
			input: "Product Name (Limited Edition) - Special Version 2.0!",
		},
		{
			name:  "Numbers preserved",
			input: "Product123",
		},
		{
			name:  "Multiple spaces",
			input: "Hello  World",
		},
		{
			name:  "Special characters between words",
			input: "Hello@World",
		},
		{
			name:  "Parentheses with no content",
			input: "Product ()",
		},
		{
			name:  "Multiple parenthetical expressions",
			input: "Product (A) (B)",
		},
		{
			name:  "Multiple hyphen-space patterns",
			input: "A - B - C",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SlugifyName(tt.input)
			cupaloy.SnapshotT(t, result)
		})
	}
}
