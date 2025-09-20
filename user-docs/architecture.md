# Architecture Overview

This document provides a comprehensive overview of the Keystone Security Platform architecture, including system components, data flow, and integration patterns.

## System Architecture Overview

Keystone is designed as a comprehensive DevSecOps automation platform that integrates vulnerability scanning, SBOM generation, and policy enforcement into GitHub workflows.

### High-Level Architecture

```mermaid
graph TB
    subgraph "Development Environment"
        DEV[Developer Workstation]
        IDE[IDE/Editor]
        LOCAL[Local Testing]
    end
    
    subgraph "Source Control"
        REPO[GitHub Repository]
        ACTIONS[GitHub Actions]
        REGISTRY[GitHub Container Registry]
    end
    
    subgraph "Security Pipeline"
        TRIVY[Trivy Scanner]
        GRYPE[Grype Scanner]
        SYFT[Syft SBOM Generator]
        COSIGN[Cosign Signing]
    end
    
    subgraph "Vulnerable Application"
        VULN[Go Demo App]
        DOCKER[Container Image]
    end
    
    subgraph "Future Components"
        DASH[React Dashboard]
        API[Go Microservices]
        OPA[Policy Engine]
    end
    
    DEV --> REPO
    REPO --> ACTIONS
    ACTIONS --> TRIVY
    ACTIONS --> GRYPE
    ACTIONS --> SYFT
    ACTIONS --> COSIGN
    TRIVY --> REGISTRY
    GRYPE --> REGISTRY
    SYFT --> REGISTRY
    VULN --> DOCKER
    DOCKER --> TRIVY
    DOCKER --> GRYPE
    
    ACTIONS -.-> DASH
    DASH -.-> API
    API -.-> OPA
```

## Current Implementation Architecture

### Security Pipeline Workflow

```mermaid
sequenceDiagram
    participant Dev as Developer
    participant GH as GitHub Repository
    participant GA as GitHub Actions
    participant T as Trivy
    participant G as Grype
    participant S as Syft
    participant R as Container Registry
    
    Dev->>GH: Push Code/Create PR
    GH->>GA: Trigger Security Workflow
    
    par Vulnerability Scanning
        GA->>T: Execute Trivy Scan
        T->>T: Scan Filesystem/Container
        T->>GA: Return JSON Results
    and
        GA->>G: Execute Grype Scan
        G->>G: Scan Packages
        G->>GA: Return JSON Results
    end
    
    GA->>S: Generate SBOM
    S->>S: Create SPDX/CycloneDX
    S->>GA: Return SBOM Files
    
    GA->>R: Upload Artifacts
    R->>R: Store SBOMs & Reports
    
    GA->>GH: Update Workflow Status
    GH->>Dev: Notify Results
```

### Component Integration Architecture

```mermaid
graph LR
    subgraph "Input Sources"
        FS[Filesystem]
        CONT[Container Images]
        DEPS[Dependencies]
    end
    
    subgraph "Security Scanners"
        TV[Trivy<br/>CVE Database]
        GP[Grype<br/>Anchore Database]
    end
    
    subgraph "SBOM Generation"
        SY[Syft<br/>Catalogers]
        SPDX[SPDX Format]
        CDX[CycloneDX Format]
    end
    
    subgraph "Output Artifacts"
        JSON[JSON Reports]
        SBOMS[SBOM Files]
        LOGS[Workflow Logs]
    end
    
    FS --> TV
    FS --> GP
    FS --> SY
    CONT --> TV
    DEPS --> GP
    DEPS --> SY
    
    TV --> JSON
    GP --> JSON
    SY --> SPDX
    SY --> CDX
    SPDX --> SBOMS
    CDX --> SBOMS
```

## Data Flow Architecture

### Vulnerability Detection Data Flow

