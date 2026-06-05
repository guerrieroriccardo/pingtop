# Hero demo — main banner GIF. Wide window, mix of hosts including a
# hostname, longer duration so the sparkline has time to fill.
# Run from the repo root: `vhs docs/recordings/hero.tape`

Output docs/hero.gif

Set Shell "bash"
Set FontSize 14
Set Width 1400
Set Height 700
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms

Type "./pingtop 1.1.1.1 8.8.8.8 9.9.9.9 example.com cloudflare.com"
Sleep 2s
Enter
Sleep 15s
Type "q"
Sleep 500ms
