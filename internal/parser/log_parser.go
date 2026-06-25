package parser

import (
	"bufio"
	"encoding/json"
	"html"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	MaxPRDetails    = 100
	MaxLogIssues    = 20
	MaxIssueMsgLen  = 256
	MaxFormattedLen = 1024

	scannerInitialBuf = 64 * 1024
	scannerMaxBuf     = 1024 * 1024

	levelWarn  = 40
	levelError = 50

	regexMatchLen = 2

	actionOrderAutomerged    = 0
	actionOrderCreated       = 1
	actionOrderUpdated       = 2
	actionOrderNeedsApproval = 3
	actionOrderUnchanged     = 4
)

type PRAction string

const (
	PRActionAutomerged    PRAction = "automerged"
	PRActionCreated       PRAction = "created"
	PRActionUpdated       PRAction = "updated"
	PRActionNeedsApproval PRAction = "needs-approval"
	PRActionUnchanged     PRAction = "unchanged"
)

type LogLevel int

const (
	LogLevelTrace LogLevel = 10
	LogLevelDebug LogLevel = 20
	LogLevelInfo  LogLevel = 30
	LogLevelWarn  LogLevel = 40
	LogLevelError LogLevel = 50
	LogLevelFatal LogLevel = 60
)

type LogIssue struct {
	Level   int
	Message string
}

type PRDetail struct {
	Branch string
	Number int
	Title  string
	Action PRAction
	URL    string
}

type PRActivity struct {
	Automerged    int
	Created       int
	Updated       int
	NeedsApproval int
	Unchanged     int
	PRs           []PRDetail
	Truncated     bool
}

type LogIssues struct {
	WarnCount  int
	ErrorCount int
	Issues     []LogIssue
	Truncated  bool
}

type FormattedLine struct {
	Level   LogLevel
	Message string
	Class   string
	Time    string
}

type ParseResult struct {
	HasIssues            bool
	RenovateResultStatus string
	PRActivity           *PRActivity
	LogIssues            *LogIssues
	Lines                []FormattedLine
}

type renovateLogEntry struct {
	Level int    `json:"level"`
	Msg   string `json:"msg"`
	Time  string `json:"time"`
}

type repositoryFinishedEntry struct {
	Msg    string `json:"msg"`
	Result string `json:"result,omitempty"`
	Status string `json:"status,omitempty"`
}

type prCreateUpdateEntry struct {
	Msg    string `json:"msg"`
	Branch string `json:"branch"`
	Title  string `json:"title"`
}

type prUnchangedEntry struct {
	Msg    string `json:"msg"`
	Branch string `json:"branch"`
}

type gitPushEntry struct {
	Msg    string `json:"msg"`
	Branch string `json:"branch"`
	Result struct {
		RemoteMessages struct {
			All []string `json:"all"`
		} `json:"remoteMessages"`
	} `json:"result"`
}

type prCreatedEntry struct {
	Msg     string `json:"msg"`
	Branch  string `json:"branch"`
	PR      int    `json:"pr"`
	PRTitle string `json:"prTitle"`
}

type branchInfoItem struct {
	BranchName string `json:"branchName"`
	PRNo       *int   `json:"prNo"`
	PRTitle    string `json:"prTitle"`
	Result     string `json:"result"`
}

type branchesInfoEntry struct {
	Msg          string           `json:"msg"`
	BranchesInfo []branchInfoItem `json:"branchesInformation"`
}

var (
	prURLRegex    = regexp.MustCompile(`https?://[^\s"]+/(?:pulls|pull|merge_requests)/(\d+)`)
	prNumberRegex = regexp.MustCompile(`Pull Request #(\d+)`)

	actionOrder = map[PRAction]int{
		PRActionAutomerged:    actionOrderAutomerged,
		PRActionCreated:       actionOrderCreated,
		PRActionUpdated:       actionOrderUpdated,
		PRActionNeedsApproval: actionOrderNeedsApproval,
		PRActionUnchanged:     actionOrderUnchanged,
	}

	levelClassMap = map[LogLevel]string{
		LogLevelTrace: "text-gray-500",
		LogLevelDebug: "text-purple-400",
		LogLevelInfo:  "text-white",
		LogLevelWarn:  "text-yellow-400",
		LogLevelError: "text-red-500 font-bold",
		LogLevelFatal: "text-white bg-red-600 font-bold px-1 rounded",
	}

	levelLabelMap = map[LogLevel]string{
		LogLevelTrace: "TRACE",
		LogLevelDebug: "DEBUG",
		LogLevelInfo:  "INFO",
		LogLevelWarn:  "WARN",
		LogLevelError: "ERROR",
		LogLevelFatal: "FATAL",
	}
)

