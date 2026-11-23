# Production deployment — GitHub Actions + SSH + systemd

This repository's default branch is **main**. The workflow also listens for pushes to **master** if that branch is created. Pull requests are tested; only a successful push to main/master (including a merge commit) triggers production deployment. Production deploy requires the GitHub **production** environment.

## Flow

```text
feature/* -> PR into main -> Go tests & vet
                           |
                         merge
                           v
                      push to main
                           |
                     CI tests & vet
                           |
                 build Linux binary
                           |
                   production job
                           |
               SSH deploy to server
                           |
                  systemd restart
                           |
              GET /ready (localhost)
                           |
                   success / rollback
```

This is single-server deployment, **not zero-downtime**, and the service will briefly restart. It does **not** install PostgreSQL, provision TLS, configure firewall, migrate the database, or create GitHub secrets automatically.

## 1. GitHub configuration

Open **Repository → Settings → Environments → New environment → production**. Optionally require deployment approval and restrict allowed deployment branches to main/master.

Then add the following values in **Settings → Secrets and variables → Actions** or preferably on the **production** environment.

### GitHub Actions secrets (sensitive, masked)

| Name | Content |
|---|---|
| `DEPLOY_SSH_PRIVATE_KEY` | SSH **private** key for deployment user; use a dedicated key |
| `DEPLOY_SSH_KNOWN_HOSTS` | SSH **host public key entry** (known_hosts format), independently fingerprint-verified |
| `DATABASE_URL` | Production PostgreSQL connection string (with DB username/password) |
| `API_KEY` | Matching API secret; generate using `go run ./cmd/genapikey` |

Do **not** commit any of these, including the SSH private key or production DB URL. Note `DATABASE_URL` contains credentials. HTTPS is mandatory for requests outside a trusted private network.

### GitHub Actions variables (nonsecret configuration)

| Name | Example | Meaning |
|---|---|---|
| `DEPLOY_HOST` | `203.0.113.10` | Server public IP / DNS (example address; replace) |
| `DEPLOY_USER` | `deploymatcher` | Dedicated SSH account |
| `DEPLOY_PORT` | `22` | SSH port (default 22) |
| `DEPLOY_GOARCH` | `amd64` | Linux CPU architecture: amd64 or arm64 |
| `APP_PORT` | `8080` | Go HTTP listen port |
| `MAX_CVS` | `500000` | Maximum CV count per request |
| `MATCHING_WORKERS` | `4` | Workers per request (1–64) |
| `ALLOWED_IPS` | `127.0.0.1,192.0.2.55` | Client socket IPs and CIDRs; **must include 127.0.0.1** for local health check |
| `ALLOWED_DOMAINS` | `app.example.com` | Optional browser Origin hostnames; NOT authentication |

If the server IP or username should be hidden too, move `DEPLOY_HOST` and/or `DEPLOY_USER` to GitHub **Secrets** and change the corresponding `vars.*` references in `.github/workflows/deploy.yml` to `secrets.*`. The repository itself contains only placeholder examples.

Generate an SSH key locally:

```bash
ssh-keygen -t ed25519 -a 100 -f ~/.ssh/go_matching_deploy -C "github-actions-go-matching"
# Copy the PUBLIC key into the destination user's ~/.ssh/authorized_keys.
# Copy the PRIVATE key (not the public key) into DEPLOY_SSH_PRIVATE_KEY.
```

Host key entry: obtain using `ssh-keyscan -H SERVER_HOST` **then verify its fingerprint independently**, e.g. via the server console `ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub`. Do not blindly trust `ssh-keyscan` output; otherwise first connection is vulnerable to interception.

## 2. Prepare the Ubuntu server (one-time, manual)

Install systemd, `curl`, `tar`, and configure PostgreSQL separately. The service connects to existing PostgreSQL over `DATABASE_URL`. Ensure DB access and tables (see `migrations/001_init.sql`); migrations are **not executed automatically** during deploy.

As a privileged operator:

