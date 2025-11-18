package metadata

import "strings"

func BuildName(name, group string) string {
	builder := strings.Builder{}
	builder.WriteString(name)
	builder.WriteRune('-')
	builder.WriteString(group)

	return builder.String()
}
