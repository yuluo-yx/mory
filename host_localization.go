package mory

import (
	"encoding/json"
	"strings"
)

var hostMessages = func() map[string]string {
	data, err := webAssets.ReadFile("Sources/Mory/Web/host-messages.json")
	if err != nil {
		panic(err)
	}
	var messages map[string]string
	if err := json.Unmarshal(data, &messages); err != nil {
		panic(err)
	}
	return messages
}()

// HostMessage translates host-owned labels and error prefixes, preserving external details.
func HostMessage(message, locale string) string {
	if translated, ok := hostMessages[message]; locale == "en" && ok {
		return translated
	}
	text := strings.TrimSuffix(strings.TrimSuffix(message, "。"), ".")
	if locale != "en" {
		if strings.EqualFold(text, "Path must remain inside the selected directory") {
			return "路径必须位于所选目录内。"
		}
		return message
	}
	if translated, ok := hostMessages[text]; ok {
		return translated
	}
	if prefix, detail, ok := strings.Cut(text, "："); ok {
		if translated, exists := hostMessages[prefix]; exists {
			return translated + ": " + HostMessage(detail, locale)
		}
	}
	return message
}
