package attestation

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type AttestationStatement struct {
	Type          string      `json:"_type"`
	Subject       []Subject   `json:"subject"`
	PredicateType string      `json:"predicateType"`
	Predicate     interface{} `json:"predicate"`
}

type Subject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

type SBOMPredicate struct {
	BOMFormat   string      `json:"bomFormat"`
	SpecVersion string      `json:"specVersion"`
	Version     int         `json:"version"`
	Components  []Component `json:"components"`
}

type Component struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Purl    string `json:"purl,omitempty"`
}

type VulnerabilityPredicate struct {
	Invocation Invocation `json:"invocation"`
	Scanner    Scanner    `json:"scanner"`
	Result     VulnResult `json:"result"`
}

type Invocation struct {
	URI        string `json:"uri"`
	ProducerID string `json:"producer_id"`
}

type Scanner struct {
	Vendor  string `json:"vendor"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

type VulnResult struct {
	Results []VulnScanResult `json:"Results,omitempty"`
}

type VulnScanResult struct {
	Target          string          `json:"Target"`
	Class           string          `json:"Class"`
	Type            string          `json:"Type"`
	Vulnerabilities []Vulnerability `json:"Vulnerabilities,omitempty"`
}

type Vulnerability struct {
	VulnerabilityID  string   `json:"VulnerabilityID"`
	PkgName          string   `json:"PkgName"`
	InstalledVersion string   `json:"InstalledVersion"`
	FixedVersion     string   `json:"FixedVersion"`
	Severity         string   `json:"Severity"`
	References       []string `json:"References,omitempty"`
}

func TestMultiPredicateAttestationGeneration(t *testing.T) {
	containerTarget := "vulnerable-demo:latest"
	artifactDigest := "sha256:abc123def456"

	tests := []struct {
		name          string
		predicateType string
		expectError   bool
	}{
		{
			name:          "SLSA provenance attestation",
			predicateType: "https://slsa.dev/provenance/v1",
			expectError:   false,
		},
		{
			name:          "SBOM attestation",
			predicateType: "https://spdx.dev/Document",
			expectError:   false,
		},
		{
			name:          "Vulnerability scan attestation",
			predicateType: "https://cosign.sigstore.dev/attestation/vuln/v1",
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attestation, err := generateAttestationByType(
				tt.predicateType,
				containerTarget,
				artifactDigest,
			)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, attestation)

			assert.Equal(t, "https://in-toto.io/Statement/v1", attestation.Type)
			assert.Equal(t, tt.predicateType, attestation.PredicateType)
			assert.Len(t, attestation.Subject, 1)
			assert.Equal(t, containerTarget, attestation.Subject[0].Name)
			assert.Equal(t, artifactDigest, attestation.Subject[0].Digest["sha256"])
			assert.NotNil(t, attestation.Predicate)
		})
	}
}

func TestSBOMAttestationGeneration(t *testing.T) {
	containerTarget := "vulnerable-demo:latest"
	artifactDigest := "sha256:abc123def456"

	attestation, err := generateSBOMAttestation(containerTarget, artifactDigest)
	require.NoError(t, err)
	require.NotNil(t, attestation)

	assert.Equal(t, "https://in-toto.io/Statement/v1", attestation.Type)
	assert.Equal(t, "https://spdx.dev/Document", attestation.PredicateType)

	// Verify predicate is SBOM format
	predicateBytes, err := json.Marshal(attestation.Predicate)
	require.NoError(t, err)

	var sbomPredicate SBOMPredicate
	err = json.Unmarshal(predicateBytes, &sbomPredicate)
	require.NoError(t, err)

	assert.Equal(t, "CycloneDX", sbomPredicate.BOMFormat)
	assert.NotEmpty(t, sbomPredicate.SpecVersion)
	assert.Greater(t, sbomPredicate.Version, 0)
	assert.NotEmpty(t, sbomPredicate.Components)

	// Verify components have required fields
	for _, component := range sbomPredicate.Components {
		assert.NotEmpty(t, component.Type)
		assert.NotEmpty(t, component.Name)
		assert.NotEmpty(t, component.Version)
	}
}

func TestVulnerabilityAttestationGeneration(t *testing.T) {
	containerTarget := "vulnerable-demo:latest"
	artifactDigest := "sha256:abc123def456"

	attestation, err := generateVulnerabilityAttestation(containerTarget, artifactDigest)
	require.NoError(t, err)
	require.NotNil(t, attestation)

	assert.Equal(t, "https://in-toto.io/Statement/v1", attestation.Type)
	assert.Equal(t, "https://cosign.sigstore.dev/attestation/vuln/v1", attestation.PredicateType)

	// Verify predicate is vulnerability scan format
	predicateBytes, err := json.Marshal(attestation.Predicate)
	require.NoError(t, err)

	var vulnPredicate VulnerabilityPredicate
	err = json.Unmarshal(predicateBytes, &vulnPredicate)
	require.NoError(t, err)

	assert.NotEmpty(t, vulnPredicate.Invocation.URI)
	assert.NotEmpty(t, vulnPredicate.Invocation.ProducerID)
	assert.Equal(t, "Aqua Security", vulnPredicate.Scanner.Vendor)
	assert.Equal(t, "Trivy", vulnPredicate.Scanner.Name)
	assert.NotEmpty(t, vulnPredicate.Scanner.Version)
}

func TestAttestationValidation(t *testing.T) {
	validAttestation := &AttestationStatement{
		Type: "https://in-toto.io/Statement/v1",
		Subject: []Subject{
			{
				Name: "vulnerable-demo:latest",
				Digest: map[string]string{
					"sha256": "abc123def456",
				},
			},
		},
		PredicateType: "https://slsa.dev/provenance/v1",
		Predicate:     map[string]interface{}{"valid": "predicate"},
	}

	t.Run("Valid attestation passes validation", func(t *testing.T) {
		err := validateAttestation(validAttestation)
		assert.NoError(t, err)
	})

	t.Run("Invalid statement type fails validation", func(t *testing.T) {
		invalidAttestation := *validAttestation
		invalidAttestation.Type = "invalid-type"

		err := validateAttestation(&invalidAttestation)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid statement type")
	})

	t.Run("Empty subject fails validation", func(t *testing.T) {
		invalidAttestation := *validAttestation
		invalidAttestation.Subject = []Subject{}

		err := validateAttestation(&invalidAttestation)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "subject is required")
	})

	t.Run("Empty predicate type fails validation", func(t *testing.T) {
		invalidAttestation := *validAttestation
		invalidAttestation.PredicateType = ""

		err := validateAttestation(&invalidAttestation)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "predicate type is required")
	})

	t.Run("Nil predicate fails validation", func(t *testing.T) {
		invalidAttestation := *validAttestation
		invalidAttestation.Predicate = nil

		err := validateAttestation(&invalidAttestation)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "predicate is required")
	})
}

func TestAttestationSigning(t *testing.T) {
	attestation := &AttestationStatement{
		Type: "https://in-toto.io/Statement/v1",
		Subject: []Subject{
			{
				Name: "vulnerable-demo:latest",
				Digest: map[string]string{
					"sha256": "abc123def456",
				},
			},
		},
		PredicateType: "https://slsa.dev/provenance/v1",
		Predicate:     map[string]interface{}{"test": "predicate"},
	}

	signature, err := signAttestation(attestation, "keyless")
	require.NoError(t, err)
	require.NotNil(t, signature)

	assert.NotEmpty(t, signature.Signature)
	assert.NotEmpty(t, signature.Bundle)
	assert.Equal(t, "keyless", signature.SigningMethod)
	assert.NotEmpty(t, signature.SignedAt)
}

func TestAttestationVerification(t *testing.T) {
	attestation := &AttestationStatement{
		Type: "https://in-toto.io/Statement/v1",
		Subject: []Subject{
			{
				Name: "vulnerable-demo:latest",
				Digest: map[string]string{
					"sha256": "abc123def456",
				},
			},
		},
		PredicateType: "https://slsa.dev/provenance/v1",
		Predicate:     map[string]interface{}{"test": "predicate"},
	}

	signature, err := signAttestation(attestation, "keyless")
	require.NoError(t, err)

	verified, err := verifyAttestationSignature(attestation, signature, "test-identity", "test-issuer")
	require.NoError(t, err)
	assert.True(t, verified)
}

// Mock functions for testing
func generateAttestationByType(predicateType, containerTarget, artifactDigest string) (*AttestationStatement, error) {
	baseAttestation := &AttestationStatement{
		Type: "https://in-toto.io/Statement/v1",
		Subject: []Subject{
			{
				Name: containerTarget,
				Digest: map[string]string{
					"sha256": artifactDigest,
				},
			},
		},
		PredicateType: predicateType,
	}

	switch predicateType {
	case "https://slsa.dev/provenance/v1":
		baseAttestation.Predicate = map[string]interface{}{
			"buildDefinition": map[string]interface{}{
				"buildType": "https://github.com/Attestations/GitHubActionsWorkflow@v1",
			},
		}
	case "https://spdx.dev/Document":
		baseAttestation.Predicate = SBOMPredicate{
			BOMFormat:   "CycloneDX",
			SpecVersion: "1.6",
			Version:     1,
			Components: []Component{
				{
					Type:    "library",
					Name:    "test-component",
					Version: "1.0.0",
					Purl:    "pkg:golang/test-component@1.0.0",
				},
			},
		}
	case "https://cosign.sigstore.dev/attestation/vuln/v1":
		baseAttestation.Predicate = VulnerabilityPredicate{
			Invocation: Invocation{
				URI:        "https://github.com/aquasecurity/trivy",
				ProducerID: "trivy-scanner",
			},
			Scanner: Scanner{
				Vendor:  "Aqua Security",
				Name:    "Trivy",
				Version: "latest",
			},
			Result: VulnResult{
				Results: []VulnScanResult{
					{
						Target: containerTarget,
						Class:  "os-pkgs",
						Type:   "debian",
						Vulnerabilities: []Vulnerability{
							{
								VulnerabilityID:  "CVE-2023-1234",
								PkgName:          "test-package",
								InstalledVersion: "1.0.0",
								FixedVersion:     "1.0.1",
								Severity:         "HIGH",
								References:       []string{"https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2023-1234"},
							},
						},
					},
				},
			},
		}
	default:
		baseAttestation.Predicate = map[string]interface{}{"mock": "predicate"}
	}

	return baseAttestation, nil
}

func generateSBOMAttestation(containerTarget, artifactDigest string) (*AttestationStatement, error) {
	return generateAttestationByType("https://spdx.dev/Document", containerTarget, artifactDigest)
}

func generateVulnerabilityAttestation(containerTarget, artifactDigest string) (*AttestationStatement, error) {
	return generateAttestationByType("https://cosign.sigstore.dev/attestation/vuln/v1", containerTarget, artifactDigest)
}

func validateAttestation(attestation *AttestationStatement) error {
	if attestation.Type != "https://in-toto.io/Statement/v1" {
		return assert.AnError
	}
	if len(attestation.Subject) == 0 {
		return assert.AnError
	}
	if attestation.PredicateType == "" {
		return assert.AnError
	}
	if attestation.Predicate == nil {
		return assert.AnError
	}
	return nil
}

type SignatureResult struct {
	Signature     string `json:"signature"`
	Bundle        string `json:"bundle"`
	SigningMethod string `json:"signingMethod"`
	SignedAt      string `json:"signedAt"`
}

func signAttestation(attestation *AttestationStatement, method string) (*SignatureResult, error) {
	return &SignatureResult{
		Signature:     "mock-signature-data",
		Bundle:        "mock-bundle-data",
		SigningMethod: method,
		SignedAt:      "2024-01-01T00:00:00Z",
	}, nil
}

func verifyAttestationSignature(attestation *AttestationStatement, signature *SignatureResult, identity, issuer string) (bool, error) {
	// Mock verification - always returns true for valid inputs
	if signature.Signature == "" || signature.Bundle == "" {
		return false, assert.AnError
	}
	return true, nil
}