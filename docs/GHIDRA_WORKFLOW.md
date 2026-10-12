# Reading the original binary: the Ghidra and exe-notes workflow

Behaviour in this engine is ported from the original 1.14b `Game.exe`, which is read-only in
[Ghidra](https://ghidra-sre.org/). This page is the contributor's short path. The longer method, tooling and naming rules
are in [REVERSE_ENGINEERING.md](REVERSE_ENGINEERING.md).

## What may and may not be written down

You supply your own copy of the game; the project supplies none of it.

| Allowed in the repo, notes and comments | Never |
|---|---|
| addresses, function names you chose, structure offsets | pasted decompiler output or disassembly bodies |
| small constants and tables needed for interoperability (a multiplier, a field offset, a handful of small numbers) | a function transcribed line by line |
| control flow described in your own words | game files: MPQs, extracted tables or strings, art, audio, video, executables, saves |
| test numbers derived from our own generator | screenshots of game data, CD keys |

If in doubt, write the idea in words and cite the address.

## The loop

1. **Pick a question** with a visible effect ("what does the engine do when X happens?"). Check
   [KNOWN_GAPS.md](KNOWN_GAPS.md), [HELP_WANTED.md](HELP_WANTED.md) and the notes index in
   [REVERSE_ENGINEERING.md](REVERSE_ENGINEERING.md#7-notes-index) first; someone may have answered it.
2. **Open your own copy of `Game.exe` (1.14b)** in Ghidra (x86 32-bit, image base `0x400000`). Auto-analysis is enough to start.
3. **Find an anchor.** The original leaves its source file names in assert strings (for example `UI\npcmenu.cpp`). Search
   strings, take the cross references, and you land inside the function that owns the behaviour.
4. **Read callers and callees**, rename locally only when you understand the function. Names are
   `MODULE_VerbFirstPascalCase` (`UI_BuildNpcMenu`, `TRADE_CalcItemPrice`). A guess keeps a hedged name and is listed as
   unverified.
5. **Write a note in your own words**: address, what it reads and writes, the formula or table in prose, edge cases, how the
   Go port should differ (if at all). Tag every claim with an evidence level: `VERIFIED`, `inferred` or `UNVERIFIED`
   ([REVERSE_ENGINEERING.md](REVERSE_ENGINEERING.md#4-evidence-levels)). Never upgrade a tag without new evidence.
6. **Port with a citation.** In the Go code put the Ghidra name and address in a comment
   (`// SKILL_ServerRunStartFunc (0x56d4e0), VERIFIED`) and keep the rule in a small pure package where possible.
7. **Test against the engine.** A unit test with a case that could fail, then a verify scenario if the behaviour is visible
   in play ([ADDING_A_SCENARIO.md](ADDING_A_SCENARIO.md)). When the binary and an earlier note disagree, the binary wins;
   record it as `NOTE (binary)`.

## Where the notes live

Notes are plain markdown in the contributor's own notes folder (the author's is `~/git/d2-re-notes/`, kept outside this
repo). Name a note by topic (`skills-2.md`). If you send a note to the project, put it in a pull request as text under
`docs/` only after checking it against the table above. Behaviour notes that the engine depends on are summarised in
[STATUS_MATRIX.md](STATUS_MATRIX.md) with the test or scenario that is the evidence.

## Using the Ghidra MCP server (optional)

The project drives Ghidra from an AI agent through an MCP server plugin listening on `127.0.0.1:8089`. Policy: queries are
modest, one at a time, read-only unless you own the naming task, no scripts, never close the window. Details and the
fallback over plain HTTP: [REVERSE_ENGINEERING.md](REVERSE_ENGINEERING.md#11a-the-ghidra-workflow-step-by-step). You do not
need MCP to contribute; the Ghidra GUI is enough.

## Before you push

Run `scripts/check_repo_hygiene.sh`, check `git status` for stray game files, and make sure no comment or note contains
decompiler output. See [CONTRIBUTING.md](../CONTRIBUTING.md).