```mermaid
flowchart TD
    A[Source Code Repository] --> B[GitHub Actions Trigger]
    B --> C{Parallel Scanning}
    
    C --> D[Trivy Filesystem Scan]
    C --> E[Grype Package Scan]
    
    D --> F[Trivy JSON Output]
    E --> G[Grype JSON Output]
    
    F --> H[Vulnerability Correlation]
    G --> H
    
    H --> I[Combined Security Report]
    I --> J[GitHub Container Registry]
    
    subgraph "Scan Results Structure"
        K[CVE IDs]
        L[Severity Levels]
        M[Package Information]
        N[Fix Recommendations]
    end
    
    I --> K
    I --> L
    I --> M
    I --> N
```

### SBOM Generation Data Flow

```mermaid
flowchart TD
    A[Application Source] --> B[Syft Analysis]
    B --> C{Cataloger Selection}
    
    C --> D[Go Module Cataloger]
    C --> E[File Cataloger]
    C --> F[OS Package Cataloger]
    
    D --> G[Dependency Tree]
    E --> H[File Metadata]
    F --> I[System Packages]
    
    G --> J[SBOM Generation]
    H --> J
    I --> J
    
    J --> K[SPDX JSON]
    J --> L[CycloneDX JSON]
    
    K --> M[Artifact Storage]
    L --> M
    
    subgraph "SBOM Content"
        N[Package Names]
        O[Versions]
        P[Licenses]
        Q[Relationships]
    end
    
    M --> N
    M --> O
    M --> P
    M --> Q
```

## Security Pipeline Implementation

### GitHub Actions Workflow Architecture

```mermaid
flowchart TD
    A[Workflow Trigger] --> B[Environment Setup]
    B --> C[Tool Installation]
    
    C --> D[Go Build Step]
    D --> E{Parallel Security Tasks}
    
    E --> F[Trivy Vulnerability Scan]
    E --> G[Grype Vulnerability Scan]
    E --> H[Syft SBOM Generation]
    
    F --> I[Trivy JSON Report]
    G --> J[Grype JSON Report]
    H --> K[SPDX SBOM]
    H --> L[CycloneDX SBOM]
    
    I --> M[Artifact Upload]
    J --> M
    K --> M
    L --> M
    
    M --> N[Workflow Completion]
    
    subgraph "Workflow Triggers"
        O[Push Events]
        P[Pull Requests]
        Q[Release Creation]
    end
    
    O --> A
    P --> A
    Q --> A
```

### Local Development Architecture

```mermaid
graph TB
    subgraph "Developer Machine"
        subgraph "Container Runtime"
            COL[Docker]
            DOCK[Docker CLI]
        end
        
        subgraph "Security Tools"
            TRIV[Trivy CLI]
            GRYP[Grype CLI]
            SYFT_CLI[Syft CLI]
            COSIGN_CLI[Cosign CLI]
        end
        
        subgraph "Development Tools"
            GO[Go 1.21+]
            GH[GitHub CLI]
            NODE[Node.js 18+]
        end
        
        subgraph "Project Structure"
            SCRIPTS[Validation Scripts]
            VULN_APP[Vulnerable App]
            WORKFLOWS[GitHub Actions]
        end
    end
    
    subgraph "External Services"
        GHUB[GitHub Repository]
        GCREG[GitHub Container Registry]
        DATABASES[Vulnerability Databases]
    end
    
    COL --> DOCK
    DOCK --> TRIV
    DOCK --> GRYP
    
    SCRIPTS --> TRIV
    SCRIPTS --> GRYP
    SCRIPTS --> SYFT_CLI
    
    VULN_APP --> GO
    WORKFLOWS --> GH
    
    GH --> GHUB
    TRIV --> DATABASES
    GRYP --> DATABASES
    
    GHUB --> GCREG
```

## Technology Stack Architecture

### Current Implementation Stack

