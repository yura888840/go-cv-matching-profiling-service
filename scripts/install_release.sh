#!/usr/bin/env bash
# Executed with sudo on the destination host. Assumes trusted GitHub Actions
# deploy workflow and a dedicated deploy user with scoped administrative access.
set -euo pipefail
stage="${1:?staging directory required}"
sha="${2:?commit SHA required}"
[[ "$sha" =~ ^[a-f0-9]{40}$ ]] || { echo "Invalid commit SHA" >&2; exit 1; }
[[ "$stage" == "/tmp/go-matching-$sha" ]] || { echo "Invalid staging directory" >&2; exit 1; }
[[ -f "$stage/release.tar.gz" && -f "$stage/service.env" ]] || { echo "Missing release/config" >&2; exit 1; }

base=/opt/go-cv-matching
release="$base/releases/$sha"
env_dir=/etc/go-cv-matching
env_file="$env_dir/service.env"
unit_file=/etc/systemd/system/go-cv-matching.service
previous=""
if [[ -L "$base/current" ]]; then previous="$(readlink -f "$base/current")"; fi
backup="$(mktemp /tmp/go-matching-env.XXXXXXXX)"
had_env=false
if [[ -f "$env_file" ]]; then cp -p "$env_file" "$backup"; had_env=true; fi
activated=false

rollback() {
  code=$?
  if [[ "$code" -ne 0 && "$activated" == true ]]; then
    echo "Deployment failed; restoring previous release" >&2
    if [[ -n "$previous" ]]; then
      ln -sfn "$previous" "$base/current.next"
      mv -Tf "$base/current.next" "$base/current"
    else
      rm -f "$base/current"
    fi
    if [[ "$had_env" == true ]]; then
      install -o root -g matcher -m 0640 "$backup" "$env_file"
    else
      rm -f "$env_file"
    fi
    systemctl daemon-reload || true
    if [[ -n "$previous" ]]; then systemctl restart go-cv-matching || true
    else systemctl stop go-cv-matching || true; fi
  fi
  rm -f "$backup"
  rm -rf -- "$stage"
  exit "$code"
}
trap rollback EXIT

if ! getent group matcher >/dev/null; then groupadd --system matcher; fi
if ! id -u matcher >/dev/null 2>&1; then
  useradd --system --gid matcher --home-dir /nonexistent --shell /usr/sbin/nologin matcher
fi
install -d -m 0755 -o root -g root "$base" "$base/releases"
install -d -m 0750 -o root -g matcher "$env_dir"
install -d -m 0755 -o root -g root "$release"
tar -xzf "$stage/release.tar.gz" -C "$release"
test -x "$release/matching-api" || chmod 0755 "$release/matching-api"
install -o root -g matcher -m 0640 "$stage/service.env" "$env_file"
install -o root -g root -m 0644 "$release/deploy/go-cv-matching.service" "$unit_file"
ln -sfn "$release" "$base/current.next"
mv -Tf "$base/current.next" "$base/current"
activated=true
systemctl daemon-reload
systemctl enable go-cv-matching
systemctl restart go-cv-matching
# The deployment allowlist must explicitly contain 127.0.0.1.
port="$(sed -n 's/^PORT="\([0-9]\+\)"$/\1/p' "$env_file")"
[[ -n "$port" ]] || { echo "PORT missing in service env" >&2; exit 1; }
ready=false
for i in $(seq 1 30); do
  if curl --fail --silent --show-error --max-time 2 "http://127.0.0.1:$port/ready" >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 1
done
if [[ "$ready" != true ]]; then
  echo "Readiness endpoint failed. Inspect: journalctl -u go-cv-matching" >&2
  exit 1
fi
echo "Release $sha deployed; readiness successful"
