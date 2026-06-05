# Live filter — eight rows on screen, narrow to one with "8.8", lock with
# enter, clear with esc, see all rows return, quit.
# Run from the repo root: `vhs docs/recordings/filter.tape`

Output docs/filter.gif

Set Shell "bash"
Set FontSize 14
Set Width 1400
Set Height 500
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms

Type "./pingtop 192.0.2.0/29 1.1.1.1 8.8.8.8"
Sleep 2s
Enter
Sleep 4s

# Open the filter, type a substring
Type "/"
Sleep 500ms
Type "8.8"
Sleep 2s

# Lock the filter (exit edit mode, filter stays)
Enter
Sleep 2s

# Clear the filter
Escape
Sleep 2s

Type "q"
Sleep 500ms
