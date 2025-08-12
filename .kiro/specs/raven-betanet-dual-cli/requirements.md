# Requirements Document — Raven Betanet 1.1 Dual Bounty Implementation

## Introduction

This project implements two independent CLI tools for Raven Betanet 1.1 bounty compliance within a single repository.

**Tool 1: Spec-Compliance Linter** — validates binaries against the 11 mandatory compliance checks defined in §11 of the Raven Betanet 1.1 specification and generates a Software Bill of Materials (SBOM).

**Tool 2: Chrome-Stable (N or N-2) uTLS Template Generator** — produces deterministic TLS ClientHello templates matching Chrome Stable handshakes, with JA3 fingerprint verification and automatic update when Chrome releases new stable versions.

Both tools share common infrastructure including CLI framework, configuration loading, logging, HTTP utilities, and CI/CD workflows.

## Requirements

### Requirement 1: Spec-Compliance Linter CLI

**User Story:** As a developer, I want a CLI tool that validates my binary against Raven Betanet 1.1 specification requirements, so that I can ensure compliance before deployment.

#### Acceptance Criteria

1. **Execution:**
   - WHEN I run `raven-linter check <binary>` THEN the system SHALL execute all 11 compliance checks from §11 of the Raven Betanet 1.1 spec
   - These checks SHALL be implemented as individual functions returning `{id, description, pass/fail, details}`

2. **Output formats:**
   - `--format json` → Output SHALL follow JSON schema `{check_id, description, status, details}`
   - `--format text` → Output SHALL be human-readable, aligned in columns

3. **SBOM generation:**
   - Output SHALL conform to CycloneDX v1.5 JSON schema or SPDX 2.3 JSON schema
   - SBOM SHALL be saved to `sbom.json` in the working directory

4. **Exit codes:**
   - If any compliance check fails → exit status 1
   - If all checks pass → exit status 0

5. **Test coverage:**
   - Unit tests SHALL verify each compliance check independently using golden files
   - Integration test SHALL validate an entire CLI run with both passing and failing binaries

### Requirement 2: Chrome-Stable (N-2) uTLS Template Generator

**User Story:** As a security researcher, I want a tool that generates deterministic TLS ClientHello templates matching Chrome Stable versions, so that I can replicate Chrome's TLS behavior for testing purposes.

#### Acceptance Criteria

1. **Handshake generation:**
   - WHEN I run `chrome-utls-gen generate` THEN the system SHALL output a binary ClientHello blob identical to Chrome Stable (N or N-2) handshake bytes
   - Determinism SHALL be verified by matching byte-for-byte against a stored golden file

2. **JA3 self-test:**
   - WHEN I run `chrome-utls-gen ja3-test <target>` THEN the system SHALL:
     - Connect to `<target>`
     - Extract the JA3 fingerprint string
     - Verify that the MD5 hash matches the expected Chrome JA3 fingerprint exactly

3. **Update mechanism:**
   - `chrome-utls-gen update` SHALL fetch the latest Chrome Stable version from: `https://chromiumdash.appspot.com/fetch_releases?channel=Stable&platform=Linux`
   - The tool SHALL regenerate the ClientHello blob and update JA3 fingerprint data

4. **Automatic refresh:**
   - A GitHub Actions scheduled workflow SHALL run every 7 days to check for new Chrome versions and commit updated templates

5. **Testing:**
   - Golden files SHALL store captured Chrome handshakes for N and N-2 versions
   - Integration tests SHALL verify handshake generation and JA3 matching logic

### Requirement 3: Shared Project Infrastructure

**User Story:** As a maintainer, I want both CLI tools to share common infrastructure components, so that the codebase is maintainable and consistent.

#### Acceptance Criteria

1. CLI framework SHALL be Cobra (Go) for both tools
2. Logging SHALL use a shared utility in `/internal/utils/logging.go`
3. Configuration SHALL load from env vars and optional `config.yaml` using a shared loader
4. HTTP operations SHALL use a shared client with retry/backoff logic
5. Shared integration test data SHALL reside in `/tests`

### Requirement 4: GitHub Actions CI/CD Integration

**User Story:** As a DevOps engineer, I want automated workflows for both tools, so that I can integrate compliance checking and uTLS generation into my CI/CD pipeline.

#### Acceptance Criteria

1. **Triggers:**
   - On PR creation
   - On push to main
   - On scheduled events (uTLS auto-refresh)

2. **spec-linter.yml workflow SHALL:**
   - Build `raven-linter`
   - Run unit and integration tests
   - Execute on sample binary
   - Upload `sbom.json` as artifact
   - Fail if compliance fails

3. **chrome-utls-gen.yml workflow SHALL:**
   - Build `chrome-utls-gen`
   - Run unit and integration tests
   - Generate ClientHello blob
   - Run JA3 self-test
   - Fail if JA3 mismatch

4. **Secrets handling:**
   - Workflows SHALL NOT expose sensitive values in logs

### Requirement 5: Comprehensive Testing

**User Story:** As a developer, I want comprehensive test coverage for both tools, so that I can confidently deploy and maintain the codebase.

#### Acceptance Criteria

1. Each of the 11 compliance checks SHALL have its own unit test
2. Linter integration tests SHALL simulate both full pass and partial fail
3. TLS generation integration tests SHALL compare generated handshake bytes against golden files
4. JA3 tests SHALL validate exact hash match for known Chrome fingerprints
5. SBOM output tests SHALL validate schema compliance
6. All tests SHALL run in CI and pass before merge

### Requirement 6: Documentation and Usability

**User Story:** As a user, I want clear documentation and examples, so that I can quickly understand and use both CLI tools.

#### Acceptance Criteria

1. Main `README.md` SHALL include installation, usage, and CI integration steps
2. Each CLI SHALL have a `--help` flag that outputs detailed command usage
3. CLI run without arguments SHALL print usage examples
4. Error messages SHALL be actionable, indicating how to fix the issue
5. Linter documentation SHALL list all §11 checks with plain-language explanations

### Requirement 7: Binary Distribution and Packaging

**User Story:** As an end user, I want easy installation and distribution of both CLI tools, so that I can quickly deploy them in my environment.

#### Acceptance Criteria

1. Binaries SHALL be built for Linux, macOS, and Windows via Go cross-compilation
2. Build process SHALL embed version, commit hash, and build date in each binary via Go linker flags
3. Release artifacts SHALL be published in GitHub Releases with checksums
4. Optional package manager integration (Homebrew, apt, winget) SHALL be supported
5. In-place updates SHALL be supported via `--update` flag