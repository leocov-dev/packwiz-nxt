package core

import (
	"bytes"
	"encoding/binary"
	"github.com/bradleyjkemp/cupaloy"
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
)

func TestGetHashImpl(t *testing.T) {
	tests := []struct {
		name     string
		hashType string
		wantErr  bool
	}{
		{"SHA1", "sha1", false},
		{"SHA1 uppercase", "SHA1", false},
		{"SHA256", "sha256", false},
		{"SHA512", "sha512", false},
		{"MD5", "md5", false},
		{"Murmur2", "murmur2", false},
		{"Length-bytes", "length-bytes", false},
		{"Invalid hash", "invalid-hash", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetHashImpl(tt.hashType)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
			}
		})
	}
}

func TestHexStringer(t *testing.T) {
	tests := []struct {
		name     string
		hashType string
		data     []byte
	}{
		{"SHA1", "sha1", []byte("test data")},
		{"SHA256", "sha256", []byte("test data")},
		{"SHA512", "sha512", []byte("test data")},
		{"MD5", "md5", []byte("test data")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasher, err := GetHashImpl(tt.hashType)
			assert.NoError(t, err)

			_, err = hasher.Write(tt.data)
			assert.NoError(t, err)

			cupaloy.SnapshotT(t, hasher.String())
		})
	}
}

func TestNumber32As64Stringer(t *testing.T) {
	t.Run("Murmur2", func(t *testing.T) {
		hasher, err := GetHashImpl("murmur2")
		assert.NoError(t, err)

		_, err = hasher.Write([]byte("test data"))
		assert.NoError(t, err)

		cupaloy.SnapshotT(t, hasher.String())

		// Verify it's a number by checking if it can be parsed
		_, err = strconv.ParseUint(hasher.String(), 10, 64)
		assert.NoError(t, err)
	})
}

func TestLengthHasher(t *testing.T) {
	t.Run("Basic functionality", func(t *testing.T) {
		hasher, err := GetHashImpl("length-bytes")
		assert.NoError(t, err)

		testData := []byte("test data")
		dataLen := len(testData)

		n, err := hasher.Write(testData)
		assert.NoError(t, err)
		assert.Equal(t, dataLen, n)

		cupaloy.SnapshotT(t, hasher.String())
	})

	t.Run("Multiple writes", func(t *testing.T) {
		hasher, err := GetHashImpl("length-bytes")
		assert.NoError(t, err)

		data1 := []byte("first chunk")
		data2 := []byte("second chunk")

		_, err = hasher.Write(data1)
		assert.NoError(t, err)

		_, err = hasher.Write(data2)
		assert.NoError(t, err)

		cupaloy.SnapshotT(t, hasher.String())
	})

	t.Run("Reset functionality", func(t *testing.T) {
		lengthHasher := &LengthHasher{}

		_, err := lengthHasher.Write([]byte("test data"))
		assert.NoError(t, err)

		lengthHasher.Reset()

		buffer := new(bytes.Buffer)
		sum := lengthHasher.Sum(buffer.Bytes())

		assert.Equal(t, uint64(0), binary.BigEndian.Uint64(sum))
		assert.Equal(t, 8, lengthHasher.Size())
		assert.Equal(t, 1, lengthHasher.BlockSize())
	})
}