```mermaid
graph TD
    subgraph "CI/CD Layer"
        GA[GitHub Actions]
        GAR[GitHub Container Registry]
    end
    
    subgraph "Security Tools Layer"
        TV[Trivy v0.45+]
        GP[Grype v0.70+]
        SY[Syft v0.90+]
        CS[Cosign v2.0+]
    end
    
    subgraph "Runtime Layer"
        COL[Colima]
        DOCKER[Docker CLI]
        GO[Go 1.21+]
    end
    
    subgraph "Platform Layer"
        MACOS[macOS]
        BREW[Homebrew]
    end
    
    GA --> TV
    GA --> GP
    GA --> SY
    GA --> CS
    
    TV --> DOCKER
    GP --> DOCKER
    SY --> GO
    
    DOCKER --> COL
    GO --> MACOS
    COL --> MACOS
    
    BREW --> TV
    BREW --> GP
    BREW --> SY
    BREW --> CS
```

### Planned Future Architecture

```mermaid
graph TB
    subgraph "Frontend Layer"
        REACT[React 18.2+]
        TS[TypeScript 5.3+]
        TAILWIND[Tailwind CSS]
    end
    
    subgraph "Backend Layer"
        GIN[Gin HTTP Framework]
        MICROSERVICES[Go Microservices]
        SQLITE[SQLite Database]
    end
    
    subgraph "Security Layer"
        OPA_ENGINE[OPA Policy Engine]
        SIGSTORE[Sigstore Integration]
        JWT[JWT Authentication]
    end
    
    subgraph "Integration Layer"
        GH_OAUTH[GitHub OAuth]
        OIDC[OIDC Provider]
        WEBHOOKS[GitHub Webhooks]
    end
    
    REACT --> GIN
    TS --> MICROSERVICES
    GIN --> SQLITE
    
    MICROSERVICES --> OPA_ENGINE
    OPA_ENGINE --> SIGSTORE
    
    JWT --> GH_OAUTH
    GH_OAUTH --> OIDC
    WEBHOOKS --> MICROSERVICES
```

## Deployment Architecture

### GitHub Actions Deployment

```mermaid
flowchart LR
    subgraph "Source"
        REPO[Repository]
        TRIGGER[Workflow Trigger]
    end
    
    subgraph "GitHub Actions Runner"
        SETUP[Environment Setup]
        BUILD[Application Build]
        SCAN[Security Scanning]
        UPLOAD[Artifact Upload]
    end
    
    subgraph "Artifact Storage"
        REGISTRY[Container Registry]
        RELEASES[GitHub Releases]
        PACKAGES[GitHub Packages]
    end
    
    subgraph "Outputs"
        REPORTS[Security Reports]
        SBOMS[SBOM Files]
        ATTESTATIONS[Signed Attestations]
    end
    
    REPO --> TRIGGER
    TRIGGER --> SETUP
    SETUP --> BUILD
    BUILD --> SCAN
    SCAN --> UPLOAD
    
    UPLOAD --> REGISTRY
    UPLOAD --> RELEASES
    UPLOAD --> PACKAGES
    
    REGISTRY --> REPORTS
    REGISTRY --> SBOMS
    REGISTRY --> ATTESTATIONS
```

### Local Development Deployment

```mermaid
graph TB
    subgraph "Development Workflow"
        CLONE[Repository Clone]
        SETUP[Local Setup]
        VALIDATE[Validation Scripts]
        DEVELOP[Development]
    end
    
    subgraph "Local Testing"
        BUILD[Local Build]
        SCAN_LOCAL[Local Scanning]
        SBOM_LOCAL[Local SBOM]
        VERIFY[Verification]
    end
    
    subgraph "Integration Testing"
        ACT[Act Workflow Testing]
        MOCK[Mock Services]
        E2E[End-to-End Tests]
    end
    
    CLONE --> SETUP
    SETUP --> VALIDATE
    VALIDATE --> DEVELOP
    
    DEVELOP --> BUILD
    BUILD --> SCAN_LOCAL
    SCAN_LOCAL --> SBOM_LOCAL
    SBOM_LOCAL --> VERIFY
    
    VERIFY --> ACT
    ACT --> MOCK
    MOCK --> E2E
```

