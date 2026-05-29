# Glyph

A fast TUI **and** pipe-friendly CLI for storing code snippets and shell
commands, then finding, copying, running, and **syncing them across all your
devices**.

<!--
  Drop a screenshot/GIF of the TUI into ./docs/screenshot.png to fill this slot.
  Suggested size: ~1200×700, dark-dimmed terminal, browse screen with a
  snippet selected so the preview pane is populated.
-->
<p align="center">
  <img src="Preview Image.png" alt="glyph TUI — folders, snippet list, preview" width="820">
</p>

```
┌ Folders ─────┐┌ Snippets · Docker/Compose ───────────────────┐
│ ∗ All        ││ ❯ Recreate one service without deps   yml 2d │
│ ★ Favorites  ││ · Tail logs across all services        sh 5d │
│ ◷ Recent     ││ ★ Prune everything, volumes & nets    sh 1w │
│ ! Dangerous  │└──────────────────────────────────────────────┘
│ ▸ Docker     │┌ Preview ─────────────────────────────────────┐
│ ▸ Kubernetes ││ Recreate one service without deps        yml │
│ ▸ Git        ││ # recreate just the api container            │
└──────────────┘│ docker compose up -d --no-deps ... api       │
 NORMAL  j/k move · ⏎ yank · e edit · n new · / find · ? help
```

