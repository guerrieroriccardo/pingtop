# Basic dashboard — three public hosts, shorter window for the Usage section.
# Run from the repo root: `vhs docs/recordings/basic.tape`

Output docs/basic.gif

Set Shell "bash"
Set FontSize 14
Set Width 1100
Set Height 350
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms

Type "./pingtop 1.1.1.1 8.8.8.8 example.com"
Sleep 2s
Enter
Sleep 8s
Type "q"
Sleep 500ms
