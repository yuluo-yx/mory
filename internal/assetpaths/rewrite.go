// Package assetpaths rewrites companion asset URLs while preserving literal code.
package assetpaths

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var (
	fenceStart   = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})(.*)$")
	fenceEnd     = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})[\\t ]*$")
	indentedCode = regexp.MustCompile("(?m)^(?: {4}|\\t)[^\\r\\n]*")
	htmlTag      = regexp.MustCompile(`<\/?[A-Za-z][A-Za-z0-9:-]*(?:\s+(?:[^>"']|"[^"]*"|'[^']*')*)?\/?>`)
	imageTag     = regexp.MustCompile(`(?i)^<img\b`)
	imageSource  = regexp.MustCompile("(?i)(\\s+src\\s*=\\s*)(?:\"([^\"]*)\"|'([^']*)'|([^\\s\"'=<>`]+))")
	markdownURL  = regexp.MustCompile(`(!?\[[^\]\r\n]*\]\(\s*)(?:<([^>\r\n]+)>|([^\s)]+))`)
	htmlComments = regexp.MustCompile(`(?s)<!--[\s\S]*?-->`)
)

// Rewrite changes only the leading companion directory in supported relative URLs.
func Rewrite(source, from, to string) string {
	if from == "" || from == to {
		return source
	}
	prefix := "\uE100"
	for strings.Contains(source, prefix) || strings.Contains(from, prefix) || strings.Contains(to, prefix) {
		prefix += "\uE100"
	}
	tokens := []string{}
	tokenPattern := regexp.MustCompile(regexp.QuoteMeta(prefix) + `(\d+)` + "\uE101")
	restore := func(text string) string {
		return tokenPattern.ReplaceAllStringFunc(text, func(token string) string {
			index, _ := strconv.Atoi(tokenPattern.FindStringSubmatch(token)[1])
			return tokens[index]
		})
	}
	protect := func(text string) string {
		value := restore(text)
		tokens = append(tokens, value)
		return prefix + strconv.Itoa(len(tokens)-1) + "\uE101"
	}
	var result strings.Builder
	var fence string
	var block strings.Builder
	for _, line := range strings.SplitAfter(source, "\n") {
		content := strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if fence != "" {
			block.WriteString(line)
			if end := fenceEnd.FindStringSubmatch(content); end != nil && end[1][0] == fence[0] && len(end[1]) >= len(fence) {
				value := block.String()
				ending := value[len(strings.TrimRight(value, "\r\n")):]
				result.WriteString(protect(strings.TrimSuffix(value, ending)) + ending)
				fence = ""
				block.Reset()
			}
		} else if start := fenceStart.FindStringSubmatch(content); start != nil && (start[1][0] != '`' || !strings.Contains(start[2], "`")) {
			fence = start[1]
			block.WriteString(line)
		} else {
			result.WriteString(line)
		}
	}
	if fence != "" {
		result.WriteString(protect(block.String()))
	}
	text := result.String()
	for _, tag := range []string{"pre", "code", "script", "style"} {
		pattern := regexp.MustCompile(`(?is)<` + tag + `\b[^>]*>[\s\S]*?</` + tag + `\s*>`)
		text = pattern.ReplaceAllStringFunc(text, protect)
	}
	text = htmlComments.ReplaceAllStringFunc(text, protect)
	text = protectInlineCode(text, protect)
	text = indentedCode.ReplaceAllStringFunc(text, protect)
	text = htmlTag.ReplaceAllStringFunc(text, func(tag string) string {
		if imageTag.MatchString(tag) {
			tag = imageSource.ReplaceAllStringFunc(tag, func(attribute string) string {
				parts := imageSource.FindStringSubmatchIndex(attribute)
				for group := 2; group <= 4; group++ {
					start, end := parts[group*2], parts[group*2+1]
					if start >= 0 {
						return attribute[:start] + rewriteURL(attribute[start:end], from, to, true) + attribute[end:]
					}
				}
				return attribute
			})
		}
		return protect(tag)
	})
	var rewritten strings.Builder
	previous := 0
	for _, match := range markdownURL.FindAllStringSubmatchIndex(text, -1) {
		escaped := 0
		for index := match[0] - 1; index >= 0 && text[index] == '\\'; index-- {
			escaped++
		}
		if escaped%2 != 0 {
			continue
		}
		start, end := match[4], match[5]
		if start < 0 {
			start, end = match[6], match[7]
		}
		rewritten.WriteString(text[previous:start])
		rewritten.WriteString(rewriteURL(text[start:end], from, to, false))
		previous = end
	}
	rewritten.WriteString(text[previous:])
	return restore(rewritten.String())
}

func rewriteURL(value, from, to string, html bool) string {
	leading := ""
	rest := value
	for strings.HasPrefix(rest, "./") {
		leading += "./"
		rest = rest[2:]
	}
	slash := strings.IndexByte(rest, '/')
	if slash < 0 {
		return value
	}
	encoded := rest[:slash]
	directory := encoded
	if html {
		directory = strings.ReplaceAll(directory, "&amp;", "&")
	}
	directory, err := url.PathUnescape(directory)
	if err != nil || directory != from {
		return value
	}
	replacement := to
	if strings.Contains(encoded, "%") {
		replacement = strings.ReplaceAll(url.QueryEscape(to), "+", "%20")
	}
	if html {
		replacement = strings.ReplaceAll(replacement, "&", "&amp;")
	}
	return leading + replacement + rest[slash:]
}

func protectInlineCode(text string, protect func(string) string) string {
	var result strings.Builder
	for len(text) > 0 {
		start := strings.IndexByte(text, '`')
		if start < 0 {
			result.WriteString(text)
			break
		}
		run := 1
		for start+run < len(text) && text[start+run] == '`' {
			run++
		}
		end := start + run
		found := false
		for end < len(text) {
			next := strings.IndexByte(text[end:], '`')
			if next < 0 {
				end = len(text)
				break
			}
			next += end
			count := 1
			for next+count < len(text) && text[next+count] == '`' {
				count++
			}
			end = next + count
			if count == run {
				found = true
				break
			}
		}
		result.WriteString(text[:start])
		if found {
			result.WriteString(protect(text[start:end]))
			text = text[end:]
		} else {
			result.WriteString(text[start : start+run])
			text = text[start+run:]
		}
	}
	return result.String()
}
