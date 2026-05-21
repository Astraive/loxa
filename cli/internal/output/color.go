package output

import (
	"fmt"
	"os"
)

var NoColor = os.Getenv("NO_COLOR") != ""

const (
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorCyan   = "\033[36m"
	ColorBold   = "\033[1m"
	ColorDim    = "\033[2m"
	ColorReset  = "\033[0m"
)

func Colorize(color, text string) string {
	if NoColor {
		return text
	}
	return color + text + ColorReset
}

func Success(text string) string  { return Colorize(ColorGreen, text) }
func Error(text string) string    { return Colorize(ColorRed, text) }
func Warning(text string) string  { return Colorize(ColorYellow, text) }
func Info(text string) string     { return Colorize(ColorCyan, text) }
func Bold(text string) string     { return Colorize(ColorBold, text) }
func Dim(text string) string      { return Colorize(ColorDim, text) }

func SuccessLabel() string { return Success("ok") }
func ErrorLabel() string   { return Error("error") }

func PrintSection(title string) {
	fmt.Println(Bold(title))
}
