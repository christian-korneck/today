package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

// Default editor – override via TODAY_EDITOR env var or $EDITOR.
const defaultEditor = ""

var weekdayShort = map[time.Weekday]string{
	time.Monday:    "mo",
	time.Tuesday:   "tu",
	time.Wednesday: "we",
	time.Thursday:  "th",
	time.Friday:    "fr",
	time.Saturday:  "sa",
	time.Sunday:    "su",
}

var monthName = map[time.Month]string{
	time.January:   "January",
	time.February:  "February",
	time.March:     "March",
	time.April:     "April",
	time.May:       "May",
	time.June:      "June",
	time.July:      "July",
	time.August:    "August",
	time.September: "September",
	time.October:   "October",
	time.November:  "November",
	time.December:  "December",
}

func main() {
	flagToday := false
	flagNoRollover := false
	flagVSCode := false
	flagEditor := ""
	flagDir := ""
	flagCreateOnly := false

	flag.BoolVar(&flagToday, "t", false, "")
	flag.BoolVar(&flagToday, "today", false, "")
	flag.BoolVar(&flagNoRollover, "n", false, "")
	flag.BoolVar(&flagNoRollover, "no-rollover", false, "")
	flag.BoolVar(&flagVSCode, "v", false, "")
	flag.BoolVar(&flagVSCode, "vscode", false, "")
	flag.StringVar(&flagEditor, "e", "", "")
	flag.StringVar(&flagEditor, "editor", "", "")
	flag.BoolVar(&flagCreateOnly, "c", false, "")
	flag.BoolVar(&flagCreateOnly, "create-only", false, "")
	flag.StringVar(&flagDir, "dir", "", "")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `today - A CLI tool that bootstraps a simple journalling markdown file and opens
it in the default editor. Unfinished ToDo items by default roll over from the
last active day.

Usage: today [flags]

Flags:
  -t, --today         Force today's date even before 6 AM (by default, before
                      6 AM the previous day's file is used)
  -n, --no-rollover   Start with sample todos instead of rolling over from the
                      previous day's file
  -c, --create-only   Create the file if needed but do not open an editor
  -e, --editor CMD    Open the file with CMD instead of $TODAY_EDITOR / $EDITOR
  -v, --vscode        Open the file in VS Code (shorthand for -e code)
      --dir PATH      Notes directory (default: ~/today, or $TODAY_NOTESDIR)

Environment:
  TODAY_EDITOR    Editor command (overrides $EDITOR)
  TODAY_NOTESDIR  Notes directory (overrides the ~/today default)

Examples:
  today                     Create/open today's journal
  today -v                  Create/open in VS Code
  today -c                  Create without opening an editor
  today -n -e nano          Fresh todos, open in nano
  today --dir ~/notes       Use a custom notes directory
`)
	}
	flag.Parse()

	// Resolve base directory: -dir flag > TODAY_NOTESDIR env > ~/today
	baseDir := flagDir
	if baseDir == "" {
		baseDir = os.Getenv("TODAY_NOTESDIR")
	}
	if baseDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fatal("cannot determine home directory: %v", err)
		}
		baseDir = filepath.Join(home, "today")
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		fatal("cannot create directory %s: %v", baseDir, err)
	}

	// Effective date: before 6 AM local time counts as previous day.
	effective := effectiveDate(time.Now(), flagToday)

	// Build filename.
	u, err := user.Current()
	if err != nil {
		fatal("cannot determine user: %v", err)
	}
	username := sanitizeUsername(u.Username)
	filename := buildFilename(effective, username)
	filePath := filepath.Join(baseDir, filename)

	// Only create if it does not already exist.
	if _, err := os.Stat(filePath); err != nil {
		// Resolve todos.
		todos := sampleTodos()
		if !flagNoRollover {
			if rolled := rolloverTodos(baseDir, effective); rolled != "" {
				todos = rolled
			}
		}

		content := buildFileContent(effective, todos)

		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			fatal("cannot write file %s: %v", filePath, err)
		}
	}

	fmt.Println(filePath)

	if !flagCreateOnly {
		openEditor(filePath, flagEditor, flagVSCode)
	}
}

func effectiveDate(now time.Time, forceToday bool) time.Time {
	if now.Hour() < 6 && !forceToday {
		return now.AddDate(0, 0, -1)
	}
	return now
}

func sanitizeUsername(raw string) string {
	if i := strings.LastIndexByte(raw, '\\'); i >= 0 {
		return raw[i+1:]
	}
	return raw
}

