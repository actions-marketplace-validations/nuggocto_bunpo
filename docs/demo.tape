# Regenerate docs/demo.gif from the repository root:
#   go build -o bin/bunpo ./cmd/bunpo
#   PATH="$PWD/bin:$PATH" vhs docs/demo.tape

Output docs/demo.gif

Require bunpo
Require fish

Set Shell "fish"
Set FontFamily "JetBrainsMono Nerd Font Mono"
Set FontSize 15
Set LineHeight 1.25
Set Width 1160
Set Height 640
Set Padding 24
Set BorderRadius 10
Set Framerate 30
Set TypingSpeed 35ms

Set Theme { "name": "Bunpo", "black": "#2b2825", "red": "#d17c88", "green": "#43a581", "yellow": "#bf8a40", "blue": "#7da1c4", "magenta": "#b48ead", "cyan": "#74b2a8", "white": "#f0ebe3", "brightBlack": "#7a736b", "brightRed": "#e0919c", "brightGreen": "#5cc29b", "brightYellow": "#d8a457", "brightBlue": "#9ab8d4", "brightMagenta": "#c9a8c4", "brightCyan": "#92c9c0", "brightWhite": "#faf7f2", "background": "#181614", "foreground": "#f0ebe3", "cursor": "#bf8a40", "selection": "#3a3530" }

Env COLORTERM "truecolor"

Hide
Type `set -g fish_greeting ""; set -g fish_autosuggestion_enabled 0`
Enter
Type `set -g fish_color_command faf7f2 --bold; set -g fish_color_param e6dfd5; set -g fish_color_option 92c9c0; set -g fish_color_quote d8a457; set -g fish_color_operator bf8a40; set -g fish_color_end bf8a40; set -g fish_color_comment 7a736b; set -g fish_color_error d17c88; set -g fish_color_normal f0ebe3`
Enter
Type `function fish_prompt; printf '\e[38;2;191;138;64m$\e[0m '; end`
Enter
Type `set -g work (mktemp -d); function __cleanup --on-event fish_exit; rm -rf $work; end; cd $work; mkdir -p tests .bunpo .bunpo-stale`
Enter
Type `for f in auth billing email export health import orders pay reports search users webhooks; touch tests/test_$f.py; end`
Enter
Type `printf '%s\n' '{"version":1,"run":4,"tests":{"tests/test_auth.py":{"samples":[56000],"last_seen":4},"tests/test_billing.py":{"samples":[72000],"last_seen":4},"tests/test_pay.py":{"samples":[78000],"last_seen":4},"tests/test_email.py":{"samples":[48000],"last_seen":4},"tests/test_export.py":{"samples":[58000],"last_seen":4},"tests/test_health.py":{"samples":[44000],"last_seen":4},"tests/test_import.py":{"samples":[60000],"last_seen":4},"tests/test_orders.py":{"samples":[52000],"last_seen":4},"tests/test_reports.py":{"samples":[68000],"last_seen":4},"tests/test_search.py":{"samples":[64000],"last_seen":4},"tests/test_users.py":{"samples":[50000],"last_seen":4},"tests/test_webhooks.py":{"samples":[54000],"last_seen":4}}}' > .bunpo/timings.json`
Enter
Type `printf '%s\n' '{"version":1,"run":1,"tests":{"tests/test_auth.py":{"samples":[95000],"last_seen":1},"tests/test_billing.py":{"samples":[72000],"last_seen":1},"tests/test_pay.py":{"samples":[12000],"last_seen":1},"tests/test_email.py":{"samples":[48000],"last_seen":1},"tests/test_export.py":{"samples":[58000],"last_seen":1},"tests/test_health.py":{"samples":[44000],"last_seen":1},"tests/test_import.py":{"samples":[60000],"last_seen":1},"tests/test_orders.py":{"samples":[52000],"last_seen":1},"tests/test_reports.py":{"samples":[68000],"last_seen":1},"tests/test_search.py":{"samples":[64000],"last_seen":1},"tests/test_users.py":{"samples":[50000],"last_seen":1},"tests/test_webhooks.py":{"samples":[54000],"last_seen":1}}}' > .bunpo-stale/timings.json`
Enter
Type `for i in 0 1 2 3; begin; echo '<testsuite>'; for f in (find tests -name 'test_*.py' | bunpo split --index $i --total 4 2>/dev/null); printf '  <testcase file="%s" name="test_ok" time="55.0"/>\n' $f; end; echo '</testsuite>'; end > junit-$i.xml; end`
Enter
Type `for i in 1 2 3; bunpo record --index $i junit-$i.xml 2>/dev/null; end`
Enter
Type `cp .bunpo/manifest-0.json .bunpo/manifest-1.json .bunpo/manifest-3.json .bunpo-stale/; find tests -name 'test_*.py' | bunpo split --dir .bunpo-stale --index 2 --total 4 >/dev/null 2>&1`
Enter
Type `clear`
Enter
Sleep 400ms
Show

Sleep 500ms

# 1. Split across 4 machines using recorded timings.
Type "find tests -name 'test_*.py' | bunpo split --index 0 --total 4"
Enter
Sleep 1800ms

# 2. Record machine 0's report and merge all 4 reports.
Type "bunpo record --index 0 junit-0.xml"
Enter
Sleep 1000ms

Type "bunpo merge"
Enter
Sleep 1300ms

# 3. Verify that all 4 machines split the same input and timings.
Type "find tests -name 'test_*.py' | bunpo verify"
Enter
Sleep 2200ms

# 4. Show verify catching a stale timing cache on machine 2.
Type "# What if machine 2 restored a stale timing cache?"
Enter
Sleep 500ms

Type "find tests -name 'test_*.py' | bunpo verify --dir .bunpo-stale"
Enter
Sleep 4500ms
