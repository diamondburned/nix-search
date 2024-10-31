package main

import "strings"

type textStyler uint8

const (
	styledText textStyler = 1 << iota
	dontEndStyle
)

func (s textStyler) strikethrough(text string) string {
	return s.styleTextBlock(text, "\x1b[9m", "\x1b[29m")
}

func (s textStyler) dim(text string) string {
	return s.styleTextBlock(text, "\x1b[2m", "\x1b[22m")
}

func (s textStyler) bold(text string) string {
	return s.styleTextBlock(text, "\x1b[1m", "\x1b[22m")
}

func (s textStyler) underline(text string) string {
	return s.styleTextBlock(text, "\x1b[4m", "\x1b[24m")
}

func (s textStyler) with(o textStyler) textStyler {
	return s | o
}

func (s textStyler) style(text, prefix, suffix string) string {
	if s&1 == 0 {
		return text
	}
	if s&dontEndStyle != 0 {
		suffix = ""
	}
	return prefix + text + suffix
}

func (s textStyler) styleTextBlock(text string, prefix, suffix string) string {
	if s&1 == 0 {
		return text
	}
	if s&dontEndStyle != 0 {
		suffix = ""
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = prefix + line + suffix
	}
	return strings.Join(lines, "\n")
}
