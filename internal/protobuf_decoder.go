package internal

import (
	"encoding/binary"
	"fmt"
	"math"
)

// DecodeProtobufStrings extracts all length-delimited strings from protobuf-encoded data
// Protobuf wire format: [field_number << 3 | wire_type] [length] [data]
// Wire type 2 = length-delimited (strings, embedded messages, packed repeated fields)
func DecodeProtobufStrings(data []byte) ([]string, error) {
	return decodeProtobufStrings(data)
}

// decodeProtobufStrings is the internal implementation
func decodeProtobufStrings(data []byte) ([]string, error) {
	var strings []string
	offset := 0

	for offset < len(data) {
		if offset+1 > len(data) {
			break
		}

		// Read tag byte: [field_number << 3 | wire_type]
		tag := data[offset]
		offset++

		wireType := tag & 0x07
		_ = tag >> 3 // fieldNum (not used in this function)

		// Only handle wire type 2 (length-delimited)
		if wireType != 2 {
			// Skip other wire types (0=varint, 1=64-bit, 5=32-bit)
			switch wireType {
			case 0:
				// Varint - skip bytes until we find a non-continuation byte
				for offset < len(data) && (data[offset]&0x80) != 0 {
					offset++
				}
				if offset < len(data) {
					offset++
				}
			case 1:
				// 64-bit - skip 8 bytes
				offset += 8
			case 5:
				// 32-bit - skip 4 bytes
				offset += 4
			}
			continue
		}

		// Read length (varint)
		length, lengthBytes := decodeVarint(data[offset:])
		if lengthBytes == 0 {
			break // Invalid varint
		}
		offset += lengthBytes

		// Safely validate and convert length
		intLength, ok := safeLength(length, offset, len(data))
		if !ok {
			break // Length is unsafe (too large, negative, or exceeds buffer)
		}

		stringData := data[offset : offset+intLength]
		offset += intLength

		// Try to extract readable strings
		// Check if it's valid UTF-8 and mostly readable
		if isReadableText(string(stringData)) {
			strings = append(strings, string(stringData))
		} else {
			// Try to find JSON or other readable content within
			if jsonBytes, found := extractJSONFromBinary(stringData); found {
				strings = append(strings, string(jsonBytes))
			}
		}
	}

	return strings, nil
}

// decodeVarint decodes a protobuf varint and returns the value and number of bytes consumed
func decodeVarint(data []byte) (uint64, int) {
	var result uint64
	var shift uint

	for i, b := range data {
		if i >= 10 { // Varints can be at most 10 bytes
			return 0, 0
		}
		if i == 9 && b > 1 {
			return 0, 0
		}

		result |= uint64(b&0x7F) << shift

		if (b & 0x80) == 0 {
			return result, i + 1
		}

		shift += 7
	}

	return 0, 0
}

// safeLength checks if a uint64 length can be safely used for slicing.
// Returns the length as int and true if safe, or 0 and false if unsafe.
func safeLength(length uint64, offset int, dataLen int) (int, bool) {
	// Check if length fits in an int (prevents negative values after cast)
	if length > math.MaxInt {
		return 0, false
	}

	intLength := int(length)

	// Check if intLength is negative (shouldn't happen after above check, but be defensive)
	if intLength < 0 {
		return 0, false
	}

	// Check if offset + length would overflow or exceed buffer size
	if offset > dataLen || intLength > dataLen-offset {
		return 0, false
	}

	return intLength, true
}

// extractProtobufFields extracts all fields from protobuf data and returns them as a map
// This is a simplified decoder that focuses on extracting readable content
func extractProtobufFields(data []byte) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	offset := 0
	fieldCount := 0

	for offset < len(data) {
		if offset+1 > len(data) {
			break
		}

		// Read tag
		tag := data[offset]
		offset++

		wireType := tag & 0x07
		fieldNum := tag >> 3

		fieldKey := fmt.Sprintf("field_%d", fieldNum)
		fieldCount++

		switch wireType {
		case 0: // Varint
			value, bytesRead := decodeVarint(data[offset:])
			if bytesRead == 0 {
				return result, fmt.Errorf("invalid varint at offset %d", offset)
			}
			result[fieldKey] = value
			offset += bytesRead

		case 1: // 64-bit (fixed64)
			if offset+8 > len(data) {
				return result, fmt.Errorf("not enough data for 64-bit at offset %d", offset)
			}
			value := binary.LittleEndian.Uint64(data[offset : offset+8])
			result[fieldKey] = value
			offset += 8

		case 2: // Length-delimited (string, bytes, embedded message)
			length, lengthBytes := decodeVarint(data[offset:])
			if lengthBytes == 0 {
				return result, fmt.Errorf("invalid length varint at offset %d", offset)
			}
			offset += lengthBytes

			// Safely validate and convert length
			intLength, ok := safeLength(length, offset, len(data))
			if !ok {
				return result, fmt.Errorf("unsafe length %d at offset %d (buffer size %d)", length, offset, len(data))
			}

			fieldData := data[offset : offset+intLength]
			offset += intLength

			// Try to decode as string
			if isReadableText(string(fieldData)) {
				result[fieldKey] = string(fieldData)
			} else {
				// Try to extract JSON
				if jsonBytes, found := extractJSONFromBinary(fieldData); found {
					result[fieldKey] = string(jsonBytes)
				} else {
					// Try to decode nested protobuf
					if nestedFields, err := extractProtobufFields(fieldData); err == nil && len(nestedFields) > 0 {
						result[fieldKey] = nestedFields
					} else {
						// Store as hex for debugging
						result[fieldKey] = fmt.Sprintf("[binary: %d bytes]", len(fieldData))
					}
				}
			}

		case 5: // 32-bit (fixed32)
			if offset+4 > len(data) {
				return result, fmt.Errorf("not enough data for 32-bit at offset %d", offset)
			}
			value := binary.LittleEndian.Uint32(data[offset : offset+4])
			result[fieldKey] = value
			offset += 4

		default:
			return result, fmt.Errorf("unknown wire type %d at offset %d", wireType, offset-1)
		}

		// Safety check - don't process more than 100 fields
		if fieldCount > 100 {
			break
		}
	}

	return result, nil
}

// TryProtobufDecode attempts to decode binary data as protobuf and extract readable content
func TryProtobufDecode(data []byte) (map[string]interface{}, bool) {
	return tryProtobufDecode(data)
}

// tryProtobufDecode is the internal implementation
func tryProtobufDecode(data []byte) (map[string]interface{}, bool) {
	// Check if it looks like protobuf (starts with valid tag bytes)
	if len(data) == 0 {
		return nil, false
	}

	firstByte := data[0]
	wireType := firstByte & 0x07

	// Valid protobuf should have reasonable field numbers and wire types (0-5)
	// Field numbers are varints, but we check the first byte only
	if wireType > 5 {
		return nil, false
	}

	// Try to decode
	fields, err := extractProtobufFields(data)
	if err != nil {
		return nil, false
	}

	if len(fields) == 0 {
		return nil, false
	}

	return fields, true
}
