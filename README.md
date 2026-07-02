# ACE: Append-only Encrypted Environment Variables

## Introduction

ACE (Append-only encrypted Environment variables) is a tool designed to securely manage environment variables for different environments and applications. By leveraging age-encryption.org's robust encryption mechanisms, ACE ensures that sensitive information remains secure while providing flexibility through append-only updates. It supports multiple recipients, making it ideal for CI/CD pipelines, shared services, and any application that requires secure, environment-specific configuration.

### Key Features

- **Append-only Updates**: Safely update environment variables without the need to decrypt existing ones.
- **Encrypted Variables**: Utilize age-encryption to secure environment variables, with public keys to monitor changes.
- **Recipient-specific Blocks**: Tailor environment variables to specific recipients, enhancing security and flexibility.
- **Built on age-encryption.org**: Leverages a trusted and secure encryption framework.

## Getting Started

### Installation

Install by downloading a release for your platform and placing it somewhere on your `$PATH`.

Or if you have a Go environment setup you may also install it using `go install github.com/middle-management/ace@latest`.

### Basic Usage

To begin using ACE, follow these simple steps:

1.  **Create a key**:

    ```bash
    age-keygen -o $XDG_CONFIG_HOME/ace/identity
    ```

2.  **Add a recipient**:

    ```bash
    age-keygen -y $XDG_CONFIG_HOME/ace/identity > recipients.txt
    ```

3.  **Set Environment Variables**:

    ```bash
    ace set DATABASE_URL=postgres://example.com/db1 REDIS_URL=redis://example.com/db2
    ace set < .env
    ```

4.  **Retrieve Environment Variables**:

    ```bash
    ace get
    ace get DATABASE_URL
    ```

5.  **Execute Command with Environment**:
    ```bash
    ace env -- <COMMAND WITH ARGS...>
    ```

## Detailed Examples

### Setting and Getting Variables

- **Set a single variable**:

  ```bash
  ace set API_KEY=abc123
  ```

- **Bulk set variables from a file**:

  ```bash
  ace set < .env
  ```

- **Get a specific variable**:

  ```bash
  ace get API_KEY
  ```

- **Get all accessible variables**:

  ```bash
  ace get
  ```

- **Rotate all available keys to the most recent recipients**
  ```bash
  ace get | ace set
  ```

### Using ACE in CI/CD

ACE was meant for a workflow where a project can store all secrets in the git repository while only giving access to certain recipients, such as CI.

### Using ACE with coding agents

`ace --help` is written to be enough for an LLM coding agent to use the tool correctly on its own. To teach agents your team's conventions — and to have them reach for ACE proactively — see [docs/agents.md](docs/agents.md) for copy-paste `AGENTS.md`/`CLAUDE.md` snippets, a Claude Code skill, and permission suggestions.

## API Reference

- `ace set [KEY=VALUE...]`: Sets environment variables. Accepts multiple key-value pairs.
- `ace set < .env`: Sets variables from a file formatted as KEY=VALUE per line.
- `ace get [KEY...]`: Retrieves the values of specified environment variables.
- `ace env -- COMMAND WITH ARGS...`: Executes a command with the environment variables loaded. Use `ace env` as a docker entrypoint to have it load secrets into environment of the command.

### Common Flags

- `-e, --env-file FILE`: Path to the encrypted env file. Defaults to `./.env.ace`.
- `-i, --identity IDENTITY`: Decrypt using the specified identity file. Can be repeated. Defaults to `$XDG_CONFIG_HOME/ace/identity`. (`get` and `env`)
- `-r, --recipient RECIPIENT`: Encrypt to the specified recipient. Can be repeated. (`set`)
- `-R, --recipient-file FILE`: Encrypt to the recipients listed in FILE. Can be repeated. Defaults to `./recipients.txt`. (`set`)
- `--on-missing MODE`: How to handle a missing env-file or identity: `error` (default), `warn` or `ignore`. (`env`)

## Security Considerations

ACE leans on the simple and reliable age-encryption.org. The security of this implementation has not been vetted by security professionals, and keeping keys secure is outside of the scope of this tool.

