package structconf

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/iancoleman/strcase"
)

// commonInitialisms is a set of common initialisms.
// taken from https://github.com/golang/lint/blob/master/lint.go#L770
var commonInitialisms = map[string]bool{
	"ACL":   true,
	"API":   true,
	"ASCII": true,
	"CPU":   true,
	"CSS":   true,
	"DNS":   true,
	"EOF":   true,
	"GUID":  true,
	"HTML":  true,
	"HTTP":  true,
	"HTTPS": true,
	"ID":    true,
	"IP":    true,
	"JSON":  true,
	"LHS":   true,
	"QPS":   true,
	"RAM":   true,
	"RHS":   true,
	"RPC":   true,
	"SLA":   true,
	"SMTP":  true,
	"SQL":   true,
	"SSH":   true,
	"TCP":   true,
	"TLS":   true,
	"TTL":   true,
	"UDP":   true,
	"UI":    true,
	"UID":   true,
	"UUID":  true,
	"URI":   true,
	"URL":   true,
	"UTF8":  true,
	"VM":    true,
	"XML":   true,
	"XMPP":  true,
	"XSRF":  true,
	"XSS":   true,
}

func init() { //nolint:gochecknoinits
	for initialism := range commonInitialisms {
		strcase.ConfigureAcronym(strings.ToUpper(initialism), strings.ToLower(initialism))
	}
}

type configFieldTags struct {
	flag     string
	aliases  []string
	isGlobal bool
	isSecret bool

	json string
	toml string
	yaml string

	env string

	defaultValue string
	help         string
}

func parseTags(tag *reflect.StructTag) *configFieldTags {
	isGlobal, _ := strconv.ParseBool(tag.Get("global"))
	isSecret, _ := strconv.ParseBool(tag.Get("secret"))

	parsed := &configFieldTags{
		flag:     tag.Get("flag"),
		isGlobal: isGlobal,
		isSecret: isSecret,

		json: tag.Get("json"),
		toml: tag.Get("toml"),
		yaml: tag.Get("yaml"),

		env:          tag.Get("env"),
		defaultValue: tag.Get("default"),
		help:         tag.Get("help"),
	}
	alias := tag.Get("alias")
	if alias != "" {
		parts := strings.Split(alias, ",")
		for _, part := range parts {
			if strings.HasPrefix(part, "-") {
				parsed.aliases = append(parsed.aliases, strings.TrimPrefix(part, "-"))
			}
		}
	}

	return parsed
}

func parseTagsWithFieldNameDefault(tag *reflect.StructTag, fieldName string) *configFieldTags {
	tags := parseTags(tag)
	isExported := len(fieldName) > 0 && fieldName[0] >= 'A' && fieldName[0] <= 'Z'

	kebab := strcase.ToKebab(fieldName)

	if isExported && tags.flag == "" {
		tags.flag = kebab
	}
	if isExported && tags.json == "" {
		tags.json = strcase.ToLowerCamel(fieldName)
	}
	if isExported && tags.toml == "" {
		tags.toml = kebab
	}
	if isExported && tags.yaml == "" {
		tags.yaml = kebab
	}
	if isExported && tags.env == "" {
		tags.env = strcase.ToScreamingSnake(fieldName)
	}
	return tags
}
