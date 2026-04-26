package main

import (
	"testing"
	"time"
)

func date(year int, month time.Month, day, hour int) time.Time {
	return time.Date(year, month, day, hour, 30, 0, 0, time.UTC)
}

func TestEffectiveDate(t *testing.T) {
	tests := []struct {
		name       string
		now        time.Time
		forceToday bool
		wantDay    int
	}{
		{"3am rolls back", date(2026, 4, 26, 3), false, 25},
		{"5:59am rolls back", date(2026, 4, 26, 5), false, 25},
		{"6am stays today", date(2026, 4, 26, 6), false, 26},
		{"7am stays today", date(2026, 4, 26, 7), false, 26},
		{"3am with forceToday", date(2026, 4, 26, 3), true, 26},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := effectiveDate(tt.now, tt.forceToday)
			if got.Day() != tt.wantDay {
				t.Errorf("effectiveDate(%v, %v).Day() = %d, want %d",
					tt.now, tt.forceToday, got.Day(), tt.wantDay)
			}
		})
	}
}

func TestSanitizeUsername(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"john", "john"},
		{"DOMAIN\\john", "john"},
		{"CORP\\DESKTOP-1\\john", "john"},
		{"\\leading", "leading"},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			if got := sanitizeUsername(tt.raw); got != tt.want {
				t.Errorf("sanitizeUsername(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestBuildFilename(t *testing.T) {
	// Sunday, 26 April 2026, ISO week 17
	d := date(2026, 4, 26, 10)
	got := buildFilename(d, "alice")
	want := "2026-04-26-su_cw17_alice.md"
	if got != want {
		t.Errorf("buildFilename = %q, want %q", got, want)
	}

	// Monday, 5 January 2026, ISO week 2
	d2 := date(2026, 1, 5, 10)
	got2 := buildFilename(d2, "bob")
	want2 := "2026-01-05-mo_cw2_bob.md"
	if got2 != want2 {
		t.Errorf("buildFilename = %q, want %q", got2, want2)
	}
}

func TestBuildHeader(t *testing.T) {
	d := date(2026, 4, 26, 10)
	got := buildHeader(d)
	want := "# Su, 26 April 2026 - CW 17\n"
	if got != want {
		t.Errorf("buildHeader = %q, want %q", got, want)
	}
}

func TestParseTodosFromContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "mixed done and undone",
			content: `# Header

## ToDo (Rollover)

- [x] Done task
- [ ] Open task
- [x] Another done

### Notes
`,
			want: "- [ ] Open task\n",
		},
		{
			name: "uppercase X counts as done",
			content: `## ToDo (Rollover)

- [X] Done with uppercase
- [ ] Still open
`,
			want: "- [ ] Still open\n",
		},
		{
			name: "all done returns empty",
			content: `## ToDo (Rollover)

- [x] Done one
- [x] Done two
`,
			want: "",
		},
		{
			name: "no section returns empty",
			content: `# Header

### Notes
`,
			want: "",
		},
		{
			name:    "empty content",
			content: "",
			want:    "",
		},
		{
			name: "case insensitive header",
			content: `## TODO (Rollover)

- [ ] Found it
`,
			want: "- [ ] Found it\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTodosFromContent(tt.content)
			if got != tt.want {
				t.Errorf("parseTodosFromContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseTodosPreservesSubitems(t *testing.T) {
	content := `## ToDo (Rollover)

- [x] Done parent
  - [ ] Sub-item of done (dropped with parent)
  - [x] Another sub
- [ ] Open parent
  - [x] Done sub-item (kept because parent is open)
  - [ ] Open sub-item
    - [ ] Nested deeper
- [x] Another done
`
	got := parseTodosFromContent(content)
	want := `- [ ] Open parent
  - [x] Done sub-item (kept because parent is open)
  - [ ] Open sub-item
    - [ ] Nested deeper
`
	if got != want {
		t.Errorf("sub-item preservation failed.\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestIsTodoHeader(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"## ToDo (Rollover)", true},
		{"## TODO (Rollover)", true},
		{"## todo (rollover)", true},
		{"## ToDo (Rollover)  ", true},
		{"# ToDo (Rollover)", false},
		{"### ToDo (Rollover)", false},
		{"## ToDo", false},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			if got := isTodoHeader(tt.line); got != tt.want {
				t.Errorf("isTodoHeader(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}
