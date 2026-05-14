# secrets/

Plaintext secret files referenced by `deploy.yaml` via the
`file:` scheme.

## How it works

In `deploy.yaml`:

```yaml
env:
  COCO_IAM_DB_PASSWORD: file:./secrets/db-password
```

At deploy time the tool reads `./secrets/db-password`, trims
trailing newlines, and ships the plaintext to the target as
`COCO_IAM_DB_PASSWORD` in `<target_dir>/shared/.env`. Engines
never see the `file:` prefix.

## Layout

One secret per file. Filename is arbitrary — pick something
obvious that matches the env var name:

```
secrets/
├── db-password
├── session-secret
├── smtp-password
└── admin-password
```

## Safety

`secrets/.gitignore` ignores every file in this directory by
default (`*`) so a stray `git add` can't commit plaintext. Only
this README and the gitignore itself are tracked.

Never loosen the gitignore. Never print a secret in a Makefile
recipe, a commit message, or a CI log. If a secret leaks, rotate
it at the source (database, IdP, SMTP provider) before doing
anything else.

## Writing a secret

```sh
# Pipe avoids leaving the value in your shell history.
printf 'my-super-secret' > secrets/db-password
chmod 600 secrets/db-password
```

## Alternatives

- `env:<NAME>` reads from the operator's shell env instead of a
  file. Prefer this in CI where the secret manager injects env
  vars.
- `literal:<value>` forces literal interpretation when the value
  would otherwise look like a scheme. Never use this for real
  secrets — YAML files are easy to accidentally commit.
