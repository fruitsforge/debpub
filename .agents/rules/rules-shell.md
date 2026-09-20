# Shell Scripting & Compatibility Rules

These rules govern all bash and POSIX-compatible shell scripts, ensuring maximum safety and portability between macOS (BSD-based userland) and various Linux distributions (GNU-based userland).

## 1. Safety-First Flags
Every production script must begin with strict shell settings to prevent executing actions on failure, undefined variables, or hidden pipe failures:
```bash
#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'
```

## 2. macOS and Linux Portability
Never assume GNU options are available on macOS, or that BSD options exist on Linux.
- **Sed In-Place Editing**: Avoid `sed -i`. Instead, use a temporary file or handle the suffix conditional:
  ```bash
  # Portable approach using temporary file
  tmp_file=$(mktemp)
  sed 's/foo/bar/g' file.txt > "$tmp_file" && mv "$tmp_file" file.txt
  ```
- **Stat / Date Commands**: BSD `stat` and `date` formats differ completely from GNU versions. Use standard Python/Node or avoid using platform-specific formats inside raw shell.
- **Xargs**: Always use `xargs -I {}` or standard `find -exec` for portability instead of GNU-specific extensions.

## 3. Strict Code Practices
- **Variables**: Always double-quote variables when referencing them to prevent word splitting and globbing:
  ```bash
  # Recommended
  filepath="/path/to/some file.txt"
  cat "$filepath"
  ```
- **Error Trapping / Cleanup**: Always clean up temporary directories and resources using `trap`:
  ```bash
  temp_dir=$(mktemp -d)
  cleanup() {
      rm -rf "$temp_dir"
  }
  trap cleanup EXIT
  ```
- **ShellCheck Compliance**: All generated scripts must run cleanly through ShellCheck. Avoid obsolete backticks `` `command` ``; always use `$(command)` syntax.
- **Path Checking**: Never run relative directory changing (`cd`) without validating the target directory exists and succeeds.
  ```bash
  cd "/target/path" || exit 1
  ```

## 4. standard Command-Line Utilities & Pipelines
Antigravity must utilize native local CLI tools instead of complex custom scripts when parsing, searching, or transforming project files. Follow these strict tool standards:

- **Safe Piping with Spaces (`find` & `xargs`)**:
  - Always use `-print0` with `find` and `-0` with `xargs` to ensure filenames containing whitespace, tabs, or quotes do not break execution or pose security injection vectors:
    ```bash
    find . -type f -name "*.txt" -print0 | xargs -0 grep "search_term"
    ```
- **Column & Text Processing (`awk` & `sed`)**:
  - Use `awk` for columnar parsing. Always use `-F` for custom delimiters (e.g. processing CSV columns):
    ```bash
    # Print the first column of comma-separated list
    awk -F, '{print $1}' data.csv
    ```
  - For `sed` transformations, avoid in-place `-i` changes where BSD (macOS) and GNU (Linux) formats collide. Instead, pipe output to a temporary file and replace safely.
- **JSON Processing (`jq`)**:
  - Do not use regex or grep to parse JSON. Always use `jq`.
  - Use the raw output `-r` flag when extracting JSON string values to pipe to other terminal commands:
    ```bash
    cat config.json | jq -r '.database.url'
    ```
- **Grouping and Counting (`sort` & `uniq`)**:
  - When aggregating occurrences (e.g., parsing logs), always `sort` inputs before piping to `uniq` (as `uniq` only merges adjacent identical lines):
    ```bash
    grep "ERROR" app.log | sort | uniq -c
    ```
- **Process & Port Checking (`ps` & `lsof`)**:
  - Use `ps aux | grep <process>` safely by excluding the grep command itself (`grep [p]rocess` or checking status codes).
  - Use `lsof -i :<port>` to inspect active local port usage rather than relying on raw guesses.

