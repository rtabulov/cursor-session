package internal

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestDecodeVarint(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    uint64
		wantErr bool
	}{
		{"single byte", []byte{0x01}, 1, false},
		{"two bytes", []byte{0x80, 0x01}, 128, false},
		{"zero", []byte{0x00}, 0, false},
		{"max single byte", []byte{0x7f}, 127, false},
		{"empty", []byte{}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, bytesRead := decodeVarint(tt.data)
			if bytesRead == 0 && !tt.wantErr {
				t.Errorf("decodeVarint() bytesRead = 0, want > 0")
				return
			}
			if bytesRead > 0 && tt.wantErr {
				t.Errorf("decodeVarint() should have failed but didn't")
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("decodeVarint() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestDecodeProtobufStrings(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    []string
		wantErr bool
	}{
		{
			name: "simple string field",
			// Field 1, wire type 2 (length-delimited), length 5, "Hello"
			data: []byte{0x0a, 0x05, 'H', 'e', 'l', 'l', 'o'},
			want: []string{"Hello"},
		},
		{
			name: "multiple string fields",
			// Field 1: "Hello", Field 2: "World"
			data: []byte{
				0x0a, 0x05, 'H', 'e', 'l', 'l', 'o',
				0x12, 0x05, 'W', 'o', 'r', 'l', 'd',
			},
			want: []string{"Hello", "World"},
		},
		{
			name: "with varint field",
			// Field 1 (varint): 42, Field 2 (string): "Hello"
			data: []byte{
				0x08, 0x2a, // varint 42
				0x12, 0x05, 'H', 'e', 'l', 'l', 'o',
			},
			want: []string{"Hello"},
		},
		{
			name:    "empty data",
			data:    []byte{},
			want:    []string{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeProtobufStrings(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeProtobufStrings() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("decodeProtobufStrings() returned %d strings, want %d", len(got), len(tt.want))
				return
			}
			for i, w := range tt.want {
				if i < len(got) && got[i] != w {
					t.Errorf("decodeProtobufStrings() [%d] = %q, want %q", i, got[i], w)
				}
			}
		})
	}
}

func TestExtractProtobufFields(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name: "simple string field",
			// Field 1, wire type 2, length 5, "Hello"
			data: []byte{0x0a, 0x05, 'H', 'e', 'l', 'l', 'o'},
		},
		{
			name: "varint field",
			// Field 1, wire type 0 (varint), value 42
			data: []byte{0x08, 0x2a},
		},
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractProtobufFields(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractProtobufFields() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == nil {
				t.Error("extractProtobufFields() returned nil map")
			}
		})
	}
}

func TestTryProtobufDecode(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		want     bool
		wantKeys int
	}{
		{
			name: "valid protobuf with string",
			// Field 1, wire type 2, length 5, "Hello"
			data:     []byte{0x0a, 0x05, 'H', 'e', 'l', 'l', 'o'},
			want:     true,
			wantKeys: 1,
		},
		{
			name: "valid protobuf with varint",
			// Field 1, wire type 0, value 42
			data:     []byte{0x08, 0x2a},
			want:     true,
			wantKeys: 1,
		},
		{
			name: "invalid wire type",
			// Invalid wire type (7)
			data: []byte{0x3f},
			want: false,
		},
		{
			name: "empty data",
			data: []byte{},
			want: false,
		},
		{
			name: "random binary data",
			data: []byte{0xff, 0xfe, 0xfd, 0xfc},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := tryProtobufDecode(tt.data)
			if found != tt.want {
				t.Errorf("tryProtobufDecode() found = %v, want %v", found, tt.want)
				return
			}
			if found && got != nil {
				if len(got) < tt.wantKeys {
					t.Errorf("tryProtobufDecode() returned %d fields, want at least %d", len(got), tt.wantKeys)
				}
			}
		})
	}
}

func TestExtractProtobufFields_WithJSON(t *testing.T) {
	// Create protobuf data containing JSON
	jsonData := `{"key":"value","number":42}`
	jsonBytes := []byte(jsonData)

	// Create a simple protobuf message with the JSON as a string field
	// Field 1, wire type 2, length = len(jsonBytes), then the JSON bytes
	protobufData := []byte{0x0a, byte(len(jsonBytes))}
	protobufData = append(protobufData, jsonBytes...)

	fields, err := extractProtobufFields(protobufData)
	if err != nil {
		t.Fatalf("extractProtobufFields() error = %v", err)
	}

	if fields == nil {
		t.Fatal("extractProtobufFields() returned nil")
	}

	// Verify JSON was extracted
	foundJSON := false
	for _, value := range fields {
		if str, ok := value.(string); ok {
			// Try to parse as JSON
			var testData map[string]interface{}
			if json.Unmarshal([]byte(str), &testData) == nil {
				foundJSON = true
				break
			}
		}
	}

	if !foundJSON {
		t.Error("extractProtobufFields() did not extract JSON from protobuf")
	}
}

