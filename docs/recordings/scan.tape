# CIDR scan with -d — TEST-NET-1 is RFC 5737 reserved and unreachable
# from any network, so all six expanded hosts will hit -d 5 and disappear.
# Run from the repo root: `vhs docs/recordings/scan.tape`

Output docs/scan.gif

Set Shell "bash"
Set FontSize 14
Set Width 1100
Set Height 500
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms

Type "./pingtop -d 5 192.0.2.0/29"
Sleep 2s
Enter
Sleep 8s
Type "q"
Sleep 500ms
