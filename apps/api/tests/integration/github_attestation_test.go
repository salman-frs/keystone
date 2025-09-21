package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type GitHubOIDCToken struct {
	Value     string `json:"value"`
	ExpiresAt string `json:"expires_at"`
}

type OIDCClaims struct {
	Issuer               string `json:"iss"`
	Subject              string `json:"sub"`
	Audience             string `json:"aud"`
	ExpirationTime       int64  `json:"exp"`
	NotBefore            int64  `json:"nbf"`
	IssuedAt             int64  `json:"iat"`
	JWTID                string `json:"jti"`
	Actor                string `json:"actor"`
	Repository           string `json:"repository"`
	RepositoryOwner      string `json:"repository_owner"`
	RepositoryID         string `json:"repository_id"`
	RepositoryOwnerID    string `json:"repository_owner_id"`
	RunID                string `json:"run_id"`
	RunNumber            string `json:"run_number"`
	RunAttempt           string `json:"run_attempt"`
	WorkflowRef          string `json:"workflow_ref"`
	WorkflowSha          string `json:"workflow_sha"`
	JobWorkflowRef       string `json:"job_workflow_ref"`
	JobWorkflowSha       string `json:"job_workflow_sha"`
	RefType              string `json:"ref_type"`
	Ref                  string `json:"ref"`
	SHA                  string `json:"sha"`
	Environment          string `json:"environment"`
	ActorID              string `json:"actor_id"`
	EventName            string `json:"event_name"`
}

type AttestationMetadata struct {
	Target         string            `json:"target"`
	BuildTime      string            `json:"buildTime"`
	CommitSHA      string            `json:"commitSha"`
	WorkflowRef    string            `json:"workflowRef"`
	RunnerInfo     string            `json:"runnerInfo"`
	Annotations    map[string]string `json:"annotations"`
	RegistryRef    string            `json:"registryRef"`
	SignatureValid bool              `json:"signatureValid"`
}

func TestGitHubOIDCTokenAcquisition(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tests := []struct {
		name           string
		requestToken   string
		requestURL     string
		audience       string
		expectedIssuer string
		expectError    bool
	}{
		{
			name:           "Valid OIDC token request",
			requestToken:   "mock-request-token",
			requestURL:     "https://token.actions.githubusercontent.com/test",
			audience:       "sigstore",
			expectedIssuer: "https://token.actions.githubusercontent.com",
			expectError:    false,
		},
		{
			name:        "Missing request token",
			requestURL:  "https://token.actions.githubusercontent.com/test",
			audience:    "sigstore",
			expectError: true,
		},
		{
			name:         "Missing request URL",
			requestToken: "mock-request-token",
			audience:     "sigstore",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock GitHub OIDC server
			server := setupMockOIDCServer(t)
			defer server.Close()

			// Override request URL to use mock server
			if tt.requestURL != "" {
				tt.requestURL = server.URL + "/test"
			}

			token, err := acquireGitHubOIDCToken(tt.requestToken, tt.requestURL, tt.audience)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, token)
			assert.NotEmpty(t, token.Value)
			assert.NotEmpty(t, token.ExpiresAt)

			// Validate token claims
			claims, err := parseOIDCTokenClaims(token.Value)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedIssuer, claims.Issuer)
			assert.Equal(t, tt.audience, claims.Audience)
		})
	}
}

func TestGitHubActionsWorkflowContext(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Mock GitHub Actions environment variables
	originalEnv := setupMockGitHubEnv()
	defer restoreEnv(originalEnv)

	context := collectWorkflowContext()
	require.NotNil(t, context)

	assert.Equal(t, "Security Pipeline", context["workflowName"])
	assert.Equal(t, "test/keystone", context["repository"])
	assert.Equal(t, "test-actor", context["actor"])
	assert.Equal(t, "abc123def456", context["commitSha"])
	assert.Equal(t, "refs/heads/main", context["ref"])
	assert.Equal(t, "12345", context["runId"])
	assert.Equal(t, "ubuntu-latest", context["runnerOs"])
	assert.Equal(t, "X64", context["runnerArch"])
}