func ParseRenovateLogs(logs string) *ParseResult {
	result := &ParseResult{}

	if logs == "" {
		return result
	}

	if !isNDJSON(logs) {
		return result
	}

	branchMap := make(map[string]*PRDetail)
	parsedAnyLine := false

	var warnCount, errorCount int

	var issues []LogIssue

	seenMessages := make(map[string]bool)

	issuesTruncated := false

	var lines []FormattedLine

	scanner := bufio.NewScanner(strings.NewReader(logs))
	scanner.Buffer(make([]byte, scannerInitialBuf), scannerMaxBuf)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry renovateLogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			lines = append(lines, formatRawLine(line))

			continue
		}

		parsedAnyLine = true
		level := LogLevel(entry.Level)

		lines = append(lines, formatLogLine(level, entry.Msg, entry.Time))

		if entry.Level >= levelWarn {
			result.HasIssues = true

			trackIssue(entry.Level, entry.Msg, &warnCount, &errorCount, &issues, seenMessages, &issuesTruncated)
		}

		processLogEntry(line, entry, branchMap, result)
	}

	if parsedAnyLine {
		result.PRActivity = buildPRActivity(branchMap)
		result.LogIssues = &LogIssues{
			WarnCount:  warnCount,
			ErrorCount: errorCount,
			Issues:     issues,
			Truncated:  issuesTruncated,
		}
	}

	result.Lines = lines

	return result
}

func trackIssue(
	level int,
	msg string,
	warnCount, errorCount *int,
	issues *[]LogIssue,
	seen map[string]bool,
	truncated *bool,
) {
	if level >= levelError {
		*errorCount++
	} else {
		*warnCount++
	}

	if len(msg) > MaxIssueMsgLen {
		msg = msg[:MaxIssueMsgLen] + "…"
	}

	if msg == "" || seen[msg] {
		return
	}

	seen[msg] = true

	if len(*issues) < MaxLogIssues {
		*issues = append(*issues, LogIssue{Level: level, Message: msg})
	} else {
		*truncated = true
	}
}

func processLogEntry(line string, entry renovateLogEntry, branchMap map[string]*PRDetail, result *ParseResult) {
	switch {
	case entry.Msg == "Repository finished":
		var finished repositoryFinishedEntry
		if err := json.Unmarshal([]byte(line), &finished); err == nil {
			result.RenovateResultStatus = resolveFinishResult(finished)
		}

	case entry.Msg == "Creating PR":
		var pr prCreateUpdateEntry
		if err := json.Unmarshal([]byte(line), &pr); err == nil && pr.Branch != "" {
			d := getOrCreateDetail(branchMap, pr.Branch)
			d.Action = PRActionCreated
			d.Title = pr.Title
		}

	case entry.Msg == "Updating PR":
		var pr prCreateUpdateEntry
		if err := json.Unmarshal([]byte(line), &pr); err == nil && pr.Branch != "" {
			d := getOrCreateDetail(branchMap, pr.Branch)
			d.Action = PRActionUpdated
			d.Title = pr.Title
		}

	case strings.Contains(entry.Msg, "does not need updating"):
		processUnchangedPR(line, entry.Msg, branchMap)

	case entry.Msg == "git push":
		processGitPush(line, branchMap)

	case entry.Msg == "PR created":
		var pc prCreatedEntry
		if err := json.Unmarshal([]byte(line), &pc); err == nil && pc.Branch != "" && pc.PR > 0 {
			d := getOrCreateDetail(branchMap, pc.Branch)
			d.Number = pc.PR

			if d.Title == "" && pc.PRTitle != "" {
				d.Title = pc.PRTitle
			}
		}

	case entry.Msg == "PR automerged":
		var pc prCreatedEntry
		if err := json.Unmarshal([]byte(line), &pc); err == nil && pc.Branch != "" {
			d := getOrCreateDetail(branchMap, pc.Branch)
			d.Action = PRActionAutomerged

			if pc.PR > 0 {
				d.Number = pc.PR
			}

			if d.Title == "" && pc.PRTitle != "" {
				d.Title = pc.PRTitle
			}
		}

	case entry.Msg == "branches info extended":
		processBranchesInfo(line, branchMap)
	}
}

func processUnchangedPR(line, msg string, branchMap map[string]*PRDetail) {
	matches := prNumberRegex.FindStringSubmatch(msg)
	if len(matches) != regexMatchLen {
		return
	}

	num, err := strconv.Atoi(matches[1])
	if err != nil {
		return
	}

	var unchanged prUnchangedEntry
	if err := json.Unmarshal([]byte(line), &unchanged); err != nil || unchanged.Branch == "" {
		return
	}

	d := getOrCreateDetail(branchMap, unchanged.Branch)
	d.Action = PRActionUnchanged
	d.Number = num
}

