// Package sanitize provides escaping helpers that produce safe attribute
// values for templ attributes that interpolate user-controlled strings into
// either JavaScript expressions (Alpine x-data / @click) or URL query
// parameters (htmx hx-get / href).
//
// Templ auto-escapes HTML attribute values for ordinary string interpolation,
// but it does NOT inspect the contents of attribute values for JavaScript
// or URL context. Passing a raw user-controlled string into an Alpine
// expression or a query parameter is a stored-XSS vector: a value containing
// ', ", <, >, or whitespace can break out of the surrounding JS string or
// URL. The helpers in this package produce output that is always safe to
// drop into the corresponding attribute, regardless of input.
package sanitize

import (
	"encoding/json"
	"net/url"
)

// mustMarshalString is json.Marshal restricted to the string type. The
// encoding/json package documents that marshalling a string never returns an
// error, so any error here is a programmer bug rather than a runtime
// condition. Panicking surfaces that bug immediately at the call site.
func mustMarshalString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic("sanitize: json.Marshal(string) returned an error: " + err.Error())
	}

	return string(b)
}

// mustMarshalValue is json.Marshal for arbitrary JSON-serializable values.
// For the value types the package passes through (string, bool, int, float)
// marshalling never fails. Any other type would indicate a programmer error
// and is surfaced via panic.
func mustMarshalValue(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic("sanitize: json.Marshal returned an error for " + err.Error())
	}

	return string(b)
}

// JSString returns a JavaScript string literal (with surrounding double
// quotes and JSON-style escaping) safe to embed in an inline Alpine x-data,
// @click, or hx-vals expression. The output is itself a valid JS string and
// is safe even when the input contains ', ", <, >, newlines, or other
// characters that would otherwise break out of the surrounding JS string.
func JSString(s string) string {
	return mustMarshalString(s)
}

// JSValue returns a JavaScript literal (string, number, bool) safe to embed
// in an inline Alpine expression.
func JSValue(v any) string {
	return mustMarshalValue(v)
}

// QueryEscape escapes a string for use as an HTTP query parameter value,
// safe to embed in an hx-get / href URL attribute. Reserved characters are
// percent-encoded so that user-controlled values cannot inject additional
// parameters or break out of the URL.
func QueryEscape(s string) string {
	return url.QueryEscape(s)
}

// GitreposURL builds a /gitrepos URL with safely escaped query parameters.
// The sort and order parameters are supplied at request time via hx-vals and
// are therefore not included in the base URL.
func GitreposURL(namespace, renovatorUID string) string {
	return "/gitrepos?namespace=" + QueryEscape(namespace) +
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

// RenovatorOpenXData returns an Alpine x-data expression that persists the
// open/closed state of a Renovator details element. The key is namespaced
// and the namespace segment is JSON-escaped so a Renovator whose name
// contains quotes, slashes, or unicode separators cannot break out of the
// JS string.
func RenovatorOpenXData(name string) string {
	return "{ open: $persist(false).as(" + JSString("renovator-"+name) + ") }"
}

// RepoSortXData returns an Alpine x-data expression that persists the sort
// field and order for a Renovator's repository list. Keys are scoped per
// Renovator so each panel remembers its own sort state across page reloads.
func RepoSortXData(name string) string {
	return "{ sort: $persist(" + JSString("name") + ").as(" + JSString("sort-field-"+name) + "), " +
		"order: $persist(" + JSString("asc") + ").as(" + JSString("sort-order-"+name) + ") }"
}

// JobListXData returns an Alpine x-data expression that references the
// per-repo jobList Alpine component. The composite key is JSON-escaped.
func JobListXData(repoNamespace, repoName string) string {
	return "jobList(" + JSString(repoNamespace+"-"+repoName) + ")"
}

// LogViewerXData returns an Alpine x-data expression that initializes the
// logViewer component for a specific job. All four arguments are
// JSON-escaped.
func LogViewerXData(namespace, runner, jobName string, isRunning bool) string {
	return "logViewer(" + JSString(namespace) + ", " +
		JSString(runner) + ", " +
		JSString(jobName) + ", " +
		JSValue(isRunning) + ")"
}

// SelectJobExpr returns an Alpine @click expression that selects a job. The
// job's log URL is read from the button's hx-get attribute (rendered
// server-side by JobLogsURL) so the URL schema lives in one place.
func SelectJobExpr(name string) string {
	return "selectJob($event.currentTarget.getAttribute('hx-get'), " + JSString(name) + ")"
}
