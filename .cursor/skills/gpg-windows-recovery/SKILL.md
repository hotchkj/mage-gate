---
name: gpg-windows-recovery
description: >-
  Recover a stalled GPG commit signing agent on Windows. Use when git commit
  hangs, errors on GPG, "cannot connect to agent", pinentry stuck, or
  gpg failed to sign the data.
---

# GPG Windows Recovery

Never disable signing. --no-gpg-sign and commit.gpgsign=false are forbidden.

## Preferred reset (confirmed working)

gpgconf --kill gpg-agent
gpgconf --launch gpg-agent
git commit --amend --no-edit -S   (or repeat original commit with -S for new commits)
git log -1 --format='%G?' → must return G

## Windows process fallback (if gpgconf --launch fails)

taskkill /F /IM gpg-agent.exe
taskkill /F /IM pinentry-basic.exe   (also: pinentry-w32.exe, pinentry-qt.exe)
Then repeat preferred reset steps.

## Escalate

Stop and surface to user after two full reset cycles without G signature.

Policy: docs/kb/gpg-windows-recovery.md
