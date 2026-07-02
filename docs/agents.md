# Using ACE with coding agents

ACE works well with LLM coding agents (Claude Code, Cursor, Copilot, aider, …)
out of the box: the CLI is small and `ace --help` describes the full model,
so an agent that encounters `.env.ace` and runs `--help` can usually figure
things out on its own.

What the help output can't carry is *when* to reach for ace and *your team's
conventions* — that an agent should use `ace env` instead of writing a plain
`.env` file, that secrets must never end up in shell history or the
transcript, or how your team onboards a new recipient. That knowledge belongs
in your project's agent instructions. This page gives you copy-paste starting
points.

## AGENTS.md / CLAUDE.md snippet

Most agents read a project instructions file (`AGENTS.md`, `CLAUDE.md`,
`.cursorrules`, …). Add a section like this and adjust the conventions to
your team:

```markdown
## Secrets and environment variables

Environment variables are managed with ace (https://github.com/middle-management/ace)
and stored encrypted in `.env.ace`, which is safe to commit. Run `ace --help`
for full usage. Rules:

- To run anything that needs secrets, use `ace env -- <command>` instead of
  exporting variables or writing a plain `.env` file.
- To add or update a secret, prefer stdin so values stay out of shell history
  and logs: `ace set < file-with-pairs` (or pipe `KEY=VALUE` lines to it).
  Setting an existing key appends a new value; the latest value wins.
- Decrypted values are sensitive: never write them to disk, commit them, or
  echo them. Only run `ace get` when the user explicitly asks for a value.
- Never edit `.env.ace` by hand — it is append-only and only written by
  `ace set`. Treat merge conflicts in it by keeping both sides' blocks.
- To give a new person or machine access: append their age public key to
  `recipients.txt`, then re-encrypt existing values to the new recipient
  list with `ace get | ace set` (run by someone who can already decrypt).
```

## Claude Code skill

A [skill](https://code.claude.com/docs/en/skills) goes further than an
instructions file: its description tells Claude when to load it, so it
triggers on requests like "add a secret for staging" even before ace is
mentioned. Create `.claude/skills/ace/SKILL.md` in your project:

```markdown
---
name: ace
description: >-
  Manage this project's encrypted environment variables with the ace CLI.
  Use when the user asks to add, read, update, rotate, or share secrets,
  API keys, or env vars, when a command fails for lack of credentials,
  or when .env.ace or recipients.txt come up.
---

# Managing secrets with ace

Secrets live encrypted in `.env.ace` (append-only, safe to commit) and are
encrypted to the age public keys in `recipients.txt`. Decryption requires an
identity, `$XDG_CONFIG_HOME/ace/identity` by default. `ace --help` has the
full reference.

## Reading secrets

- `ace get` prints all KEY=VALUE pairs the local identity can decrypt;
  `ace get KEY` prints selected keys.
- Output is sensitive. Only run `ace get` when the user asked for a value,
  and never redirect it into a file that could be committed.

## Adding or updating secrets

- Ask the user for the value rather than inventing one, then pass it via
  stdin so it stays out of shell history and the transcript:
  `ace set < path/to/pairs.env`, or have the user run
  `ace set KEY=VALUE` themselves.
- Updating uses the same command: setting an existing key appends a new
  value and the latest value wins on read. Nothing is ever deleted.
- Quote values like in a .env file when they contain spaces or newlines:
  `KEY="multi word value"`.

## Running commands that need secrets

- Use `ace env -- <command...>` to run a command with the decrypted
  variables in its environment. Do not export secrets into the shell or
  materialize a plaintext `.env` file.
- In environments without an identity (e.g. some CI jobs), degrade
  gracefully with `ace env --on-missing=warn -- <command...>`.

## Sharing access / onboarding a recipient

1. The new person or machine creates an identity:
   `age-keygen -o "$XDG_CONFIG_HOME/ace/identity"`
2. They add its public key to the recipients:
   `age-keygen -y "$XDG_CONFIG_HOME/ace/identity" >> recipients.txt`
3. Someone who can already decrypt re-encrypts existing values to the new
   recipient list: `ace get | ace set`
4. Commit `.env.ace` and `recipients.txt`.

## Troubleshooting

- "no identity matched any encrypted block": the local identity's public
  key was not among the recipients when the values were set. Onboard it as
  above, or pass the right identity with `-i <file>`.
- `ace get` prints fewer keys than expected: same cause — a recipient can
  only decrypt blocks that were encrypted to it.
- Never hand-edit `.env.ace` to fix problems; append corrected values with
  `ace set` instead.
```

## Permissions

Coding agents typically ask before running shell commands. `ace set`,
`ace env`, and `ace version` neither print nor persist plaintext secrets, so
they are safe to allowlist. Consider keeping `ace get` behind a prompt since
its output is sensitive. For Claude Code, in `.claude/settings.json`:

```json
{
  "permissions": {
    "allow": [
      "Bash(ace set:*)",
      "Bash(ace env:*)",
      "Bash(ace version)"
    ],
    "ask": [
      "Bash(ace get:*)"
    ]
  }
}
```