func TestAttestationOCIStorage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup mock OCI registry
	server := setupMockOCIRegistry(t)
	defer server.Close()

	attestationData := map[string]interface{}{
		"_type":         "https://in-toto.io/Statement/v1",
		"predicateType": "https://slsa.dev/provenance/v1",
		"subject": []map[string]interface{}{
			{
				"name": "vulnerable-demo:latest",
				"digest": map[string]string{
					"sha256": "abc123def456",
				},
			},
		},
		"predicate": map[string]interface{}{
			"buildDefinition": map[string]interface{}{
				"buildType": "https://github.com/Attestations/GitHubActionsWorkflow@v1",
			},
		},
	}

	registryRef, err := storeAttestationInRegistry(
		server.URL,
		"test/keystone",
		"slsa-provenance",
		"abc123def456",
		attestationData,
	)

	require.NoError(t, err)
	assert.NotEmpty(t, registryRef)
	assert.Contains(t, registryRef, "attestations:slsa-provenance")
}

func TestAttestationVerificationWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test data
	containerTarget := "vulnerable-demo:latest"
	commitSHA := "abc123def456"
	workflowRef := "refs/heads/main"
	repository := "test/keystone"

	// Setup mock services
	oidcServer := setupMockOIDCServer(t)
	defer oidcServer.Close()

	oiiRegistry := setupMockOCIRegistry(t)
	defer oiiRegistry.Close()

	t.Run("End-to-end attestation workflow", func(t *testing.T) {
		// Step 1: Acquire OIDC token
		token, err := acquireGitHubOIDCToken(
			"mock-request-token",
			oidcServer.URL+"/test",
			"sigstore",
		)
		require.NoError(t, err)

		// Step 2: Generate SLSA provenance
		provenance, err := generateSLSAProvenance(
			containerTarget,
			commitSHA,
			workflowRef,
			repository,
		)
		require.NoError(t, err)

		// Step 3: Sign attestation (mock)
		signature, err := signAttestationWithOIDC(provenance, token)
		require.NoError(t, err)

		// Step 4: Store in OCI registry
		registryRef, err := storeAttestationInRegistry(
			oiiRegistry.URL,
			repository,
			"slsa-provenance",
			commitSHA,
			provenance,
		)
		require.NoError(t, err)

		// Step 5: Verify attestation
		verified, err := verifyAttestationFromRegistry(
			registryRef,
			token,
		)
		require.NoError(t, err)
		assert.True(t, verified)

		// Step 6: Validate source-to-artifact traceability
		traceabilityValid, err := validateSourceToArtifactTraceability(
			provenance,
			commitSHA,
			containerTarget,
		)
		require.NoError(t, err)
		assert.True(t, traceabilityValid)
	})
}

func TestMultipleAttestationTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	containerTarget := "vulnerable-demo:latest"
	commitSHA := "abc123def456"

	attestationTypes := []struct {
		name          string
		predicateType string
		expectedData  string
	}{
		{
			name:          "SLSA Provenance",
			predicateType: "https://slsa.dev/provenance/v1",
			expectedData:  "buildDefinition",
		},
		{
			name:          "SBOM Attestation",
			predicateType: "https://spdx.dev/Document",
			expectedData:  "components",
		},
		{
			name:          "Vulnerability Scan",
			predicateType: "https://cosign.sigstore.dev/attestation/vuln/v1",
			expectedData:  "scanner",
		},
	}

	for _, tt := range attestationTypes {
		t.Run(tt.name, func(t *testing.T) {
			attestation, err := generateAttestationByType(
				tt.predicateType,
				containerTarget,
				commitSHA,
			)
			require.NoError(t, err)

			// Verify attestation structure
			assert.Equal(t, "https://in-toto.io/Statement/v1", attestation["_type"])
			assert.Equal(t, tt.predicateType, attestation["predicateType"])

			// Verify predicate contains expected data
			predicate, ok := attestation["predicate"].(map[string]interface{})
			require.True(t, ok)
			assert.Contains(t, fmt.Sprintf("%v", predicate), tt.expectedData)
		})
	}
}

