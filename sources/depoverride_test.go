package sources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapDepOverride(t *testing.T) {
	tests := []struct {
		name      string
		depID     uint32
		isQuilt   bool
		mcVersion string
		want      uint32
	}{
		{"not quilt, matching depID left unchanged", 306612, false, "1.19.2", 306612},
		{"quilt, unknown depID left unchanged", 999999, true, "1.19.2", 999999},
		{"quilt, Fabric API override applies regardless of mcVersion", 306612, true, "1.16.5", 634179},
		{"quilt, Fabric Language Kotlin inside version gate", 308769, true, "1.19.2", 720410},
		{"quilt, Fabric Language Kotlin at upper edge of gate", 308769, true, "1.20", 720410},
		{"quilt, Fabric Language Kotlin at lower boundary (excluded)", 308769, true, "1.19.1", 308769},
		{"quilt, Fabric Language Kotlin below gate", 308769, true, "1.18", 308769},
		{"quilt, Fabric Language Kotlin at upper boundary (excluded)", 308769, true, "2.0.0", 308769},
		{"quilt, Fabric Language Kotlin above gate", 308769, true, "3.0.0", 308769},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapDepOverride(tt.depID, tt.isQuilt, tt.mcVersion)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMrMapDepOverride(t *testing.T) {
	tests := []struct {
		name      string
		depID     string
		isQuilt   bool
		mcVersion string
		want      string
	}{
		{"not quilt, matching depID left unchanged", "P7dR8mSH", false, "1.19.2", "P7dR8mSH"},
		{"quilt, unknown depID left unchanged", "some-unrelated-mod", true, "1.19.2", "some-unrelated-mod"},
		{"quilt, Fabric API override applies via project ID alias", "P7dR8mSH", true, "1.16.5", "qvIfYCYJ"},
		{"quilt, Fabric API override applies via slug alias", "fabric-api", true, "1.16.5", "qvIfYCYJ"},
		{"quilt, Fabric Language Kotlin inside version gate via project ID", "Ha28R6CL", true, "1.19.2", "lwVhp9o5"},
		{"quilt, Fabric Language Kotlin inside version gate via slug", "fabric-language-kotlin", true, "1.19.2", "lwVhp9o5"},
		{"quilt, Fabric Language Kotlin at lower boundary (excluded)", "Ha28R6CL", true, "1.19.1", "Ha28R6CL"},
		{"quilt, Fabric Language Kotlin below gate", "Ha28R6CL", true, "1.18", "Ha28R6CL"},
		{"quilt, Fabric Language Kotlin at upper boundary (excluded)", "Ha28R6CL", true, "2.0.0", "Ha28R6CL"},
		{"quilt, Fabric Language Kotlin above gate", "Ha28R6CL", true, "3.0.0", "Ha28R6CL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mrMapDepOverride(tt.depID, tt.isQuilt, tt.mcVersion)
			assert.Equal(t, tt.want, got)
		})
	}
}
