# today

```
today - A CLI tool that bootstraps a simple journalling markdown file and opens
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
```

