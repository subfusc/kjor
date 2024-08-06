package main

import (
	"bytes"
	"fmt"
)

type Color [3]byte

func (c Color) EscSequence(fg bool) string {
	if fg {
		return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c[0], c[1], c[2])
	}
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", c[0], c[1], c[2])
}

type AnsiColorBuilder struct {
	fgColor Color
	bgColor Color
	fgSet   bool
	bgSet   bool
	content *bytes.Buffer
}

func (acb *AnsiColorBuilder) String() string {
	colorString := ""
	if acb.fgSet {
		colorString += acb.fgColor.EscSequence(true)
	}
	if acb.bgSet {
		colorString += acb.bgColor.EscSequence(false)
	}

	return fmt.Sprintf("%s%s\x1b[0m", colorString, acb.content.String())
}

func NewAnsiColorBuilder(text string) *AnsiColorBuilder {
	return &AnsiColorBuilder{content: bytes.NewBuffer([]byte(text))}
}

func (acb *AnsiColorBuilder) Colorize(fg Color, bg Color) {
	acb.Fg(fg)
	acb.Bg(bg)
}

func (acb *AnsiColorBuilder) Fg(c Color) {
	acb.fgColor = c
	acb.fgSet = true
}

func (acb *AnsiColorBuilder) Bg(c Color) {
	acb.bgColor = c
	acb.bgSet = true
}
