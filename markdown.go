package main

import (
	"fmt"
	"strings"
)

// Escape user input outside of inline code blocks
func Escape(userInput string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"`", "\\`",
		"*", "\\*",
		"_", "\\_",
		"{", "\\{",
		"}", "\\}",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		".", "\\.",
		"!", "\\!",
	)

	return replacer.Replace(userInput)
}

// WrapInInlineCodeBlock puts the user input into a inline codeblock that is properly escaped.
func WrapInInlineCodeBlock(userInput string) (userOutput string) {
	numberBackticks := strings.Count(userInput, "`") + 1

	userOutput = userInput
	for idx := 0; idx < numberBackticks; idx++ {
		userOutput = fmt.Sprintf("`%s`", userOutput)
	}
	return
}
