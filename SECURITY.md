# Security

Do not open a public issue for a suspected vulnerability.

Use GitHub's private vulnerability reporting from this repository's Security
tab. Include the affected version, impact, minimal reproduction, and any known
mitigations. Do not include live credentials, private host details, or full
config files that contain secrets.

Useful reports include command injection beyond configured checks, path
traversal or unsafe writes outside intended config/state paths, secret leakage
through alerts, logs, notifier payloads, or JSON status output, and notifier
handling that amplifies or exfiltrates local data unintentionally.

Security fixes are applied on a best-effort basis to the latest release and
the latest code on `main`.
