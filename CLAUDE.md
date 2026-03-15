# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

FabDoYouMeme is a GPLv3-licensed open-source project hosted at github.com/MorganKryze/FabDoYouMeme. It is in early development — no build system, source code, or tooling has been established yet. Update this file as the stack, build commands, and architecture are established.

## Workflow Orchestration

### Planning

- Enter plan mode for any non-trivial task (3+ steps or architectural decisions)
- If something goes sideways, stop and re-plan — don't keep pushing
- Use plan mode for verification steps, not just building
- Write detailed specs upfront to reduce ambiguity

### Subagent Strategy

- Use subagents liberally to keep the main context window clean
- Offload research, exploration, and parallel analysis to subagents
- For complex problems, use more subagents for parallel compute
- One focused task per subagent

### Self-Improvement Loop

- After any correction from the user: update `tasks/lessons.md` with the pattern
- Write rules that prevent the same mistake from recurring
- Review relevant lessons at session start

### Verification Before Done

- Never mark a task complete without proving it works
- Diff behavior between main and your changes when relevant
- Run tests, check logs, demonstrate correctness

### Elegance (Balanced)

- For non-trivial changes: pause and ask "is there a more elegant way?"
- If a fix feels hacky: implement the elegant solution instead
- Skip this for simple, obvious fixes — don't over-engineer

### Autonomous Bug Fixing

- When given a bug report: fix it without hand-holding
- Point at logs, errors, failing tests — then resolve them
- Fix failing CI tests without being told how

## Task Management

1. **Plan First**: Write plan to `tasks/todo.md` with checkable items
2. **Verify Plan**: Check in before starting implementation
3. **Track Progress**: Mark items complete as you go
4. **Explain Changes**: High-level summary at each step
5. **Document Results**: Add review section to `tasks/todo.md`
6. **Capture Lessons**: Update `tasks/lessons.md` after corrections

## Core Principles

- **Simplicity First**: Make every change as simple as possible; impact minimal code
- **No Laziness**: Find root causes; no temporary fixes; senior developer standards
- **Minimal Impact**: Changes should only touch what's necessary
