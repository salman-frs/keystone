-- Description: Create initial database schema for vulnerability tracking and external service caching

-- +migrate Up
CREATE TABLE vulnerability_cache (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    cve_id TEXT UNIQUE NOT NULL,
    severity TEXT NOT NULL,
    description TEXT,
    cvss_score REAL,
    published_date DATETIME,
    modified_date DATETIME,
    source TEXT NOT NULL, -- 'nvd', 'github', 'trivy', 'grype'
    raw_data TEXT, -- JSON blob of original data
    cache_expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE external_service_status (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    service_name TEXT UNIQUE NOT NULL, -- 'github', 'nvd', 'sigstore'
    is_available BOOLEAN NOT NULL DEFAULT TRUE,
    last_check DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_failure DATETIME,
    failure_count INTEGER NOT NULL DEFAULT 0,
    response_time_ms INTEGER,
    rate_limit_remaining INTEGER,
    rate_limit_reset DATETIME,
    circuit_breaker_state TEXT NOT NULL DEFAULT 'CLOSED', -- 'CLOSED', 'OPEN', 'HALF_OPEN'
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE scan_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    scan_id TEXT UNIQUE NOT NULL,
    repository_owner TEXT NOT NULL,
    repository_name TEXT NOT NULL,
    scan_type TEXT NOT NULL, -- 'trivy', 'grype', 'combined'
    status TEXT NOT NULL, -- 'pending', 'running', 'completed', 'failed'
    started_at DATETIME NOT NULL,
    completed_at DATETIME,
    total_vulnerabilities INTEGER DEFAULT 0,
    critical_count INTEGER DEFAULT 0,
    high_count INTEGER DEFAULT 0,
    medium_count INTEGER DEFAULT 0,
    low_count INTEGER DEFAULT 0,
    raw_output TEXT, -- JSON blob of scanner output
    correlation_id TEXT, -- Links to vulnerability correlation
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE vulnerability_correlations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    correlation_id TEXT UNIQUE NOT NULL,
    cve_id TEXT NOT NULL,
    scan_id TEXT NOT NULL,
    package_name TEXT NOT NULL,
    package_version TEXT NOT NULL,
    fixed_version TEXT,
    severity_trivy TEXT,
    severity_grype TEXT,
    severity_nvd TEXT,
    severity_final TEXT NOT NULL, -- Correlated severity
    confidence_score REAL NOT NULL, -- 0.0 to 1.0
    is_false_positive BOOLEAN NOT NULL DEFAULT FALSE,
    remediation_advice TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (scan_id) REFERENCES scan_results(scan_id)
);

-- Create indexes for performance
CREATE INDEX idx_vulnerability_cache_cve_id ON vulnerability_cache(cve_id);
CREATE INDEX idx_vulnerability_cache_expires ON vulnerability_cache(cache_expires_at);
CREATE INDEX idx_vulnerability_cache_source ON vulnerability_cache(source);
CREATE INDEX idx_external_service_status_name ON external_service_status(service_name);
CREATE INDEX idx_scan_results_repository ON scan_results(repository_owner, repository_name);
CREATE INDEX idx_scan_results_status ON scan_results(status);
CREATE INDEX idx_scan_results_started ON scan_results(started_at);
CREATE INDEX idx_vulnerability_correlations_scan_id ON vulnerability_correlations(scan_id);
CREATE INDEX idx_vulnerability_correlations_cve_id ON vulnerability_correlations(cve_id);
CREATE INDEX idx_vulnerability_correlations_severity ON vulnerability_correlations(severity_final);

-- +migrate Down
DROP INDEX IF EXISTS idx_vulnerability_correlations_severity;
DROP INDEX IF EXISTS idx_vulnerability_correlations_cve_id;
DROP INDEX IF EXISTS idx_vulnerability_correlations_scan_id;
DROP INDEX IF EXISTS idx_scan_results_started;
DROP INDEX IF EXISTS idx_scan_results_status;
DROP INDEX IF EXISTS idx_scan_results_repository;
DROP INDEX IF EXISTS idx_external_service_status_name;
DROP INDEX IF EXISTS idx_vulnerability_cache_source;
DROP INDEX IF EXISTS idx_vulnerability_cache_expires;
DROP INDEX IF EXISTS idx_vulnerability_cache_cve_id;

DROP TABLE IF EXISTS vulnerability_correlations;
DROP TABLE IF EXISTS scan_results;
DROP TABLE IF EXISTS external_service_status;
DROP TABLE IF EXISTS vulnerability_cache;