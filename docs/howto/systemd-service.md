# How to Deploy as a systemd Service

Install the deb or rpm package. It includes a systemd unit, default config, and creates an `outrunner` system user.

## 1. Install

```bash
# Ubuntu/Debian
curl -LO https://github.com/NetwindHQ/gha-outrunner/releases/latest/download/outrunner_amd64.deb
sudo dpkg -i outrunner_amd64.deb

# CentOS/RHEL
curl -LO https://github.com/NetwindHQ/gha-outrunner/releases/latest/download/outrunner_amd64.rpm
sudo rpm -i outrunner_amd64.rpm
```

The package creates:
- `/usr/bin/outrunner`
- `/lib/systemd/system/outrunner.service`
- `/etc/outrunner/config.yml` (default config, preserved on upgrade)
- `outrunner` system user and group

## 2. Configure

Edit `/etc/outrunner/config.yml`:

```yaml
url: https://github.com/your-org/your-repo

runners:
  linux:
    labels: [self-hosted, linux]
    docker:
      image: ghcr.io/actions/actions-runner:latest
```

If using Docker, add the outrunner user to the docker group:

```bash
sudo usermod -aG docker outrunner
```

If using libvirt, add to the libvirt group and ensure qcow2 images are readable by the outrunner user:

```bash
sudo usermod -aG libvirt outrunner
sudo chmod -R o+r /var/lib/libvirt/images/ci-runners/
```

## 3. Set Up the Token

### Option A: systemd-creds (recommended, encrypted at rest)

Requires systemd v250+ (Ubuntu 22.04+, Debian 12+).

```bash
echo -n "ghp_YOUR_TOKEN" | sudo systemd-creds encrypt --name=github-token - /etc/outrunner/github-token.cred
sudo chown outrunner:outrunner /etc/outrunner/github-token.cred
```

Then uncomment the `LoadCredentialEncrypted` line in the unit file:

```bash
sudo systemctl edit outrunner
```

Add:

```ini
[Service]
LoadCredentialEncrypted=github-token:/etc/outrunner/github-token.cred
```

The encrypted credential can only be decrypted on this machine. Even if the file is exfiltrated, it's useless elsewhere.

### Option B: Environment file (simpler)

```bash
echo 'GITHUB_TOKEN=ghp_YOUR_TOKEN' | sudo tee /etc/outrunner/env
sudo chmod 600 /etc/outrunner/env
sudo chown outrunner:outrunner /etc/outrunner/env
```

The systemd unit loads this file automatically.

### Option C: Token file

```bash
echo -n "ghp_YOUR_TOKEN" | sudo tee /etc/outrunner/token
sudo chmod 600 /etc/outrunner/token
sudo chown outrunner:outrunner /etc/outrunner/token
```

Add to `/etc/outrunner/config.yml`:

```yaml
token_file: /etc/outrunner/token
```

## 4. Enable and Start

```bash
sudo systemctl enable outrunner
sudo systemctl start outrunner
```

## 5. Check Status

```bash
sudo systemctl status outrunner
sudo journalctl -u outrunner -f
```

## Updating

Package upgrades preserve your config (marked `config|noreplace`). After installing a new version:

```bash
sudo systemctl restart outrunner
```

## Token Rotation

Update the token using whichever method you chose (re-encrypt with systemd-creds, update the env file, or update the token file), then restart:

```bash
sudo systemctl restart outrunner
```