```bash
sudo apt-get update
sudo apt-get install -y ca-certificates curl tar openssh-server
sudo adduser --disabled-password --gecos "" deploymatcher
sudo install -d -o deploymatcher -g deploymatcher -m 0700 /home/deploymatcher/.ssh
sudo touch /home/deploymatcher/.ssh/authorized_keys
sudo chown deploymatcher:deploymatcher /home/deploymatcher/.ssh/authorized_keys
sudo chmod 0600 /home/deploymatcher/.ssh/authorized_keys
# Append the deploy PUBLIC key to authorized_keys, not the private key.
```

Install the reviewed installer from **the checked-out and reviewed repository** as a **root-owned, immutable-to-deploy-user** command:

```bash
sudo install -o root -g root -m 0755 scripts/install_release.sh /usr/local/sbin/go-matching-install
sudo visudo -f /etc/sudoers.d/go-matching-deploy
```

In the sudoers file, write **exactly**:

```text
deploymatcher ALL=(root) NOPASSWD: /usr/local/sbin/go-matching-install
```

Verify using `sudo visudo -cf /etc/sudoers.d/go-matching-deploy`. Do not grant unrestricted sudo to the deploy user. Understand that deployment of executable application code is a privileged capability; protect its SSH key and restrict repository write access. Installer updates require review and an explicit manual copy to the server.

The installer creates an unprivileged `matcher` service account, installs the binary under `/opt/go-cv-matching/releases/<commit-sha>/`, stores secrets as `/etc/go-cv-matching/service.env` (mode 0640, root:matcher), and manages the root-owned `/etc/systemd/system/go-cv-matching.service` unit. It switches the `current` symlink to the new version, restarts systemd, checks `/ready`, and restores the previous binary/configuration on failure.

**Security:** the Go app currently listens on `0.0.0.0:APP_PORT`. Restrict public access to that port with the host firewall and/or place TLS-terminating Nginx in front. When behind a reverse proxy, Go sees the proxy's source IP, not necessarily the original client. Enforce remote client-IP restrictions at the trusted proxy. Browser `Origin` is spoofable and cannot replace API-key authentication.

## 3. Deploy

1. Open a PR to `main`. CI runs Go tests and `go vet`.
2. Merge the PR after CI passes.
3. The push to `main` triggers another build and a production deploy job.
4. If an environment approval gate is enabled, approve production deployment in the GitHub Actions UI.
5. Inspect the **Actions → CI and production deploy** run.

On server:

```bash
sudo systemctl status go-cv-matching
sudo journalctl -u go-cv-matching -n 100 --no-pager
curl -f http://127.0.0.1:8080/ready
```

The `/ready` probe is performed locally and requires `127.0.0.1` in `ALLOWED_IPS`, plus working DB connectivity. Public access from another client may still return 403 unless its IP is allowlisted.

## 4. Troubleshooting and rollback

- `401`: API key missing/wrong (`X-API-Key`).
- `403`: remote peer IP/Origin denied. Check whether app runs behind Nginx.
- `503 /ready`: PostgreSQL unreachable or invalid credentials.
- `sudo: a password is required`: the sudoers rule does not permit the installed command.
- `Host key verification failed`: known_hosts value mismatch; **verify actual host identity**, do not disable host-key checking.
- `go mod tidy` CI failure: Go dependencies/network unavailable. The current repository previously lacked a complete `go.sum`; CI fetches modules afresh.

The installer automatically rolls back the old binary and environment on failed local readiness. It does not roll back DB migrations and it does not guarantee zero downtime. Previous releases stay in `/opt/go-cv-matching/releases`; manual rollback is possible by atomically changing the `current` symlink and restarting the systemd service. Limit old-release retention and keep PostgreSQL backups.

## Security notes

- GitHub **Secrets** are designed to store credentials; GitHub **Variables** are for nonsecret settings. Neither goes in tracked source code.
- Avoid printing secrets, private keys, `service.env` contents or database URLs in logs.
- Rotate deploy SSH key, database credentials and API key if exposed.
- Use branch protection and production environment approval for safe release management.
- On a single-server install, changing secrets/variables requires a new successful deployment push to propagate them.
