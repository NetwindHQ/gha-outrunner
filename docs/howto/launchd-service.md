# How to Deploy as a launchd Service

On macOS, install via Homebrew and use `brew services` to manage outrunner as a background service.

## 1. Install

```bash
brew tap NetwindHQ/tap
brew install outrunner
```

## 2. Configure

Create the config directory and file:

```bash
mkdir -p $(brew --prefix)/etc/outrunner
```

Create `$(brew --prefix)/etc/outrunner/config.yml`:

```yaml
url: https://github.com/your-org/your-repo

runners:
  macos:
    labels: [self-hosted, macos]
    tart:
      image: ghcr.io/cirruslabs/macos-sequoia-base:latest
      runner_cmd: /Users/admin/actions-runner/run.sh
      cpus: 4
      memory: 8192
```

## 3. Set Up the Token

### Option A: Environment variable

Create an env file:

```bash
echo 'GITHUB_TOKEN=ghp_YOUR_TOKEN' > $(brew --prefix)/etc/outrunner/env
chmod 600 $(brew --prefix)/etc/outrunner/env
```

### Option B: Token file

```bash
echo -n "ghp_YOUR_TOKEN" > $(brew --prefix)/etc/outrunner/token
chmod 600 $(brew --prefix)/etc/outrunner/token
```

Add to your config.yml:

```yaml
token_file: /opt/homebrew/etc/outrunner/token
```

## 4. Start the Service

```bash
brew services start outrunner
```

Check status:

```bash
brew services list
tail -f $(brew --prefix)/var/log/outrunner.log
```

## Managing the Service

```bash
brew services stop outrunner
brew services restart outrunner
```

## Launch Agent vs Launch Daemon

`brew services` runs as a Launch Agent (your user, starts on login). If you need outrunner to run before login (headless Mac mini), use `sudo brew services start outrunner` which installs a Launch Daemon instead.

Note: Tart requires a user session, so Launch Agent is usually the right choice for Tart-based runners.

## Updating

```bash
brew upgrade outrunner
brew services restart outrunner
```
