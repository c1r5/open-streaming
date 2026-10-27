package application

import "strings"

func SRTToVTT(body string) string {
	body = strings.TrimPrefix(body, "\uFEFF")
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	if strings.HasPrefix(strings.TrimSpace(body), "WEBVTT") {
		return body
	}
	var out strings.Builder
	out.WriteString("WEBVTT\n\n")
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, "-->") {
			line = strings.ReplaceAll(line, ",", ".")
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return out.String()
}