Built in Go with the [Charm](https://charm.sh) stack (Bubble Tea + Lipgloss).
The UI follows the GitHub dark-dimmed design in [`TUI Design/`](./TUI%20Design).

## Why

- **One store, two front-ends.** Browse visually in the TUI; capture and reuse
  from scripts via the CLI.
- **Pipelines first.** `cmd | glyph add` to capture, `glyph get x | sh` to run.
- **Runs anywhere.** Pure Go, no cgo, single static binary for Linux, macOS,
  Windows, and the BSDs across amd64 / arm64 / arm / 386 / riscv64.
- **Sync across devices** via a private GitHub Gist or any folder you already
  sync (Dropbox / iCloud / OneDrive / Syncthing).

## Install

If you have a working Go toolchain (1.21+), the one-liner works on every
platform:

```sh
go install github.com/joshuademarco/glyph@latest
```

This drops a `glyph` (`glyph.exe` on Windows) binary into `$(go env GOBIN)` —
typically `~/go/bin` on Linux/macOS or `%USERPROFILE%\go\bin` on Windows. Make
sure that directory is on your `PATH`.

### Linux

```sh
# Go toolchain
go install github.com/joshuademarco/glyph@latest

# …or grab a prebuilt binary from the releases page
curl -L -o glyph https://github.com/joshuademarco/glyph/releases/latest/download/glyph_linux_amd64
chmod +x glyph
sudo mv glyph /usr/local/bin/
```

Add `~/go/bin` to your `PATH` if you used `go install`:

```sh
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.bashrc   # or ~/.zshrc
```

### macOS

```sh
# Go toolchain (Apple Silicon and Intel)
go install github.com/joshuademarco/glyph@latest

# …or prebuilt binary
curl -L -o glyph https://github.com/joshuademarco/glyph/releases/latest/download/glyph_darwin_arm64
chmod +x glyph
sudo mv glyph /usr/local/bin/   # or /opt/homebrew/bin on Apple Silicon
```

If macOS Gatekeeper blocks the first run, allow it once with:

```sh
xattr -d com.apple.quarantine /usr/local/bin/glyph
```

### Windows

```powershell
# Go toolchain
go install github.com/joshuademarco/glyph@latest

# …or grab the prebuilt .exe from the releases page and put it on your PATH
Invoke-WebRequest -Uri https://github.com/joshuademarco/glyph/releases/latest/download/glyph_windows_amd64.exe -OutFile glyph.exe
Move-Item glyph.exe "$env:USERPROFILE\bin\glyph.exe"
```

Add the install directory to your user `PATH`:

```powershell
[Environment]::SetEnvironmentVariable(
  "Path",
  [Environment]::GetEnvironmentVariable("Path", "User") + ";$env:USERPROFILE\go\bin",
  "User"
)
```

Glyph works in Windows Terminal, PowerShell, `cmd.exe`, and inside WSL. Mouse
support and 256-colour styling need a modern terminal — Windows Terminal is the
recommended host.

### Build from source

```sh
git clone https://github.com/joshuademarco/glyph
cd glyph
go build -o glyph .
```

Cross-compile every supported target at once (Linux, macOS, Windows, BSDs;
amd64/arm64/arm/386/riscv64):

```sh
./scripts/build-all.sh v1.0.0    # writes to ./dist
```

## The TUI

Run with no arguments:

```sh
glyph
```

Vim-style, modal, three panes (folders · list · preview):

| Key | Action |
|-----|--------|
| `j` / `k` | move within the focused pane |
| `h` / `l` · `tab` | switch pane |
| `g` / `G` | jump to top / bottom |
| `⏎` / `y` | yank command to clipboard |
| `x` | run the command in your shell |
| `e` / `n` | edit / new snippet |
| `f` | toggle favorite |
| `dd` | delete |
| `/` · `⌃P` | fuzzy command palette |
| click | focus a pane / select the clicked row (works in the editor too) |
| `?` | help |

In the editor, the **folder field autocompletes** from your existing folders and
subfolders, press `⇥` or `→` to accept the ghost suggestion. The editor also
detects `{{variable}}` placeholders and previews the resolved command live.
Smart groups (Favorites, Recently used, Dangerous) are computed automatically;
destructive commands (`rm -rf`, `drop table`, `--force`, …) are flagged on
save.

## The CLI

Everything the TUI does, scriptably. The same commands work on every platform —
only the shell syntax around them differs.

```sh
# capture from a flag…
glyph add -t "k8s restart" -f Kubernetes -l sh \
  -c 'kubectl rollout restart deployment/{{name}} -n {{ns}}'

# …or straight from a pipe (great for "save that last command")
history | tail -1 | glyph add -t "resize image"
cat deploy.sh | glyph add -t deploy -f Ops/Deploy -l sh

# find things
glyph list
glyph list -f Docker --tag compose
glyph search rollout

# use them, `get` prints the raw command for piping
glyph get "k8s restart" | sh
ssh host "$(glyph get 'disk usage')"
glyph copy deploy                      # → system clipboard
glyph run deploy --set name=api --set ns=prod

# manage
glyph edit deploy                      # opens $EDITOR on the body
glyph rm deploy
glyph where                            # where data lives
```

Snippets are referenced by id, unique id-prefix, exact title, or a unique fuzzy
match, so `glyph get deploy` usually just works.

### Per-OS command behavior

`glyph run` executes the snippet body in **your platform's shell** — there is
no portable command syntax, glyph just hands the string to the right
interpreter:

| Platform | Shell used by `glyph run` | Default editor for `glyph edit` / `glyph add` |
|---|---|---|
| Linux / macOS / BSD | `$SHELL` (falls back to `/bin/sh`) | `$GLYPH_EDITOR` → config → `$EDITOR` → `vi` |
| Windows | `cmd.exe /c <body>` | `$GLYPH_EDITOR` → config → `%EDITOR%` → `notepad` |

So a Docker one-liner that works in bash works just as well in `cmd.exe`, but
shell-specific syntax (here-docs, `$( )` substitution, `&&` chains in old
PowerShell) only works on the platforms that understand it. If you want
PowerShell instead of `cmd.exe`, set `GLYPH_EDITOR`/`SHELL` accordingly, or
prefix the snippet with `pwsh -c`.

**Pipe examples** below use POSIX shells. The PowerShell equivalents are the
same commands without the `|`-shell tricks:

```powershell
# bash:    glyph get deploy | sh
# pwsh:
glyph get deploy | Invoke-Expression

# bash:    history | tail -1 | glyph add -t "that thing"
# pwsh:
Get-History | Select-Object -Last 1 -ExpandProperty CommandLine | glyph add -t "that thing"
```

### Pipelining patterns

```sh
# build a command, run it remotely
glyph get 'prune docker' | ssh build-box sh

# fill a template and pipe into kubectl
glyph run 'scale workers' --set n=5 --dry-run | kubectl apply -f -

# capture a working one-liner the moment it works
some-long-command --that=works | tee /dev/tty | glyph add -t "that thing"
```

`glyph get` writes only the command to **stdout**; status messages go to
**stderr**, so pipes stay clean.

## Sync across devices

Glyph stores everything in a single `snippets.json`. Sync is a two-way,
last-write-wins merge **per snippet** (newest `updatedAt` wins; deletes are
tracked too), so two devices that edit different snippets between syncs both
keep their changes.

Configure a backend once with `glyph sync setup`, then run `glyph sync` on each
device whenever you want to push/pull. Run `glyph sync status` at any time to
see what's configured.

### Option A — GitHub Gist (recommended)

Needs only HTTPS plus a personal access token, so it works on any machine with
network access and nothing to host.

1. Create a token at <https://github.com/settings/tokens> with **only the
   `gist` scope**.
2. On the first device:

   ```sh
   glyph sync setup gist        # paste the token when prompted
   glyph sync                    # creates a private gist on first run, prints its id
   ```

3. On each additional device, reuse that gist id:

   ```sh
   glyph sync setup gist <id>
   glyph sync
   ```

The token is stored locally in `config.json` with mode `0600`. To rotate it,
run `glyph sync setup gist` again and paste the new token.

### Option B — Shared file (Dropbox / iCloud / OneDrive / Syncthing)

Point glyph at a file inside a folder that another tool already replicates. No
account, no token, broadest compatibility.

```sh
# Linux / macOS
glyph sync setup file ~/Dropbox/glyph/snippets.json

# Windows (PowerShell)
glyph sync setup file "$env:USERPROFILE\OneDrive\glyph\snippets.json"

glyph sync
```

Run `glyph sync` on each device after the host folder has finished
syncing — that way glyph always merges with the latest remote state.

### Option C — Custom backend

Both built-in backends implement the same `Backend` interface
([`internal/sync`](./internal/sync)), so a custom HTTP endpoint (an S3 bucket, a
private API, …) can be dropped in without touching the rest of the app.

### Automating it

Glyph never syncs on its own — wire it into your shell or scheduler if you want
it automatic:

```sh
# Linux/macOS: a cron entry that syncs every 30 minutes
*/30 * * * *  /usr/local/bin/glyph sync >/dev/null 2>&1
```

```powershell
# Windows: a Scheduled Task that syncs at logon
schtasks /Create /SC ONLOGON /TN "glyph sync" /TR "glyph sync"
```

## Data & config

`glyph where` prints the locations. By default they live under your OS config
dir:

| OS | Default path |
|---|---|
| Linux | `$XDG_CONFIG_HOME/glyph` (falls back to `~/.config/glyph`) |
| macOS | `~/Library/Application Support/glyph` |
| Windows | `%AppData%\glyph` (e.g. `C:\Users\you\AppData\Roaming\glyph`) |

Override the whole directory with `GLYPH_HOME`.

- `snippets.json`, your library (plain JSON, easy to back up or hand-edit).
- `config.json`, sync settings (tokens stored `0600`).

## Layout

```
main.go                 entry point
internal/
  model/                Snippet & Library types, variable parsing
  store/                JSON persistence, filtering, fuzzy search
  config/               paths & settings
  clipboard/            native clipboard + OSC 52 fallback (works over SSH)
  sync/                 Backend interface, Gist + File backends, merge
  cli/                  Cobra commands (add/get/run/sync/…)
  tui/                  Bubble Tea UI (browse/editor/palette/help)
TUI Design/             the original design mockups this UI follows
scripts/build-all.sh    cross-compile matrix
```

## License

MIT