func TestExtractProtobufFields_OversizedLength(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
		desc    string
	}{
		{
			name: "length exceeds buffer size",
			// Field 1, wire type 2, length varint = 1000, but only 5 bytes of data follow
			data:    []byte{0x0a, 0xe8, 0x07, 'H', 'e', 'l', 'l', 'o'},
			wantErr: true,
			desc:    "length 1000 exceeds remaining buffer",
		},
		{
			name: "length equals math.MaxInt64",
			// Field 1, wire type 2, length = math.MaxInt64 encoded as varint
			// MaxInt64 = 9223372036854775807 = 0x7FFFFFFFFFFFFFFF
			// Varint encoding: 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0x7F
			data:    []byte{0x0a, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x7F},
			wantErr: true,
			desc:    "length equals MaxInt64, exceeds buffer",
		},
		{
			name: "length overflows when cast to int",
			// Field 1, wire type 2, length = math.MaxUint64 encoded as varint
			// MaxUint64 = 18446744073709551615 = 0xFFFFFFFFFFFFFFFF
			// Varint encoding: 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0x01
			data:    []byte{0x0a, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x01},
			wantErr: true,
			desc:    "length overflows int on 64-bit systems",
		},
		{
			name: "nested oversized length fails safely",
			// Outer field: Field 1, wire type 2, length 3, containing nested protobuf
			// Inner field: Field 1, wire type 2, oversized length 1000 (but no data follows)
			// The outer decode succeeds (has valid 3 bytes), inner decode fails safely without panic
			data: func() []byte {
				// Inner protobuf with oversized length
				inner := []byte{0x0a, 0xe8, 0x07} // Field 1, length 1000
				// Outer protobuf wrapping inner
				outer := []byte{0x0a, byte(len(inner))}
				return append(outer, inner...)
			}(),
			wantErr: false, // Outer decode succeeds, inner fails safely
			desc:    "nested protobuf with oversized length fails safely without panic",
		},
		{
			name: "valid small length still works",
			// Field 1, wire type 2, length 5, "Hello"
			data:    []byte{0x0a, 0x05, 'H', 'e', 'l', 'l', 'o'},
			wantErr: false,
			desc:    "valid small length decodes successfully",
		},
		{
			name: "valid medium length still works",
			// Field 1, wire type 2, length 200, followed by 200 bytes
			data: func() []byte {
				data := []byte{0x0a, 0xc8, 0x01} // Field 1, length 200 (varint)
				for i := 0; i < 200; i++ {
					data = append(data, byte('A'+i%26))
				}
				return data
			}(),
			wantErr: false,
			desc:    "valid medium length (200 bytes) decodes successfully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields, err := extractProtobufFields(tt.data)

			if tt.wantErr {
				if err == nil {
					t.Errorf("extractProtobufFields() expected error for %s, got nil", tt.desc)
				}
			} else {
				if err != nil {
					t.Errorf("extractProtobufFields() unexpected error for %s: %v", tt.desc, err)
				}
				if fields == nil {
					t.Errorf("extractProtobufFields() returned nil for %s", tt.desc)
				}
			}
		})
	}
}

func TestDecodeProtobufStrings_OversizedLength(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want []string
	}{
		{
			name: "length exceeds buffer size",
			data: []byte{0x0a, 0xe8, 0x07, 'H', 'e', 'l', 'l', 'o'},
		},
		{
			name: "length overflows when cast to int",
			data: []byte{0x0a, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x01},
		},
		{
			name: "valid small length still works",
			data: []byte{0x0a, 0x05, 'H', 'e', 'l', 'l', 'o'},
			want: []string{"Hello"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeProtobufStrings(tt.data)
			if err != nil {
				t.Fatalf("decodeProtobufStrings() unexpected error: %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("decodeProtobufStrings() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTryProtobufDecode_OversizedLength(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
		desc string
	}{
		{
			name: "oversized length returns false not panic",
			data: []byte{0x0a, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x01},
			want: false,
			desc: "length overflows int on 64-bit systems",
		},
		{
			name: "valid data still returns true",
			data: []byte{0x0a, 0x05, 'H', 'e', 'l', 'l', 'o'},
			want: true,
			desc: "valid small length decodes successfully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := tryProtobufDecode(tt.data)
			if got != tt.want {
				t.Errorf("tryProtobufDecode() = %v, want %v (test: %s)", got, tt.want, tt.desc)
			}
		})
	}
}
