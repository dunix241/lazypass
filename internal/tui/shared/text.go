package shared

import (
	"unicode"
	"unicode/utf8"
)

func DeleteLastRune(value string) string {
	_, size := utf8.DecodeLastRuneInString(value)
	return value[:len(value)-size]
}

func DeletePreviousWord(value string) string {
	runes := []rune(value)
	for len(runes) > 0 && unicode.IsSpace(runes[len(runes)-1]) {
		runes = runes[:len(runes)-1]
	}
	for len(runes) > 0 && !unicode.IsSpace(runes[len(runes)-1]) && runes[len(runes)-1] != '/' {
		runes = runes[:len(runes)-1]
	}
	for len(runes) > 0 && unicode.IsSpace(runes[len(runes)-1]) {
		runes = runes[:len(runes)-1]
	}
	return string(runes)
}