func processGitPush(line string, branchMap map[string]*PRDetail) {
	var gp gitPushEntry
	if err := json.Unmarshal([]byte(line), &gp); err != nil || gp.Branch == "" {
		return
	}

	for _, msg := range gp.Result.RemoteMessages.All {
		matches := prURLRegex.FindStringSubmatch(msg)
		if len(matches) != regexMatchLen {
			continue
		}

		d := getOrCreateDetail(branchMap, gp.Branch)
		d.URL = matches[0]

		if num, err := strconv.Atoi(matches[1]); err == nil {
			d.Number = num
		}

		break
	}
}

func processBranchesInfo(line string, branchMap map[string]*PRDetail) {
	var bi branchesInfoEntry
	if err := json.Unmarshal([]byte(line), &bi); err != nil {
		return
	}

	for _, b := range bi.BranchesInfo {
		if b.BranchName == "" {
			continue
		}

		if existing, exists := branchMap[b.BranchName]; exists {
			if existing.Title == "" && b.PRTitle != "" {
				existing.Title = b.PRTitle
			}

			continue
		}

		if !isActiveBranchResult(b.Result) {
			continue
		}

		d := getOrCreateDetail(branchMap, b.BranchName)

		if b.Result == "needs-approval" {
			d.Action = PRActionNeedsApproval
		} else {
			d.Action = PRActionUnchanged
		}

		d.Title = b.PRTitle

		if b.PRNo != nil {
			d.Number = *b.PRNo
		}
	}
}

func isActiveBranchResult(result string) bool {
	return result == "needs-approval" || result == "done" || result == "automerged" || result == ""
}

func resolveFinishResult(finished repositoryFinishedEntry) string {
	switch finished.Result {
	case "disabled-by-config":
		return "Disabled"
	case "disabled-closed-onboarding":
		return "Onboarding Closed"
	case "disabled-no-config":
		return "No Config"
	}

	if finished.Result == "" {
		if finished.Status == "onboarding" {
			return "Onboarding"
		}

		return "Unknown"
	}

	return finished.Result
}

func getOrCreateDetail(m map[string]*PRDetail, branch string) *PRDetail {
	if d, ok := m[branch]; ok {
		return d
	}

	d := &PRDetail{Branch: branch}
	m[branch] = d

	return d
}

func buildPRActivity(branchMap map[string]*PRDetail) *PRActivity {
	activity := &PRActivity{}

	if len(branchMap) == 0 {
		return activity
	}

	for _, detail := range branchMap {
		if detail.Action == "" && detail.Number > 0 {
			detail.Action = PRActionUpdated
		}
	}

	for _, detail := range branchMap {
		switch detail.Action {
		case PRActionAutomerged:
			activity.Automerged++
		case PRActionCreated:
			activity.Created++
		case PRActionUpdated:
			activity.Updated++
		case PRActionNeedsApproval:
			activity.NeedsApproval++
		case PRActionUnchanged:
			activity.Unchanged++
		}
	}

	prs := make([]PRDetail, 0, len(branchMap))
	for _, detail := range branchMap {
		prs = append(prs, *detail)
	}

	sort.Slice(prs, func(i, j int) bool {
		oi, oj := actionOrder[prs[i].Action], actionOrder[prs[j].Action]
		if oi != oj {
			return oi < oj
		}

		return prs[i].Branch < prs[j].Branch
	})

	if len(prs) > MaxPRDetails {
		prs = prs[:MaxPRDetails]
		activity.Truncated = true
	}

	activity.PRs = prs

	return activity
}

func formatLogLine(level LogLevel, msg, time string) FormattedLine {
	class := levelClassMap[level]
	if class == "" {
		class = "text-gray-300"
	}

	escaped := html.EscapeString(msg)
	if len(escaped) > MaxFormattedLen {
		escaped = escaped[:MaxFormattedLen] + "…"
	}

	return FormattedLine{
		Level:   level,
		Message: escaped,
		Class:   class,
		Time:    formatTime(time),
	}
}

func formatTime(time string) string {
	if time == "" {
		return ""
	}

	if idx := strings.Index(time, "T"); idx != -1 && len(time) > idx+8 {
		return time[idx+1 : idx+9]
	}

	return time
}

func formatRawLine(line string) FormattedLine {
	escaped := html.EscapeString(line)
	if len(escaped) > MaxFormattedLen {
		escaped = escaped[:MaxFormattedLen] + "…"
	}

	return FormattedLine{
		Level:   0,
		Message: escaped,
		Class:   "text-gray-500",
	}
}

func (f FormattedLine) LevelLabel() string {
	return LevelLabel(f.Level)
}

func LevelLabel(level LogLevel) string {
	if l, ok := levelLabelMap[level]; ok {
		return l
	}

	return "LOG"
}

func isNDJSON(logs string) bool {
	scanner := bufio.NewScanner(strings.NewReader(logs))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if len(line) > 0 && line[0] == '{' {
			var test json.RawMessage

			return json.Unmarshal([]byte(line), &test) == nil
		}

		return false
	}

	return false
}
