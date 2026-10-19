# Adopters

Projects that run the GCP Secret Manager Emulator, each confirmed in the project's own repository. Every entry links to the file where the emulator is used. Last checked October 2026.

| Project | What it is | How it uses the emulator |
|---|---|---|
| [Apereo CAS](#apereo-cas) (11.4K stars) | Identity and single sign-on platform | CI tests for its GCP Secret Manager support |
| [Flipt](#flipt) (4.9K stars) | Feature flag platform | Integration tests |
| [Camunda](#camunda) (4.3K stars) | Process orchestration | End-to-end tests of its GCP secret store |
| [minisky](#minisky) | Local GCP emulator | Its Secret Manager service |
| [dature](#dature) | Type-safe configuration loader for Python | Integration tests |
| [Status List Server](#status-list-server) (adorsys) | Status list server for verifiable credentials | Rust integration tests |
| [suve](#suve) | CLI and TUI for cloud parameter and secret stores | Test and end-to-end environments |
| [SeedStream](#seedstream) | Test data generator | Integration tests |
| [dbadmin-mono](#dbadmin-mono) | Database admin dashboard | Docker Compose secrets setup |

## Apereo CAS

[Apereo CAS](https://github.com/apereo/cas) is an open-source identity and single sign-on platform. Its CI starts the dual-protocol image to test CAS's GCP Secret Manager support.

- [`ci/tests/gcp/run-gcp-server.sh`](https://github.com/apereo/cas/blob/HEAD/ci/tests/gcp/run-gcp-server.sh): `ghcr.io/blackwell-systems/gcp-secret-manager-emulator-dual:latest`

## Flipt

[Flipt](https://github.com/flipt-io/flipt) is a Git-native feature management platform. Its integration test harness runs the emulator as a container.

- [`build/testing/integration.go`](https://github.com/flipt-io/flipt/blob/HEAD/build/testing/integration.go): `ghcr.io/blackwell-systems/gcp-secret-manager-emulator:1.3`

## Camunda

[Camunda](https://github.com/camunda/camunda) is a process orchestration platform. Its GCP secret store has an end-to-end test suite that runs against the emulator through Testcontainers, pinned by image digest. From the source:

> GCP ships no official Secret Manager emulator, so this uses the `blackwell-systems` image, which implements the full Secret Manager gRPC API without requiring credentials.

- [`SecretManagerEmulatorContainer.java`](https://github.com/camunda/camunda/blob/HEAD/secret-store/secret-store-gcp/src/test/java/io/camunda/secretstore/gcp/util/SecretManagerEmulatorContainer.java): `ghcr.io/blackwell-systems/gcp-secret-manager-emulator-dual:1.9`

## minisky

[minisky](https://github.com/qamarudeenm/minisky) is a local Google Cloud emulator for development. Its Secret Manager service is this emulator, installed from source.

- [`docker/secretmanager.Dockerfile`](https://github.com/qamarudeenm/minisky/blob/HEAD/docker/secretmanager.Dockerfile): `go install github.com/blackwell-systems/gcp-secret-manager-emulator/cmd/server-dual@latest`

## dature

[dature](https://github.com/reagento/dature) is a type-safe configuration loader for Python dataclasses with a GCP Secret Manager source. Its integration tests run the emulator, and pair it with the [GCP IAM Emulator](https://github.com/blackwell-systems/gcp-iam-emulator) in strict mode to test authorization failures.

- [`tests/integration/sources/gcp_secret_manager/conftest.py`](https://github.com/reagento/dature/blob/HEAD/tests/integration/sources/gcp_secret_manager/conftest.py): `ghcr.io/blackwell-systems/gcp-secret-manager-emulator-dual`

## Status List Server

[Status List Server](https://github.com/adorsys/status-list-server), from adorsys, publishes status lists for verifiable credential issuers, implementing the IETF OAuth Token Status List draft. Its Rust integration tests use the emulator.

- [`tests/gcp_secrets_manager.rs`](https://github.com/adorsys/status-list-server/blob/HEAD/tests/gcp_secrets_manager.rs): `ghcr.io/blackwell-systems/gcp-secret-manager-emulator-dual`

## suve

[suve](https://github.com/mpyw/suve) is a Git-like CLI, TUI and GUI for AWS Parameter Store, AWS Secrets Manager and Google Secret Manager. Its test and end-to-end environments run the emulator.

- [`compose.yaml`](https://github.com/mpyw/suve/blob/HEAD/compose.yaml) and [`compose.test.yaml`](https://github.com/mpyw/suve/blob/HEAD/compose.test.yaml): `ghcr.io/blackwell-systems/gcp-secret-manager-emulator-dual:latest`

## SeedStream

[SeedStream](https://github.com/mferretti/SeedStream) is a test data generator for enterprise applications. Its Google Secret Manager resolver is tested against the emulator.

- [`GoogleSecretManagerResolverIT.java`](https://github.com/mferretti/SeedStream/blob/HEAD/schema/src/test/java/com/datagenerator/schema/secret/GoogleSecretManagerResolverIT.java): `ghcr.io/blackwell-systems/gcp-secret-manager-emulator-dual:latest`

## dbadmin-mono

[dbadmin-mono](https://github.com/lagdo/dbadmin-mono) is a database admin dashboard built on Jaxon and Adminer. Its secrets setup runs the emulator in Docker Compose.

- [`docker/compose-secrets/docker-compose.yml`](https://github.com/lagdo/dbadmin-mono/blob/HEAD/docker/compose-secrets/docker-compose.yml): `ghcr.io/blackwell-systems/gcp-secret-manager-emulator-dual:latest`

## Written about

- [Building a Multi-Tenant Dev Environment](https://github.com/bubunyo/bubunyo.github.io/blob/HEAD/content/posts/2026-07-14-multi-tenant-dev-env.md), a post on faking Secret Manager in a local cluster with the [GCP Emulator](https://github.com/blackwell-systems/gcp-emulator).

## Using it in your project?

Open a pull request adding your project to this page, or [open an issue](https://github.com/blackwell-systems/gcp-secret-manager-emulator/issues) with a link to where you use it.
