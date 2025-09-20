-- Description: Add policy evaluation and compliance tracking tables

-- +migrate Up
CREATE TABLE policy_definitions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    policy_id TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    policy_type TEXT NOT NULL, -- 'security', 'compliance', 'custom'
    rego_policy TEXT NOT NULL, -- OPA/Rego policy content
    version TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    severity_threshold TEXT NOT NULL DEFAULT 'MEDIUM', -- 'LOW', 'MEDIUM', 'HIGH', 'CRITICAL'
    created_by TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE policy_evaluations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    evaluation_id TEXT UNIQUE NOT NULL,
    policy_id TEXT NOT NULL,
    scan_id TEXT NOT NULL,
    evaluation_result TEXT NOT NULL, -- 'PASS', 'FAIL', 'WARNING', 'SKIP'
    decision_details TEXT, -- JSON blob with OPA decision details
    violations_count INTEGER DEFAULT 0,
    warnings_count INTEGER DEFAULT 0,
    evaluation_time_ms INTEGER,
    evaluated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (policy_id) REFERENCES policy_definitions(policy_id),
    FOREIGN KEY (scan_id) REFERENCES scan_results(scan_id)
);

CREATE TABLE compliance_reports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id TEXT UNIQUE NOT NULL,
    scan_id TEXT NOT NULL,
    compliance_framework TEXT NOT NULL, -- 'SLSA', 'NIST', 'CIS', 'CUSTOM'
    overall_score REAL NOT NULL, -- 0.0 to 100.0
    passed_policies INTEGER DEFAULT 0,
    failed_policies INTEGER DEFAULT 0,
    warning_policies INTEGER DEFAULT 0,
    total_policies INTEGER DEFAULT 0,
    report_data TEXT, -- JSON blob with detailed results
    generated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (scan_id) REFERENCES scan_results(scan_id)
);

-- Create indexes for performance
CREATE INDEX idx_policy_definitions_type ON policy_definitions(policy_type);
CREATE INDEX idx_policy_definitions_active ON policy_definitions(is_active);
CREATE INDEX idx_policy_evaluations_policy_id ON policy_evaluations(policy_id);
CREATE INDEX idx_policy_evaluations_scan_id ON policy_evaluations(scan_id);
CREATE INDEX idx_policy_evaluations_result ON policy_evaluations(evaluation_result);
CREATE INDEX idx_compliance_reports_scan_id ON compliance_reports(scan_id);
CREATE INDEX idx_compliance_reports_framework ON compliance_reports(compliance_framework);

-- +migrate Down
DROP INDEX IF EXISTS idx_compliance_reports_framework;
DROP INDEX IF EXISTS idx_compliance_reports_scan_id;
DROP INDEX IF EXISTS idx_policy_evaluations_result;
DROP INDEX IF EXISTS idx_policy_evaluations_scan_id;
DROP INDEX IF EXISTS idx_policy_evaluations_policy_id;
DROP INDEX IF EXISTS idx_policy_definitions_active;
DROP INDEX IF EXISTS idx_policy_definitions_type;

DROP TABLE IF EXISTS compliance_reports;
DROP TABLE IF EXISTS policy_evaluations;
DROP TABLE IF EXISTS policy_definitions;