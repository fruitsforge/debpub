---
name: shell-developer
description: Guides the agent to write robust, cross-platform bash and POSIX shell scripts, ensuring safety flags, correct quoting, and clean macOS/Linux userland interoperability.
---

# Shell Developer Skill

Optimizes and validates bash and shell scripts for safe execution across diverse execution environments (macOS and various Linux distributions).

## Guidelines & Best Practices

1. **Mandatory Configuration**:
   - Always enforce strict shell options at the top of scripts:
     ```bash
     set -euo pipefail
     ```

2. **Interoperability Management**:
   - Do not use GNU-specific command parameters (like `sed -i` without temp-files, or specialized `date`/`stat` options) if target script execution environments span macOS and Linux. Use standard POSIX structures or scripting language wrappers (Python/Node).

3. **Validation & Quoting**:
   - Double-quote all shell variable expansions to prevent unexpected word splitting.
   - Clean up temporary files on exit using a clean `trap` callback structure.
   - Ensure the generated code compiles cleanly under ShellCheck criteria.

4. **Standard Composable CLI Pipelines**:
   - Instruct the agent to build composable Unix pipelines rather than custom Python or Node scripts for data/text extraction.
   - Safely chain files: utilize `find -print0 | xargs -0` for space-safe execution.
   - Direct the agent to use `jq` for JSON handling, `awk` for columnar delimiters (`-F`), and `sort | uniq -c` for counting.