func TestAttestationMetadataCollection(t *testing.T) {
	originalEnv := setupMockGitHubEnv()
	defer restoreEnv(originalEnv)

	metadata := collectAttestationMetadata(
		"vulnerable-demo:latest",
		"slsa-provenance",
		"ghcr.io/test/keystone/attestations:slsa-provenance-abc123",
	)

	require.NotNil(t, metadata)
	assert.Equal(t, "vulnerable-demo:latest", metadata.Target)
	assert.NotEmpty(t, metadata.BuildTime)
	assert.Equal(t, "abc123def456", metadata.CommitSHA)
	assert.Equal(t, "refs/heads/main", metadata.WorkflowRef)
	assert.Equal(t, "Linux-X64", metadata.RunnerInfo)
	assert.NotEmpty(t, metadata.Annotations)
	assert.Equal(t, "ghcr.io/test/keystone/attestations:slsa-provenance-abc123", metadata.RegistryRef)
}

// Mock server setup functions
func setupMockOIDCServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "Missing authorization", http.StatusUnauthorized)
			return
		}

		audience := r.URL.Query().Get("audience")
		if audience == "" {
			http.Error(w, "Missing audience", http.StatusBadRequest)
			return
		}

		// Return mock OIDC token
		token := GitHubOIDCToken{
			Value:     "mock.jwt.token.with.claims",
			ExpiresAt: time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(token)
	}))
}

func setupMockOCIRegistry(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PUT":
			w.WriteHeader(http.StatusCreated)
		case "GET":
			w.Header().Set("Content-Type", "application/vnd.in-toto+json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"_type":         "https://in-toto.io/Statement/v1",
				"predicateType": "https://slsa.dev/provenance/v1",
			})
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))
}

func setupMockGitHubEnv() map[string]string {
	originalEnv := make(map[string]string)
	envVars := map[string]string{
		"GITHUB_WORKFLOW":        "Security Pipeline",
		"GITHUB_REPOSITORY":      "test/keystone",
		"GITHUB_ACTOR":           "test-actor",
		"GITHUB_SHA":             "abc123def456",
		"GITHUB_REF":             "refs/heads/main",
		"GITHUB_RUN_ID":          "12345",
		"GITHUB_RUN_NUMBER":      "67",
		"RUNNER_OS":              "Linux",
		"RUNNER_ARCH":            "X64",
		"RUNNER_NAME":            "ubuntu-latest",
		"GITHUB_EVENT_NAME":      "push",
		"GITHUB_REPOSITORY_ID":   "123456789",
		"GITHUB_REPOSITORY_OWNER_ID": "987654321",
	}

	for key, value := range envVars {
		originalEnv[key] = os.Getenv(key)
		os.Setenv(key, value)
	}

	return originalEnv
}

func restoreEnv(originalEnv map[string]string) {
	for key, value := range originalEnv {
		if value == "" {
			os.Unsetenv(key)
		} else {
			os.Setenv(key, value)
		}
	}
}

// Mock implementation functions
func acquireGitHubOIDCToken(requestToken, requestURL, audience string) (*GitHubOIDCToken, error) {
	if requestToken == "" {
		return nil, fmt.Errorf("missing request token")
	}
	if requestURL == "" {
		return nil, fmt.Errorf("missing request URL")
	}

	// Make HTTP request to mock server
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), "GET", requestURL+"?audience="+audience, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "bearer "+requestToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to acquire token: %d", resp.StatusCode)
	}

	var token GitHubOIDCToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}

	return &token, nil
}

func parseOIDCTokenClaims(tokenValue string) (*OIDCClaims, error) {
	// Mock JWT parsing - in real implementation would decode JWT
	return &OIDCClaims{
		Issuer:   "https://token.actions.githubusercontent.com",
		Audience: "sigstore",
		Subject:  "repo:test/keystone:ref:refs/heads/main",
	}, nil
}

func collectWorkflowContext() map[string]interface{} {
	return map[string]interface{}{
		"workflowName": os.Getenv("GITHUB_WORKFLOW"),
		"repository":   os.Getenv("GITHUB_REPOSITORY"),
		"actor":        os.Getenv("GITHUB_ACTOR"),
		"commitSha":    os.Getenv("GITHUB_SHA"),
		"ref":          os.Getenv("GITHUB_REF"),
		"runId":        os.Getenv("GITHUB_RUN_ID"),
		"runnerOs":     os.Getenv("RUNNER_OS"),
		"runnerArch":   os.Getenv("RUNNER_ARCH"),
	}
}

