package output

import (
	"fmt"
	"strings"
)

func PrintTable(headers []string, rows [][]string) {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	for i, h := range headers {
		if i > 0 {
			fmt.Print("  ")
		}
		fmt.Printf("%-*s", widths[i], Bold(h))
	}
	fmt.Println()
	for i := range headers {
		if i > 0 {
			fmt.Print("  ")
		}
		fmt.Printf("%s", strings.Repeat("-", widths[i]))
	}
	fmt.Println()
	for _, row := range rows {
		for i, cell := range row {
			if i > 0 {
				fmt.Print("  ")
			}
			if i < len(widths) {
				fmt.Printf("%-*s", widths[i], cell)
			}
		}
		fmt.Println()
	}
}

func PrintKeyValue(pairs map[string]string) {
	maxLen := 0
	for k := range pairs {
		if len(k) > maxLen {
			maxLen = len(k)
		}
	}
	for k, v := range pairs {
		fmt.Printf("  %-*s  %s\n", maxLen, Dim(k), v)
	}
}
