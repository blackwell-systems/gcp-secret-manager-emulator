# Security Policy

## Scope

This project is a **testing emulator** designed for local development and CI/CD environments. It is explicitly **not intended for production use** or security-sensitive applications.

## Security Features

- No authentication by design (testing-only tool)
- In-memory storage (no persistent data)
- No network encryption (local gRPC only)
- Runs as non-root user in Docker container

## Reporting Security Issues

If you discover a security vulnerability in this emulator that could affect users in development or testing environments, please report it privately:

**Email:** dayna@blackwell-systems.com

**Please include:**
- Description of the vulnerability
- Steps to reproduce
- Potential impact on testing environments
- Suggested fix (if available)

**Response time:** We aim to respond within 48 hours.

## Out of Scope

The following are **not** considered security issues for this project:

- Lack of authentication (by design - testing tool)
- Lack of encryption (by design - local use only)
- Data persistence (by design - ephemeral storage)
- Production use issues (explicitly not supported)
- Performance/DoS in testing scenarios

## Supported Versions

| Version | Supported |
| ------- | --------- |
| 1.0.x   | Yes       |
| 0.2.x   | Yes       |
| 0.1.x   | Yes       |

Security fixes will be backported to supported versions if applicable.

## Security Best Practices

When using this emulator:

- Only use in development/testing environments
- Do not expose the emulator port to untrusted networks
- Do not store real production secrets in the emulator
- Use Docker container to isolate the emulator process
- Restart the emulator between test runs to clear state

## Disclaimer

This emulator is provided "as is" without warranty of any kind. It is not affiliated with Google Cloud Platform and does not implement GCP's security features. Use at your own risk.