## Security Architecture

### Threat Model Architecture

```mermaid
flowchart TD
    subgraph "Assets"
        SOURCE[Source Code]
        DEPS[Dependencies]
        ARTIFACTS[Build Artifacts]
        CREDS[Credentials]
    end
    
    subgraph "Threats"
        VULN[Vulnerabilities]
        SUPPLY[Supply Chain Attacks]
        MALWARE[Malicious Code]
        LEAK[Credential Leakage]
    end
    
    subgraph "Controls"
        SCANNING[Vulnerability Scanning]
        SBOM_GEN[SBOM Generation]
        SIGNING[Digital Signing]
        POLICIES[Security Policies]
    end
    
    subgraph "Monitoring"
        ALERTS[Security Alerts]
        COMPLIANCE[Compliance Checking]
        AUDIT[Audit Logging]
    end
    
    SOURCE --> VULN
    DEPS --> SUPPLY
    ARTIFACTS --> MALWARE
    CREDS --> LEAK
    
    SCANNING --> VULN
    SBOM_GEN --> SUPPLY
    SIGNING --> MALWARE
    POLICIES --> LEAK
    
    SCANNING --> ALERTS
    SBOM_GEN --> COMPLIANCE
    SIGNING --> AUDIT
```

### Security Pipeline Architecture

```mermaid
sequenceDiagram
    participant Dev as Developer
    participant Repo as Repository
    participant Scanner as Security Scanner
    participant Policy as Policy Engine
    participant Registry as Artifact Registry
    participant Monitor as Monitoring
    
    Dev->>Repo: Commit Code
    Repo->>Scanner: Trigger Scan
    Scanner->>Scanner: Vulnerability Analysis
    Scanner->>Policy: Submit Results
    Policy->>Policy: Evaluate Policies
    
    alt Policy Pass
        Policy->>Registry: Store Artifacts
        Registry->>Monitor: Log Success
    else Policy Fail
        Policy->>Dev: Block & Notify
        Policy->>Monitor: Log Failure
    end
    
    Monitor->>Dev: Send Reports
```

## API Architecture (Planned)

### RESTful API Design

```mermaid
graph TB
    subgraph "API Gateway"
        GATEWAY[API Gateway]
        AUTH[Authentication]
        RATE[Rate Limiting]
    end
    
    subgraph "Microservices"
        VULN_API[Vulnerability API]
        SBOM_API[SBOM API]
        POLICY_API[Policy API]
        ATTEST_API[Attestation API]
    end
    
    subgraph "Data Layer"
        VULN_DB[Vulnerability DB]
        SBOM_DB[SBOM Storage]
        POLICY_DB[Policy Storage]
        AUDIT_DB[Audit Logs]
    end
    
    GATEWAY --> AUTH
    AUTH --> RATE
    RATE --> VULN_API
    RATE --> SBOM_API
    RATE --> POLICY_API
    RATE --> ATTEST_API
    
    VULN_API --> VULN_DB
    SBOM_API --> SBOM_DB
    POLICY_API --> POLICY_DB
    ATTEST_API --> AUDIT_DB
```

## Data Architecture

### Data Storage Strategy

```mermaid
erDiagram
    VULNERABILITY {
        string id PK
        string cve_id
        string severity
        string package_name
        string version
        string scanner_source
        datetime discovered_at
    }
    
    SBOM {
        string id PK
        string format
        string version
        json content
        string checksum
        datetime created_at
    }
    
    SCAN_RESULT {
        string id PK
        string repository
        string commit_sha
        string branch
        datetime scanned_at
    }
    
    POLICY {
        string id PK
        string name
        string version
        text rego_code
        boolean enabled
        datetime created_at
    }
    
    VULNERABILITY }|--|| SCAN_RESULT : belongs_to
    SBOM }|--|| SCAN_RESULT : belongs_to
    SCAN_RESULT }|--|| POLICY : evaluated_by
```

