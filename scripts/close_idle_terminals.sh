#!/bin/zsh
# Close every Terminal window that is not busy. Windows whose name contains D2MOO or claude (the owner's own
# session, "D2MOO installation and setup") are NEVER closed. Safe to run any time.
/usr/bin/osascript <<'OSA'
tell application "System Events" to if not (exists process "Terminal") then return
tell application "Terminal"
	set n to count of windows
	repeat with i from n to 1 by -1
		try
			set w to window i
			set nm to name of w
			if nm does not contain "D2MOO" and nm does not contain "claude" and not (busy of w) then close w saving no
		end try
	end repeat
end tell
OSA
