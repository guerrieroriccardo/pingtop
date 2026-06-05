# Sort demo — cycle the active sort column with s, flip the direction
# with r, walk backward with S, then back to unsorted.
# Run from the repo root: `vhs docs/recordings/sort.tape`

Output docs/sort.gif

Set Shell "bash"
Set FontSize 14
Set Width 1400
Set Height 500
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms

Type "./pingtop 1.1.1.1 8.8.8.8 9.9.9.9 example.com cloudflare.com"
Sleep 1s
Enter
Sleep 2s

# s cycles forward through the visible sortable columns
Type "s"
Sleep 1000ms
Type "s"
Sleep 1000ms

# r flips the direction (sticky across column changes)
Type "r"
Sleep 1000ms

# Keep walking forward
Type "s"
Sleep 1000ms
Type "s"
Sleep 1000ms

# S walks backward
Type "S"
Sleep 1000ms
Type "S"
Sleep 1000ms

Type "q"
Sleep 500ms