## Performance Architecture

### Scalability Considerations

```mermaid
graph TD
    subgraph "Load Distribution"
        LB[Load Balancer]
        API1[API Instance 1]
        API2[API Instance 2]
        API3[API Instance N]
    end
    
    subgraph "Caching Layer"
        REDIS[Redis Cache]
        CDN[CDN]
    end
    
    subgraph "Data Storage"
        PRIMARY[Primary DB]
        REPLICA[Read Replica]
        BACKUP[Backup Storage]
    end
    
    subgraph "Processing"
        QUEUE[Job Queue]
        WORKER1[Worker 1]
        WORKER2[Worker N]
    end
    
    LB --> API1
    LB --> API2
    LB --> API3
    
    API1 --> REDIS
    API2 --> CDN
    
    API1 --> PRIMARY
    API2 --> REPLICA
    
    API3 --> QUEUE
    QUEUE --> WORKER1
    QUEUE --> WORKER2
    
    PRIMARY --> BACKUP
```

## Monitoring and Observability

### Observability Architecture

```mermaid
graph TB
    subgraph "Application Layer"
        APPS[Applications]
        METRICS[Metrics Export]
        LOGS[Log Export]
        TRACES[Trace Export]
    end
    
    subgraph "Collection Layer"
        PROMETHEUS[Prometheus]
        LOKI[Loki]
        JAEGER[Jaeger]
    end
    
    subgraph "Visualization"
        GRAFANA[Grafana Dashboards]
        ALERTS[Alert Manager]
    end
    
    subgraph "Storage"
        TSDB[Time Series DB]
        LOG_STORE[Log Storage]
        TRACE_STORE[Trace Storage]
    end
    
    APPS --> METRICS
    APPS --> LOGS
    APPS --> TRACES
    
    METRICS --> PROMETHEUS
    LOGS --> LOKI
    TRACES --> JAEGER
    
    PROMETHEUS --> GRAFANA
    LOKI --> GRAFANA
    JAEGER --> GRAFANA
    
    PROMETHEUS --> ALERTS
    
    PROMETHEUS --> TSDB
    LOKI --> LOG_STORE
    JAEGER --> TRACE_STORE
```

## Future Architecture Roadmap

### Phase 1: Foundation (Completed)
- Multi-scanner vulnerability detection
- SBOM generation and storage
- GitHub Actions integration
- Vulnerable demo application

### Phase 2: Intelligence (Planned)
- Vulnerability correlation and deduplication
- Policy engine integration
- Automated remediation suggestions
- Security metrics and reporting

### Phase 3: Platform (Future)
- Real-time security dashboard
- API ecosystem for integrations
- Multi-repository support
- Enterprise authentication

### Phase 4: Scale (Future)
- Distributed scanning architecture
- Advanced policy as code
- Supply chain security attestation
- AI-powered vulnerability analysis

## Architecture Decision Records

### ADR-001: Scanner Selection
**Decision**: Use Trivy and Grype for vulnerability scanning
**Rationale**: Complementary coverage, active maintenance, strong community support

### ADR-002: SBOM Format Support
**Decision**: Support both SPDX and CycloneDX formats
**Rationale**: Industry standard compliance, broad tool compatibility

### ADR-003: Container Runtime
**Decision**: Use Colima for macOS development
**Rationale**: Avoid Docker Desktop licensing, maintain container compatibility

### ADR-004: GitHub Actions Integration
**Decision**: Native GitHub Actions for CI/CD pipeline
**Rationale**: Zero configuration for GitHub repositories, integrated artifact storage

This architecture documentation provides a comprehensive view of Keystone's current implementation and future roadmap, enabling technical evaluation and system understanding for developers and stakeholders.