func buildFilename(effective time.Time, username string) string {
	_, isoWeek := effective.ISOWeek()
	wd := weekdayShort[effective.Weekday()]
	return fmt.Sprintf("%s-%s_cw%d_%s.md",
		effective.Format("2006-01-02"), wd, isoWeek, username)
}

func buildHeader(effective time.Time) string {
	_, isoWeek := effective.ISOWeek()
	wd := weekdayShort[effective.Weekday()]
	wdTitle := strings.ToUpper(wd[:1]) + wd[1:]
	return fmt.Sprintf("# %s, %d %s %d - CW %d\n",
		wdTitle, effective.Day(), monthName[effective.Month()], effective.Year(), isoWeek)
}

func buildFileContent(effective time.Time, todos string) string {
	var buf strings.Builder
	buf.WriteString(buildHeader(effective))
	buf.WriteString("\n## ToDo (Rollover)\n\n")
	buf.WriteString(todos)
	buf.WriteString("\n### Notes\n\n")
	return buf.String()
}

func sampleTodos() string {
	return "- [x] First ToDo\n- [ ] Second ToDo\n"
}

// rolloverTodos finds the most recent previous notes file and extracts
// unchecked top-level todo items from the "## ToDo (Rollover)" section.
func rolloverTodos(baseDir string, effective time.Time) string {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return ""
	}

	effectivePrefix := effective.Format("2006-01-02")
	var candidates []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".md") || len(name) < 10 {
			continue
		}
		if name[:10] < effectivePrefix {
			candidates = append(candidates, name)
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.Strings(candidates)

	content, err := os.ReadFile(filepath.Join(baseDir, candidates[len(candidates)-1]))
	if err != nil {
		return ""
	}
	return parseTodosFromContent(string(content))
}

// parseTodosFromContent extracts unchecked top-level todo items (with sub-items)
// from the "## ToDo (Rollover)" section of a markdown file.
func parseTodosFromContent(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	inSection := false
	var sectionLines []string

	for scanner.Scan() {
		line := scanner.Text()
		if !inSection {
			if isTodoHeader(line) {
				inSection = true
			}
			continue
		}
		if strings.HasPrefix(line, "#") {
			break
		}
		sectionLines = append(sectionLines, line)
	}

	if len(sectionLines) == 0 {
		return ""
	}

	topLevelRe := regexp.MustCompile(`^- \[[ xX]\] `)

	type block struct {
		done  bool
		lines []string
	}
	var blocks []block
	var cur *block

	for _, line := range sectionLines {
		if topLevelRe.MatchString(line) {
			done := strings.HasPrefix(line, "- [x] ") || strings.HasPrefix(line, "- [X] ")
			b := block{done: done, lines: []string{line}}
			blocks = append(blocks, b)
			cur = &blocks[len(blocks)-1]
		} else if cur != nil {
			cur.lines = append(cur.lines, line)
		}
	}

	var out []string
	for _, b := range blocks {
		if !b.done {
			out = append(out, strings.Join(b.lines, "\n"))
		}
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\n") + "\n"
}

// isTodoHeader matches "## ToDo (Rollover)" (case-insensitive for "todo").
func isTodoHeader(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	return lower == "## todo (rollover)"
}

func openEditor(filePath string, flagEditor string, vscode bool) {
	var editor string
	switch {
	case vscode:
		editor = "code"
	case flagEditor != "":
		editor = flagEditor
	case os.Getenv("TODAY_EDITOR") != "":
		editor = os.Getenv("TODAY_EDITOR")
	case defaultEditor != "":
		editor = defaultEditor
	case os.Getenv("EDITOR") != "":
		editor = os.Getenv("EDITOR")
	case runtime.GOOS == "windows":
		editor = "notepad.exe"
	default:
		fmt.Fprintf(os.Stderr, "No editor configured (set TODAY_EDITOR or EDITOR)\n")
		return
	}

	// Try direct exec first (works cross-platform for binaries in PATH).
	cmd := exec.Command(editor, filePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if cmd.Run() == nil {
		return
	}

	// Fall back to platform shell to resolve aliases/functions.
	var sh *exec.Cmd
	if runtime.GOOS == "windows" {
		sh = exec.Command("cmd", "/C", editor, filePath)
	} else {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "sh"
		}
		sh = exec.Command(shell, "-ic", editor+` "$1"`, "--", filePath)
	}
	sh.Stdin = os.Stdin
	sh.Stdout = os.Stdout
	sh.Stderr = os.Stderr
	if err := sh.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "today: editor %q: %v\n", editor, err)
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "today: "+format+"\n", args...)
	os.Exit(1)
}
