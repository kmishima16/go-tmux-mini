# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Go-Tmux-Mini is a learning project that implements core tmux functionality (pane splitting and management) using Go. This is a 3-hour coding challenge to understand TUI application architecture through hands-on implementation.

**Core Technologies:**
- `github.com/gdamore/tcell/v2` - Terminal User Interface library
- `github.com/creack/pty` - Pseudo-terminal (PTY) management 
- `golang.org/x/term` - Terminal control utilities

## Commands

**Build and Run:**
```bash
go run .                    # Start the application
go build -o tmux-mini .     # Build binary
```

**Development:**
```bash
tail -f debug.log          # Monitor debug output (application logs to debug.log)
go mod tidy                 # Clean up dependencies
```

**Testing Key Functions:**
- `Ctrl+C` - Exit application
- `Ctrl+B` followed by `%` - Split pane horizontally (left/right)
- `Ctrl+B` followed by `"` - Split pane vertically (up/down) 
- `Ctrl+B` followed by arrow keys - Navigate between panes

## Architecture

**Single-File Architecture:** All functionality is contained in `main.go` (~370 lines) for simplicity.

**Core Components:**

1. **Pane Management:**
   - `Pane` struct: Manages individual terminal sessions with position, size, PTY, and output buffer
   - Each pane runs an independent `bash` process via PTY
   - Thread-safe output buffering with mutex protection

2. **TUI Layer (tcell):**
   - Screen initialization with raw mode for direct key capture
   - Custom drawing routines for pane borders (ASCII: `+`, `-`, `|`)
   - Active pane highlighted with yellow borders, inactive with white

3. **Event Loop:**
   - Prefix mode system (`Ctrl+B` activates command mode)
   - Key routing: normal keys → active pane, prefix commands → pane management
   - Dynamic pane splitting with automatic size calculation

4. **PTY Integration:**
   - Each pane spawns `exec.Command("bash")` in pseudo-terminal
   - Asynchronous output reading via goroutines
   - Input forwarding from active pane to corresponding PTY

**Key Data Flow:**
```
User Input → tcell Events → Prefix Mode Check → Active Pane PTY → bash Process
                                             ↓
Debug Log ← Pane Buffer ← PTY Output ← bash Output
```

## Current Implementation Status

**Completed (Phase 1-3):**
- ✅ Basic PTY + tcell integration
- ✅ Pane data structures and management
- ✅ Drawing system with border visualization
- ✅ Prefix key system (Ctrl+B)
- ✅ Pane splitting logic (horizontal/vertical)
- ✅ Focus management and navigation

**Known Issues:**
- Pane borders display correctly but input/output functionality may have bugs
- PTY output rendering needs refinement
- Split pane focus switching requires debugging

**Development Notes:**
- Debug logging is extensive - check `debug.log` for detailed execution traces
- WSL compatibility has been tested and works with tcell
- The application uses raw terminal mode, so normal terminal shortcuts are captured
- Border drawing uses simple ASCII characters for maximum compatibility

## Development Workflow

When debugging or extending functionality:

1. **Check debug.log first** - All pane operations, screen draws, and key events are logged
2. **Test basic rendering** - The app displays "TEST OK" and "KEY: C-B" in the top-right as visual confirmation
3. **Verify PTY creation** - Each pane should spawn its own bash process
4. **Test incrementally** - Start with single pane, then test splitting, then navigation

The codebase prioritizes learning and clarity over production robustness - it's designed to demonstrate TUI/PTY concepts in a compact, readable format.