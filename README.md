# glyph

A fast TUI **and** pipe-friendly CLI for storing code snippets and shell
commands, then finding, copying, running, and **syncing them across all your
devices**.

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

```sh
go install github.com/joshuademarco/glyph@latest
```

Or build from source:

```sh
go build -o glyph .
```

Cross-compile release binaries for everything at once:

```sh
./scripts/build-all.sh v1.0.0   # writes to ./dist
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
| click | focus a pane / select the clicked row |
| `?` | help |

In the editor, the **folder field autocompletes** from your existing folders and
subfolders, press `⇥` or `→` to accept the ghost suggestion. The editor also
detects `{{variable}}` placeholders and previews the resolved command live. Smart groups (Favorites, Recently used, Dangerous) are computed
automatically; destructive commands (`rm -rf`, `drop table`, `--force`, …) are
flagged on save.

## The CLI

Everything the TUI does, scriptably.

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

Configure a backend once, then `glyph sync` (a two-way, last-write-wins merge by
snippet) on each device.

**GitHub Gist**, needs only HTTPS and a token, so it works on any machine with
network access and nothing to host:

```sh
glyph sync setup gist        # paste a PAT with the "gist" scope
glyph sync                    # creates a private gist on first run
# on another device:
glyph sync setup gist <id>   # reuse the id printed above
glyph sync
```

**File**, point at a file inside a folder another tool already replicates. No
account, no token, broadest compatibility:

```sh
glyph sync setup file ~/Dropbox/glyph/snippets.json
glyph sync
```

Both implement the same `Backend` interface
([`internal/sync`](./internal/sync)), so a custom HTTP endpoint can be dropped in
later without touching the rest of the app.

## Data & config

`glyph where` prints the locations. By default they live under your OS config dir
(`%AppData%\glyph` on Windows, `~/Library/Application Support/glyph` on macOS,
`$XDG_CONFIG_HOME/glyph` on Linux). Override with `GLYPH_HOME`.

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
