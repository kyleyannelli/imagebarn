package main

import (
	"fmt"
	"testing"

	"kmfg.dev/imagebarn/v1/filestore"
)

var (
	testEncStrings = []string{
		"0#",
		"1#a",
		"1##",
		"4##abc",
		"4#abc#",
		"7#abc#def",
		"9#abc###def",
		"10#1234567890",
		"12#123#456#7890",
		"17#10#This is a test",
		"28#!@#$%^&*()_+-=[]{}|;':,./<>?",
		"3#   ",
		"8#abc\n\tdef",
		"7#こんにちは世界",
		"3#😀🚀🌟",
		"3#\x00\x01\x02",
		"6#123abc",
		"7#123#abc",
		"9#ababababa",
		"9#   ###   ",
		"4#©®™✓",
		"20#Line1\\nLine2\\tTabbed",
	}
	testRawStrings = []string{
		"",
		"a",
		"#",
		"#abc",
		"abc#",
		"abc#def",
		"abc###def",
		"1234567890",
		"123#456#7890",
		"10#This is a test",
		"!@#$%^&*()_+-=[]{}|;':,./<>?",
		"   ",
		"abc\n\tdef",
		"こんにちは世界",
		"😀🚀🌟",
		"\x00\x01\x02",
		"123abc",
		"123#abc",
		"ababababa",
		"   ###   ",
		"©®™✓",
		"Line1\\nLine2\\tTabbed",
	}
)

func TestDecode(t *testing.T) {
	for i := 0; i < len(testEncStrings); i++ {
		decodedStr, err := filestore.Decode(testEncStrings[i])
		if err != nil {
			t.Fatalf("Failed to decode %v: %v", testEncStrings[i], err)
		}
		if decodedStr != testRawStrings[i] {
			t.Fatalf("Decoded to %v but wanted %v", decodedStr, testEncStrings[i])
		}
		t.Log(fmt.Sprintf("%v : %v", decodedStr, testRawStrings[i]))
	}
}

func TestEncode(t *testing.T) {
	for i := 0; i < len(testRawStrings); i++ {
		encodedStr := filestore.Encode(testRawStrings[i])
		if encodedStr != testEncStrings[i] {
			t.Fatalf("Encoded to %v but wanted %v", encodedStr, testEncStrings[i])
		}
		t.Log(fmt.Sprintf("%v : %v", encodedStr, testEncStrings[i]))
	}
}