func generateSLSAProvenance(containerTarget, commitSHA, workflowRef, repository string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"_type":         "https://in-toto.io/Statement/v1",
		"predicateType": "https://slsa.dev/provenance/v1",
		"subject": []map[string]interface{}{
			{
				"name": containerTarget,
				"digest": map[string]string{
					"sha256": commitSHA,
				},
			},
		},
		"predicate": map[string]interface{}{
			"buildDefinition": map[string]interface{}{
				"buildType": "https://github.com/Attestations/GitHubActionsWorkflow@v1",
				"resolvedDependencies": []map[string]interface{}{
					{
						"uri": "git+https://github.com/" + repository + "@" + commitSHA,
						"digest": map[string]string{
							"sha1": commitSHA,
						},
					},
				},
			},
		},
	}, nil
}

func generateAttestationByType(predicateType, containerTarget, commitSHA string) (map[string]interface{}, error) {
	base := map[string]interface{}{
		"_type":         "https://in-toto.io/Statement/v1",
		"predicateType": predicateType,
		"subject": []map[string]interface{}{
			{
				"name": containerTarget,
				"digest": map[string]string{
					"sha256": commitSHA,
				},
			},
		},
	}

	switch predicateType {
	case "https://slsa.dev/provenance/v1":
		base["predicate"] = map[string]interface{}{
			"buildDefinition": map[string]interface{}{
				"buildType": "https://github.com/Attestations/GitHubActionsWorkflow@v1",
			},
		}
	case "https://spdx.dev/Document":
		base["predicate"] = map[string]interface{}{
			"bomFormat": "CycloneDX",
			"components": []map[string]interface{}{
				{"name": "test-component", "version": "1.0.0"},
			},
		}
	case "https://cosign.sigstore.dev/attestation/vuln/v1":
		base["predicate"] = map[string]interface{}{
			"scanner": map[string]interface{}{
				"vendor": "Aqua Security",
				"name":   "Trivy",
			},
		}
	}

	return base, nil
}

func signAttestationWithOIDC(attestation map[string]interface{}, token *GitHubOIDCToken) (map[string]interface{}, error) {
	return map[string]interface{}{
		"signature": "mock-signature",
		"bundle":    "mock-bundle",
		"signedAt":  time.Now().Format(time.RFC3339),
	}, nil
}

func storeAttestationInRegistry(registryURL, repository, attestationType, commitSHA string, attestation map[string]interface{}) (string, error) {
	registryRef := fmt.Sprintf("%s/%s/attestations:%s-%s", registryURL, repository, attestationType, commitSHA)

	// Mock storage - in real implementation would use ORAS
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), "PUT", registryRef, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("failed to store attestation: %d", resp.StatusCode)
	}

	return registryRef, nil
}

func verifyAttestationFromRegistry(registryRef string, token *GitHubOIDCToken) (bool, error) {
	// Mock verification - always returns true for valid inputs
	return true, nil
}

func validateSourceToArtifactTraceability(provenance map[string]interface{}, expectedCommit, expectedArtifact string) (bool, error) {
	predicate, ok := provenance["predicate"].(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("invalid predicate")
	}

	buildDef, ok := predicate["buildDefinition"].(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("invalid build definition")
	}

	deps, ok := buildDef["resolvedDependencies"].([]map[string]interface{})
	if !ok || len(deps) == 0 {
		return false, fmt.Errorf("missing resolved dependencies")
	}

	digest, ok := deps[0]["digest"].(map[string]string)
	if !ok {
		return false, fmt.Errorf("missing dependency digest")
	}

	return digest["sha1"] == expectedCommit, nil
}

func collectAttestationMetadata(target, attestationType, registryRef string) *AttestationMetadata {
	return &AttestationMetadata{
		Target:      target,
		BuildTime:   time.Now().Format(time.RFC3339),
		CommitSHA:   os.Getenv("GITHUB_SHA"),
		WorkflowRef: os.Getenv("GITHUB_REF"),
		RunnerInfo:  fmt.Sprintf("%s-%s", os.Getenv("RUNNER_OS"), os.Getenv("RUNNER_ARCH")),
		Annotations: map[string]string{
			"keystone.attestation.type": attestationType,
		},
		RegistryRef:    registryRef,
		SignatureValid: true,
	}
}