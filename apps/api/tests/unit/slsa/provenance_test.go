package slsa

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type SLSAProvenance struct {
	Type          string      `json:"_type"`
	Subject       []Subject   `json:"subject"`
	PredicateType string      `json:"predicateType"`
	Predicate     Predicate   `json:"predicate"`
}

type Subject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

type Predicate struct {
	BuildDefinition BuildDefinition `json:"buildDefinition"`
	RunDetails      RunDetails      `json:"runDetails"`
}

type BuildDefinition struct {
	BuildType            string                  `json:"buildType"`
	ExternalParameters   ExternalParameters      `json:"externalParameters"`
	InternalParameters   InternalParameters      `json:"internalParameters"`
	ResolvedDependencies []ResolvedDependency    `json:"resolvedDependencies"`
}

type ExternalParameters struct {
	Workflow WorkflowParams `json:"workflow"`
}

type WorkflowParams struct {
	Ref        string `json:"ref"`
	Repository string `json:"repository"`
	Path       string `json:"path"`
}

type InternalParameters struct {
	GitHub GitHubParams `json:"github"`
}

type GitHubParams struct {
	EventName         string `json:"event_name"`
	RepositoryID      string `json:"repository_id"`
	RepositoryOwnerID string `json:"repository_owner_id"`
}

type ResolvedDependency struct {
	URI    string            `json:"uri"`
	Digest map[string]string `json:"digest"`
}

type RunDetails struct {
	Builder    Builder      `json:"builder"`
	Metadata   Metadata     `json:"metadata"`
	Byproducts []Byproduct  `json:"byproducts"`
}

type Builder struct {
	ID string `json:"id"`
}

type Metadata struct {
	InvocationID string `json:"invocationId"`
	StartedOn    string `json:"startedOn"`
	FinishedOn   string `json:"finishedOn"`
}

type Byproduct struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

func TestSLSAProvenanceGeneration(t *testing.T) {
	tests := []struct {
		name          string
		containerName string
		commitSHA     string
		workflowRef   string
		repository    string
		expectedType  string
		expectError   bool
	}{
		{
			name:          "Valid SLSA provenance generation",
			containerName: "vulnerable-demo:latest",
			commitSHA:     "abc123def456",
			workflowRef:   "refs/heads/main",
			repository:    "test/keystone",
			expectedType:  "https://in-toto.io/Statement/v1",
			expectError:   false,
		},
		{
			name:          "Empty container name should fail",
			containerName: "",
			commitSHA:     "abc123def456",
			workflowRef:   "refs/heads/main",
			repository:    "test/keystone",
			expectError:   true,
		},
		{
			name:          "Empty commit SHA should fail",
			containerName: "vulnerable-demo:latest",
			commitSHA:     "",
			workflowRef:   "refs/heads/main",
			repository:    "test/keystone",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provenance, err := generateSLSAProvenance(
				tt.containerName,
				tt.commitSHA,
				tt.workflowRef,
				tt.repository,
			)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, provenance)

			assert.Equal(t, tt.expectedType, provenance.Type)
			assert.Equal(t, "https://slsa.dev/provenance/v1", provenance.PredicateType)
			assert.Len(t, provenance.Subject, 1)
			assert.Equal(t, tt.containerName, provenance.Subject[0].Name)
			assert.NotEmpty(t, provenance.Subject[0].Digest["sha256"])

			// Verify build definition
			assert.Equal(t, "https://github.com/Attestations/GitHubActionsWorkflow@v1",
				provenance.Predicate.BuildDefinition.BuildType)
			assert.Equal(t, tt.workflowRef,
				provenance.Predicate.BuildDefinition.ExternalParameters.Workflow.Ref)
			assert.Equal(t, tt.repository,
				provenance.Predicate.BuildDefinition.ExternalParameters.Workflow.Repository)

			// Verify resolved dependencies
			assert.Len(t, provenance.Predicate.BuildDefinition.ResolvedDependencies, 1)
			dependency := provenance.Predicate.BuildDefinition.ResolvedDependencies[0]
			assert.Contains(t, dependency.URI, tt.repository)
			assert.Equal(t, tt.commitSHA, dependency.Digest["sha1"])

			// Verify run details
			assert.Equal(t, "https://github.com/actions/runner",
				provenance.Predicate.RunDetails.Builder.ID)
			assert.NotEmpty(t, provenance.Predicate.RunDetails.Metadata.InvocationID)
		})
	}
}

func TestSLSAProvenanceValidation(t *testing.T) {
	validProvenance := &SLSAProvenance{
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
		Predicate: Predicate{
			BuildDefinition: BuildDefinition{
				BuildType: "https://github.com/Attestations/GitHubActionsWorkflow@v1",
				ExternalParameters: ExternalParameters{
					Workflow: WorkflowParams{
						Ref:        "refs/heads/main",
						Repository: "test/keystone",
						Path:       ".github/workflows/security-pipeline.yaml",
					},
				},
				ResolvedDependencies: []ResolvedDependency{
					{
						URI: "git+https://github.com/test/keystone@abc123",
						Digest: map[string]string{
							"sha1": "abc123def456",
						},
					},
				},
			},
			RunDetails: RunDetails{
				Builder: Builder{
					ID: "https://github.com/actions/runner",
				},
				Metadata: Metadata{
					InvocationID: "12345",
					StartedOn:    time.Now().UTC().Format(time.RFC3339),
					FinishedOn:   time.Now().UTC().Format(time.RFC3339),
				},
			},
		},
	}

	t.Run("Valid provenance passes validation", func(t *testing.T) {
		err := validateSLSAProvenance(validProvenance)
		assert.NoError(t, err)
	})

	t.Run("Invalid statement type fails validation", func(t *testing.T) {
		invalidProvenance := *validProvenance
		invalidProvenance.Type = "invalid-type"

		err := validateSLSAProvenance(&invalidProvenance)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid statement type")
	})

	t.Run("Invalid predicate type fails validation", func(t *testing.T) {
		invalidProvenance := *validProvenance
		invalidProvenance.PredicateType = "invalid-predicate"

		err := validateSLSAProvenance(&invalidProvenance)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid predicate type")
	})

	t.Run("Empty subject fails validation", func(t *testing.T) {
		invalidProvenance := *validProvenance
		invalidProvenance.Subject = []Subject{}

		err := validateSLSAProvenance(&invalidProvenance)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "subject is required")
	})

	t.Run("Invalid build type fails validation", func(t *testing.T) {
		invalidProvenance := *validProvenance
		invalidProvenance.Predicate.BuildDefinition.BuildType = "invalid-build-type"

		err := validateSLSAProvenance(&invalidProvenance)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid build type")
	})
}

