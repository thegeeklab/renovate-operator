// Package sanitize provides escaping helpers that produce safe attribute
// values for templ attributes that interpolate user-controlled strings into
// either data attributes or URL query parameters.
package sanitize

import (
	"encoding/json"
	"net/url"
	"strconv"
)

// mustMarshalString is json.Marshal restricted to the string type.
func mustMarshalString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic("sanitize: json.Marshal(string) returned an error: " + err.Error())
	}

	return string(b)
}

// JSString returns a JavaScript string literal (with surrounding double
// quotes and JSON-style escaping) safe to embed in inline expressions.
func JSString(s string) string {
	return mustMarshalString(s)
}

// JSValue returns a JavaScript literal (string, number, bool) safe to embed
// in an inline expression.
func JSValue(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic("sanitize: json.Marshal returned an error for " + err.Error())
	}

	return string(b)
}

// BoolAttr returns "true" or "false" for use in a data attribute.
func BoolAttr(b bool) string {
	return strconv.FormatBool(b)
}

// QueryEscape escapes a string for use as an HTTP query parameter value,
// safe to embed. Reserved characters are percent-encoded so that user-controlled
// values cannot inject additional parameters or break out of the URL.
func QueryEscape(s string) string {
	return url.QueryEscape(s)
}

// GitreposURL builds a /gitrepos URL with safely escaped query parameters.
func GitreposURL(namespace, renovatorUID string) string {
	return "/gitrepos?namespace=" + QueryEscape(namespace) +
		"&renovator=" + QueryEscape(renovatorUID)
}

// RenovatorCountURL builds a /renovators/count URL with safely escaped query parameters.
func RenovatorCountURL(namespace, renovatorUID string) string {
	return "/renovators/count?namespace=" + QueryEscape(namespace) +
		"&renovator=" + QueryEscape(renovatorUID)
}

// GitrepoURL builds a /gitrepo URL with safely escaped query parameters.
func GitrepoURL(namespace, name string) string {
	return "/gitrepo?namespace=" + QueryEscape(namespace) +
		"&name=" + QueryEscape(name)
}

// JobLogsURL builds a /joblogs URL with safely escaped query parameters.
func JobLogsURL(namespace, runner, job string) string {
	return "/joblogs?namespace=" + QueryEscape(namespace) +
		"&runner=" + QueryEscape(runner) +
		"&job=" + QueryEscape(job)
}

// JobLogsDownloadURL builds a /joblogs/download URL with safely escaped query parameters.
func JobLogsDownloadURL(namespace, job string) string {
	return "/joblogs/download?namespace=" + QueryEscape(namespace) +
		"&job=" + QueryEscape(job)
}

// PersistKey returns a storage key for per-repo persisted state. The key
// is a plain string; templ handles HTML escaping when rendering it into
// a data attribute.
func PersistKey(namespace, name string) string {
	return "repo-" + namespace + "-" + name
}

// RenovatorPersistKey returns a storage key for the open/closed state of a
// Renovator details element.
func RenovatorPersistKey(name string) string {
	return "renovator-" + name
}

// SortFieldPersistKey returns a storage key for the sort field of a
// Renovator's repository list.
func SortFieldPersistKey(name string) string {
	return "sort-field-" + name
}

// SortOrderPersistKey returns a storage key for the sort order of a
// Renovator's repository list.
func SortOrderPersistKey(name string) string {
	return "sort-order-" + name
}
