# Keep-dropped variant — same /29 as the scan demo but rows persist with
# 100% loss / 5/5 SENT/LOST so the discovery result stays on screen.
# Run from the repo root: `vhs docs/recordings/keep-dropped.tape`

Output docs/keep-dropped.gif

Set Shell "bash"
Set FontSize 14
Set Width 1400
Set Height 500
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms

Type "./pingtop -k -d 5 192.0.2.0/29"
Sleep 2s
Enter
Sleep 8s
Type "q"
Sleep 500ms