func TestSLSAProvenanceJSONSerialization(t *testing.T) {
	provenance, err := generateSLSAProvenance(
		"vulnerable-demo:latest",
		"abc123def456",
		"refs/heads/main",
		"test/keystone",
	)
	require.NoError(t, err)

	// Test JSON marshaling
	jsonBytes, err := json.Marshal(provenance)
	require.NoError(t, err)
	assert.NotEmpty(t, jsonBytes)

	// Test JSON unmarshaling
	var unmarshaledProvenance SLSAProvenance
	err = json.Unmarshal(jsonBytes, &unmarshaledProvenance)
	require.NoError(t, err)

	// Verify structure is preserved
	assert.Equal(t, provenance.Type, unmarshaledProvenance.Type)
	assert.Equal(t, provenance.PredicateType, unmarshaledProvenance.PredicateType)
	assert.Equal(t, len(provenance.Subject), len(unmarshaledProvenance.Subject))
	assert.Equal(t, provenance.Subject[0].Name, unmarshaledProvenance.Subject[0].Name)
}

func TestBuildMetadataCollection(t *testing.T) {
	metadata := collectBuildMetadata(
		"test-runner",
		"linux",
		"amd64",
		"workflow-123",
		"test/keystone",
		"actor",
		"go1.21",
		"node18",
		"docker24",
	)

	assert.NotNil(t, metadata)
	assert.Equal(t, "test-runner", metadata["runner"].(map[string]interface{})["name"])
	assert.Equal(t, "linux", metadata["runner"].(map[string]interface{})["os"])
	assert.Equal(t, "amd64", metadata["runner"].(map[string]interface{})["arch"])
	assert.Equal(t, "go1.21", metadata["buildTools"].(map[string]interface{})["go"])
	assert.Equal(t, "node18", metadata["buildTools"].(map[string]interface{})["node"])
	assert.Equal(t, "docker24", metadata["buildTools"].(map[string]interface{})["docker"])
}

// Mock functions for testing
func generateSLSAProvenance(containerName, commitSHA, workflowRef, repository string) (*SLSAProvenance, error) {
	if containerName == "" {
		return nil, assert.AnError
	}
	if commitSHA == "" {
		return nil, assert.AnError
	}

	return &SLSAProvenance{
		Type: "https://in-toto.io/Statement/v1",
		Subject: []Subject{
			{
				Name: containerName,
				Digest: map[string]string{
					"sha256": "mock-digest-" + commitSHA,
				},
			},
		},
		PredicateType: "https://slsa.dev/provenance/v1",
		Predicate: Predicate{
			BuildDefinition: BuildDefinition{
				BuildType: "https://github.com/Attestations/GitHubActionsWorkflow@v1",
				ExternalParameters: ExternalParameters{
					Workflow: WorkflowParams{
						Ref:        workflowRef,
						Repository: repository,
						Path:       ".github/workflows/security-pipeline.yaml",
					},
				},
				ResolvedDependencies: []ResolvedDependency{
					{
						URI: "git+https://github.com/" + repository + "@" + commitSHA,
						Digest: map[string]string{
							"sha1": commitSHA,
						},
					},
				},
			},
			RunDetails: RunDetails{
				Builder: Builder{
					ID: "https://github.com/actions/runner",
				},
				Metadata: Metadata{
					InvocationID: "mock-invocation-id",
					StartedOn:    time.Now().UTC().Format(time.RFC3339),
					FinishedOn:   time.Now().UTC().Format(time.RFC3339),
				},
			},
		},
	}, nil
}

func validateSLSAProvenance(provenance *SLSAProvenance) error {
	if provenance.Type != "https://in-toto.io/Statement/v1" {
		return assert.AnError
	}
	if provenance.PredicateType != "https://slsa.dev/provenance/v1" {
		return assert.AnError
	}
	if len(provenance.Subject) == 0 {
		return assert.AnError
	}
	if provenance.Predicate.BuildDefinition.BuildType != "https://github.com/Attestations/GitHubActionsWorkflow@v1" {
		return assert.AnError
	}
	return nil
}

func collectBuildMetadata(runnerName, os, arch, workflowRef, repository, actor, goVersion, nodeVersion, dockerVersion string) map[string]interface{} {
	return map[string]interface{}{
		"buildTime": time.Now().UTC().Format(time.RFC3339),
		"runner": map[string]interface{}{
			"name": runnerName,
			"os":   os,
			"arch": arch,
		},
		"workflow": map[string]interface{}{
			"ref": workflowRef,
		},
		"source": map[string]interface{}{
			"repository": repository,
			"actor":      actor,
		},
		"buildTools": map[string]interface{}{
			"go":     goVersion,
			"node":   nodeVersion,
			"docker": dockerVersion,
		},
	}
}