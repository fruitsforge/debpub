# Global Developer Rules
# Upstream Update 2026.1
# Upstream Update 2026.2

This document establishes the overarching developer expectations, quality standards, and communication behaviors that all AI coding agents must follow across all projects.

## Core Directives

1. **Do No Harm / Documentation Integrity**:
   - Maintain existing docstrings, annotations, inline comments, and type definitions unless explicitly requested otherwise or immediately affected by a refactoring change.
   - Do not replace descriptive variable names or simplify algorithms without solid justification.

2. **Strictness & Precision**:
   - Write highly explicit, type-safe, and self-documenting code. Avoid shortcuts like arbitrary casting (e.g., `any` in TypeScript or raw type conversions without safe checks in Kotlin/PHP).
   - All errors must be handled explicitly. Never write empty catch/rescue blocks or silence logs.

3. **Modern Features Standard**:
   - Always verify the project's dependency versions (e.g., in `package.json`, `build.gradle.kts`, `composer.json`) before generating code.
   - Utilize the latest safe conventions of the target language versions (e.g., Kotlin 2.2+, PHP 8.x, Angular 19/20/21) rather than legacy patterns.

4. **Deprecations & Versioning Standard**:
   - Do NOT use deprecated APIs, libraries, modules, functions, methods, or syntax in any programming language or framework.
   - If a deprecated element is encountered in the existing codebase or suggested by search, actively replace it or propose upgrading to the latest stable API/alternative.
   - Stick to the latest version of libraries and frameworks where possible, avoiding legacy or outdated paradigms.

5. **Automated Verification First**:
   - Run compilation, linting, and testing suites after modifying any core codebase. Do not assume code works without executing validations.

## Communication & Git Standards

- **Git Commit Message Rules**:
  - Use conventional commits format (`feat:`, `fix:`, `refactor:`, `chore:`, `docs:`, `test:`).
  - Write concise, lowercase titles under 50 characters, followed by a blank line and a bulleted description detailing the *why* of the changes.
- **PR Summaries**:
  - Summarize changes factually and without self-praise.
  - Detail modified files, new features, and execution verification steps.
