package murmur2

import (
	"encoding/binary"
	"github.com/bradleyjkemp/cupaloy"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMurmur2CF_New(t *testing.T) {
	// Test that New() creates a properly initialized Murmur2CF instance
	h := New()

	// Verify it's the correct type
	_, ok := h.(*Murmur2CF)
	if !ok {
		t.Errorf("New() did not return a *Murmur2CF, got %T", h)
	}

	// Check initial buffer is empty
	m := h.(*Murmur2CF)
	cupaloy.SnapshotT(t, len(m.buf))
}

func TestMurmur2CF_Write(t *testing.T) {
	m := New().(*Murmur2CF)

	// Test writing with whitespace characters
	n, err := m.Write([]byte("Hello, World!\t\n\r "))
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}

	// Check that it returns the correct number of bytes written
	if n != 17 {
		t.Errorf("Write() returned %d, want 17", n)
	}

	// Check that whitespace was stripped
	err = cupaloy.SnapshotMulti("init", m.buf)
	assert.NoError(t, err)

	// Test writing more data
	n, err = m.Write([]byte(" More data"))
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}

	// Check accumulated buffer
	err = cupaloy.SnapshotMulti("append", m.buf)
	assert.NoError(t, err)
}

func TestMurmur2CF_Sum(t *testing.T) {
	m := New().(*Murmur2CF)

	// Test with empty input
	sum := m.Sum(nil)
	err := cupaloy.SnapshotMulti("empty", sum)
	assert.NoError(t, err)

	// Test with some input
	m.Write([]byte("Hello, World!"))
	sum = m.Sum(nil)
	err = cupaloy.SnapshotMulti("str", sum)
	assert.NoError(t, err)

	// Test with pre-allocated buffer
	b := make([]byte, 4)
	sum = m.Sum(b)
	err = cupaloy.SnapshotMulti("buff", sum)
	assert.NoError(t, err)
}

func TestMurmur2CF_Reset(t *testing.T) {
	m := New().(*Murmur2CF)

	// Add some data
	m.Write([]byte("Hello, World!"))

	// Before reset
	err := cupaloy.SnapshotMulti("Before reset", m.buf)
	assert.NoError(t, err)

	// Reset
	m.Reset()

	// After reset
	err = cupaloy.SnapshotMulti("After reset", m.buf)
	assert.NoError(t, err)
}

func TestMurmur2CF_Size(t *testing.T) {
	m := New().(*Murmur2CF)
	size := m.Size()
	cupaloy.SnapshotT(t, size)
}

func TestMurmur2CF_BlockSize(t *testing.T) {
	m := New().(*Murmur2CF)
	blockSize := m.BlockSize()
	cupaloy.SnapshotT(t, blockSize)
}

func TestMurmur2CF_Sum32(t *testing.T) {
	m := New().(*Murmur2CF)

	// Test with empty input
	sum32 := m.Sum32()
	err := cupaloy.SnapshotMulti("empty", sum32)
	assert.NoError(t, err)

	// Test with some input
	m.Write([]byte("Hello, World!"))
	sum32 = m.Sum32()
	err = cupaloy.SnapshotMulti("string", sum32)
	assert.NoError(t, err)

	// Verify Sum32 matches binary.BigEndian.Uint32(m.Sum(nil))
	sumBytes := m.Sum(nil)
	expectedSum32 := binary.BigEndian.Uint32(sumBytes)
	if sum32 != expectedSum32 {
		t.Errorf("Sum32() = %d, want %d", sum32, expectedSum32)
	}
}

func TestMurmur2CF_WhitespaceHandling(t *testing.T) {
	// Test that whitespace is properly handled
	cases := []struct {
		name  string
		input string
	}{
		{"No whitespace", "HelloWorld"},
		{"With spaces", "Hello World"},
		{"With tab", "Hello\tWorld"},
		{"With newline", "Hello\nWorld"},
		{"With carriage return", "Hello\rWorld"},
		{"Mixed whitespace", "Hello \t\n\rWorld"},
		{"Only whitespace", " \t\n\r"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := New().(*Murmur2CF)
			m.Write([]byte(tc.input))

			// Snapshot the buffer and hash result
			result := struct {
				Buffer []byte
				Hash   uint32
			}{
				Buffer: m.buf,
				Hash:   m.Sum32(),
			}

			cupaloy.SnapshotT(t, result)
		})
	}
}
