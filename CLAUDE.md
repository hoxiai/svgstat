# Superpowers Engineering Rules & Disciplines

You must adhere to the engineering disciplines and workflows defined in `~/.config/superpowers`.

## Core Principles

1. **Plan Before Code**: Always write implementation plans before coding, obtain review/alignment, and break tasks into concrete atomic steps.
2. **Test-Driven Development (TDD)**: Where tests exist or can be created, follow strict TDD:
   - **RED**: Write a failing test first that demonstrates the desired behavior. Verify it fails.
   - **GREEN**: Write minimal production code to make the test pass. Verify it passes.
   - **REFACTOR**: Clean up code and eliminate duplication while remaining green.
   - **The Iron Law**: No production code without a failing test first.
3. **Isolate Critical Changes**: Keep diffs minimal, focused, and scoped to the task. Avoid unnecessary refactors outside the target area.
4. **Verification Before Completion**: Run automated test suites and verify edge cases before claiming any task is done.
5. **Zero Slop**: Respect architectural boundaries (e.g. low latency rendering, single binary simplicity, zero unnecessary dependencies).
