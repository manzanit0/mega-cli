# mega-cli

A developer-friendly CLI for [MEGA](https://mega.io), inspired by
[dbxcli](https://github.com/dropbox/dbxcli). Single static Go binary built
on [go-mega](https://github.com/t3rm1n4l/go-mega).

## Install

```sh
task install        # or: go install github.com/manzanit0/mega-cli@latest
```

## Usage

```sh
mega login                          # prompts for email, password, 2FA
mega ls -l /
mega tree /projects -L 2
mega ls -l --sort size /backup      # largest first; -r to reverse
mega find --name '*.pdf' --json | jq -r '.[].path'
mega cp ./photos mega://backup/     # upload; like `aws s3 cp`
mega cp mega://docs/a.pdf ~/Desktop/ # download
mega put ./photos /backup/          # recursive, creates parents
mega put -f notes.md /docs/notes.md # replace; old version goes to trash
pg_dump db | mega put - /db.sql     # upload from stdin
mega get /backup/photos ~/Pictures/ # recursive, atomic, MAC-verified
mega cat /config.yaml | yq .
mega mv /a.txt /archive/
mega rm -r /old                     # to trash; --permanent to destroy
mega mv trash:/old /                # restore
mega link /docs/report.pdf
mega sync -n --delete mega://Photos /Volumes/Backup/Photos  # preview
mega sync --delete -x .DS_Store /Volumes/Backup/Photos mega://Photos
mega du /backup && mega quota
```

Paths are absolute from the Cloud Drive root; `docs/a.txt`, `/docs/a.txt`
and `mega://docs/a.txt` are the same. `cp` and `sync` require the
`mega://` prefix to tell MEGA paths from local ones. `trash:` and `shared:`
prefixes address the Rubbish Bin and incoming shares.

Every command supports `--json`. Progress is shown on a TTY and hidden with
`-q`. Use `-j N` for parallel transfer workers.

## Sync

`mega sync <source> <destination>` makes the destination match the source,
transferring only new and changed files. Like `aws s3 sync`, exactly one
side must be remote, marked with the `mega://` prefix.
`--delete` removes items missing from the source: permanently on disk, or
to the MEGA trash (`--permanent` to destroy). Permanent local deletes ask
for confirmation on a terminal (`-y` skips it). Use `-n` to preview and
`-x` to exclude names by glob.

Changes are detected by size and time. Downloads are stamped with MEGA's
timestamp; uploads happen when a local file is newer than its MEGA copy.

## Authentication

`mega login` stores a session ID and master key (never the password) in the
OS keychain under the service `mega-cli`: macOS Keychain, Secret Service on
Linux, or Windows Credential Manager. `mega logout` revokes the session on
MEGA's servers and removes it locally (`--local` skips the server call).
Session files from older versions are migrated into the keychain
automatically.

On headless machines without a keychain, set `MEGA_CONFIG_DIR` to store the
session in `$MEGA_CONFIG_DIR/session.json` (mode `0600`) instead.

For CI, skip `login` and set `MEGA_EMAIL`, `MEGA_PASSWORD` and optionally
`MEGA_MFA`; nothing is persisted.

## Development

```sh
task lint test build
MEGA_EMAIL=... MEGA_PASSWORD=... task test:live
```

## Limitations

Inherited from go-mega: no server-side copy, no importing public links, no
outgoing share management, and no resumable transfers.
