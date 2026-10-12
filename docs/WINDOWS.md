# OpenDiablo2 on Windows (experimental, never run on Windows yet)

Status, stated plainly: the Windows `.exe` is **cross-compiled from a Mac and has never been started on a Windows
machine**. What was checked: `go vet` for `GOOS=windows GOARCH=amd64` is clean, the build succeeds, and `file` reports a
PE32+ x86-64 executable. What was not checked: everything at run time (window, input, audio, the first-run dialogs, saves).
Expect rough edges and please report them with the log text.

You need your **own** copy of Diablo II and Lord of Destruction (1.14b files). No game files are in this repository or in
the zip, and none are distributed.

## Run it

1. Unzip `OpenDiablo2-<version>-windows-amd64.zip` anywhere (for example `C:\Games\OpenDiablo2`). It holds only
   `OpenDiablo2.exe`, this README and the licence.
2. Double-click `OpenDiablo2.exe`. SmartScreen may warn about an unsigned program: More info, Run anyway.
3. On the first run the game looks for your Diablo II folder (`C:\Program Files (x86)\Diablo II`, `C:\Program Files\Diablo II`,
   Steam / Battle.net / `Games` folders on C: to F:). If it finds one it asks to use it; otherwise a folder picker (PowerShell
   dialog) opens. Pick the folder that contains `d2data.mpq`, `d2char.mpq`, `d2exp.mpq`, `patch_d2.mpq` and the other `.mpq`
   files. The files are only read.
4. Your real `.d2s` characters are imported (originals never changed) from `%USERPROFILE%\Saved Games\Diablo II*`, the
   `Save` folder inside the game folder and a few other known places.

## config.json

The configuration is read from, and written to, `%APPDATA%\OpenDiablo2\config.json` (that is
`C:\Users\<you>\AppData\Roaming\OpenDiablo2\config.json`; Go's `os.UserConfigDir`). Saves go in the `Saves` folder next to it.
Set the environment variable `OD2_CONFIG_DIR` to use another folder. If that file does not exist the game falls back to a
`config.json` next to the exe.

Minimal settings (JSON accepts forward slashes, which avoids escaping backslashes):

```json
{
  "MpqPath": "C:/Program Files (x86)/Diablo II",
  "MpqLoadOrder": [
    "patch_d2.mpq", "d2exp.mpq", "d2xmusic.mpq", "d2xtalk.mpq", "d2xvideo.mpq",
    "d2data.mpq", "d2char.mpq", "d2music.mpq", "d2sfx.mpq", "d2video.mpq", "d2speech.mpq"
  ]
}
```

Write backslashes as `\\` if you use them. Set `OD2_NO_SETUP=1` to skip the first-run dialogs.

## Paths and case

- The Windows file system is case-insensitive, so `D2DATA.MPQ` and `d2data.mpq` both open. Names inside the MPQ archives are
  lowercased and use backslashes internally; the loader already converts `/` to `\` and the MPQ hash is case-insensitive,
  so nothing platform specific is needed. The case-insensitive open helper is used on Linux only.
- All game-side paths are built with `filepath.Join`, so they get backslashes on Windows. The macOS-only locations in
  first-run discovery are harmless no-ops on Windows.
- Process CPU time (used only by the `OD2_PERF` self-profiler) uses `GetProcessTimes` on Windows.

## Build it yourself (from a Mac or Linux machine)

```
scripts/make-windows.sh            # dist/windows-amd64/OpenDiablo2.exe (console subsystem, log lines visible)
ZIP=1 scripts/make-windows.sh      # also dist/OpenDiablo2-<version>-windows-amd64.zip
WINGUI=1 scripts/make-windows.sh   # no console window (but then log output is invisible)
```

No C compiler is needed (`CGO_ENABLED=0`): the pinned Ebiten v2.0.2 loads an embedded `glfw.dll` and uses `winmm` for audio.
`zig` (installed at `/opt/homebrew/bin/zig` here) or mingw is only needed if Ebiten is upgraded to a version that requires
cgo; then use `CGO_ENABLED=1 CC="zig cc -target x86_64-windows-gnu"`. Nothing is installed by the script.
To check by hand: `GOOS=windows GOARCH=amd64 go vet ./... && file dist/windows-amd64/OpenDiablo2.exe`.

## Windows on ARM

`GOARCH=arm64` does **not** build: Ebiten v2.0.2 embeds `glfw.dll` for amd64 and 386 only
(`internal/glfw/load_windows.go: undefined: glfwDLLCompressed`), and the pinned `golang.org/x/sys` is also too old for
windows/arm64 (bumping it to v0.1.0 fixes that part only). Windows 11 on ARM runs the amd64 `.exe` under emulation, which is
the supported route until Ebiten is upgraded (a project-wide change, not done here).

## Known untested or unsupported on Windows

- First-run dialogs (`d2app/d2setup/ui_native_windows.go`) use PowerShell with `System.Windows.Forms`; the Yes/No box maps
  the two buttons to the "use this / choose another" choices and says so in the text.
- The macOS app supervisor, crash dialog and `.app` logging do not exist here; run from a console to see the log.
- No Wine test was done.
