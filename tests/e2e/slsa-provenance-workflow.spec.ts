import { test, expect } from '@playwright/test';
import { execSync } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';

interface WorkflowRun {
  id: number;
  status: string;
  conclusion: string;
  workflow_id: number;
  head_commit: {
    id: string;
    message: string;
  };
  run_started_at: string;
  updated_at: string;
}

interface WorkflowArtifact {
  id: number;
  name: string;
  size_in_bytes: number;
  created_at: string;
  expired: boolean;
}

interface AttestationData {
  _type: string;
  subject: Array<{
    name: string;
    digest: Record<string, string>;
  }>;
  predicateType: string;
  predicate: Record<string, any>;
}

test.describe('SLSA Provenance Workflow E2E Tests', () => {
  const testTimeout = 15 * 60 * 1000; // 15 minutes for CI/CD workflows
  const repositoryOwner = process.env.GITHUB_REPOSITORY_OWNER || 'test-owner';
  const repositoryName = process.env.GITHUB_REPOSITORY_NAME || 'keystone';
  const githubToken = process.env.GITHUB_TOKEN;

  test.beforeAll(async () => {
    if (!githubToken) {
      test.skip('GitHub token not available for E2E testing');
    }
  });

  test('Should trigger security pipeline and generate SLSA attestations', async ({ page }) => {
    test.setTimeout(testTimeout);

    // Step 1: Navigate to GitHub repository
    await page.goto(`https://github.com/${repositoryOwner}/${repositoryName}`);

    // Step 2: Navigate to Actions tab
    await page.click('text=Actions');
    await expect(page.locator('h1')).toContainText('Actions');

    // Step 3: Find the Security Pipeline workflow
    const securityPipelineLink = page.locator('text=Security Pipeline').first();
    await expect(securityPipelineLink).toBeVisible();
    await securityPipelineLink.click();

    // Step 4: Trigger workflow run (if manual trigger is available)
    const runWorkflowButton = page.locator('text=Run workflow');
    if (await runWorkflowButton.isVisible()) {
      await runWorkflowButton.click();
      await page.click('text=Run workflow', { timeout: 5000 });
    }

    // Step 5: Wait for workflow to start and get latest run
    await page.waitForTimeout(5000);

    const latestRun = page.locator('.Box-row').first();
    await expect(latestRun).toBeVisible();

    // Get run details
    const runTitle = await latestRun.locator('a[data-hovercard-type="pull_request"]').first().textContent();
    const runStatus = await latestRun.locator('[data-testid="workflow-run-status"]').getAttribute('aria-label');

    console.log(`Monitoring workflow run: ${runTitle}`);
    console.log(`Initial status: ${runStatus}`);

    // Step 6: Click on the workflow run to see details
    await latestRun.click();

    // Step 7: Wait for workflow completion (with periodic checks)
    let isCompleted = false;
    let attempts = 0;
    const maxAttempts = 30; // 15 minutes max (30 * 30 seconds)

    while (!isCompleted && attempts < maxAttempts) {
      await page.waitForTimeout(30000); // Wait 30 seconds between checks
      await page.reload();

      const statusElement = page.locator('[data-testid="workflow-run-status"]');
      const currentStatus = await statusElement.getAttribute('aria-label');

      console.log(`Attempt ${attempts + 1}: Status - ${currentStatus}`);

      if (currentStatus?.includes('completed') || currentStatus?.includes('success') || currentStatus?.includes('failed')) {
        isCompleted = true;

        // Verify workflow completed successfully
        expect(currentStatus).toContain('success');
      }

      attempts++;
    }

    if (!isCompleted) {
      throw new Error(`Workflow did not complete within ${testTimeout}ms`);
    }

    // Step 8: Verify all jobs completed successfully
    const jobsSection = page.locator('[data-testid="jobs-summary"]');
    await expect(jobsSection).toBeVisible();

    // Check for SLSA provenance job
    const slsaProvenanceJob = page.locator('text=slsa-provenance');
    await expect(slsaProvenanceJob).toBeVisible();

    // Verify job status
    const slsaJobStatus = await slsaProvenanceJob.locator('..').locator('[data-testid="job-status"]').getAttribute('aria-label');
    expect(slsaJobStatus).toContain('success');

    console.log('SLSA provenance job completed successfully');
  });

  test('Should verify SLSA attestation artifacts are generated', async ({ page }) => {
    test.setTimeout(testTimeout);

    // Step 1: Get latest workflow run via API
    const latestRun = await getLatestWorkflowRun(repositoryOwner, repositoryName, githubToken!);
    expect(latestRun.conclusion).toBe('success');

    // Step 2: Navigate to workflow run artifacts
    await page.goto(`https://github.com/${repositoryOwner}/${repositoryName}/actions/runs/${latestRun.id}`);

    // Step 3: Scroll to artifacts section
    await page.locator('text=Artifacts').scrollIntoViewIfNeeded();

    // Step 4: Verify SLSA attestation artifacts exist
    const expectedArtifacts = [
      'slsa-attestations',
      'keyless-signing-diagnostic',
      'trivy-vulnerability-results',
      'grype-vulnerability-results',
      'sbom-cyclonedx',
      'sbom-spdx'
    ];

    for (const artifactName of expectedArtifacts) {
      const artifactLink = page.locator(`text=${artifactName}`);
      await expect(artifactLink).toBeVisible();
      console.log(`Verified artifact: ${artifactName}`);
    }

    // Step 5: Download and verify SLSA attestations artifact
    const slsaAttestationsLink = page.locator('text=slsa-attestations');
    await slsaAttestationsLink.click();

    // Note: In real implementation, would need to handle file download
    // For E2E testing, we verify the download link works
    await expect(page.locator('text=Download')).toBeVisible();
  });

  test('Should validate SLSA attestation content and structure', async ({ page }) => {
    test.setTimeout(testTimeout);

    // Step 1: Get artifacts from latest successful run
    const latestRun = await getLatestWorkflowRun(repositoryOwner, repositoryName, githubToken!);
    const artifacts = await getWorkflowArtifacts(repositoryOwner, repositoryName, latestRun.id, githubToken!);

    const slsaArtifact = artifacts.find(a => a.name === 'slsa-attestations');
    expect(slsaArtifact).toBeDefined();
    expect(slsaArtifact!.size_in_bytes).toBeGreaterThan(0);

    // Step 2: Navigate to workflow summary page
    await page.goto(`https://github.com/${repositoryOwner}/${repositoryName}/actions/runs/${latestRun.id}`);

    // Step 3: Check workflow summary for SLSA compliance information
    await page.locator('text=SLSA Provenance Attestation Results').scrollIntoViewIfNeeded();

    // Verify overall status
    const overallStatus = page.locator('text=SUCCESS: All SLSA attestation operations completed successfully');
    await expect(overallStatus).toBeVisible();

    // Step 4: Verify component results table
    const componentResults = page.locator('text=Component Results').locator('..');
    await expect(componentResults).toBeVisible();

    // Check all components passed
    const expectedComponents = [
      'SLSA Tools Installation',
      'Build Metadata Collection',
      'SLSA Provenance Generation',
      'Multi-Predicate Attestations',
      'Attestation Signing',
      'OCI Registry Storage',
      'Attestation Verification'
    ];

    for (const component of expectedComponents) {
      const componentRow = page.locator(`text=${component}`);
      await expect(componentRow).toBeVisible();

      // Verify status is success and result is PASS
      const statusCell = componentRow.locator('..').locator('td').nth(1);
      const resultCell = componentRow.locator('..').locator('td').nth(2);

      await expect(statusCell).toContainText('success');
      await expect(resultCell).toContainText('PASS');
    }

    // Step 5: Verify attestation details
    const attestationDetails = page.locator('text=Attestation Details').locator('..');
    await expect(attestationDetails).toBeVisible();

    await expect(attestationDetails).toContainText('Target Artifact: vulnerable-demo:latest');
    await expect(attestationDetails).toContainText('Source Commit:');
    await expect(attestationDetails).toContainText('Workflow Reference:');
    await expect(attestationDetails).toContainText('Runner: Linux-X64');

    // Step 6: Verify registry references
    const registryRefs = page.locator('text=Registry References').locator('..');
    await expect(registryRefs).toBeVisible();

    await expect(registryRefs).toContainText('SLSA Provenance');
    await expect(registryRefs).toContainText('SBOM Attestation');
    await expect(registryRefs).toContainText('Vulnerability Attestation');

    // Step 7: Verify verification results
    const verificationResults = page.locator('text=Verification Results').locator('..');
    await expect(verificationResults).toBeVisible();

    const verificationItems = [
      'SLSA Signature: VERIFIED',
      'SBOM Signature: VERIFIED',
      'Vulnerability Signature: VERIFIED',
      'Source Traceability: VERIFIED',
      'Artifact Traceability: VERIFIED'
    ];

    for (const item of verificationItems) {
      await expect(verificationResults).toContainText(item);
    }

    // Step 8: Verify SLSA compliance level
    const complianceLevel = page.locator('text=SLSA Level 3: Build integrity, source provenance, and cryptographic verification achieved');
    await expect(complianceLevel).toBeVisible();

    console.log('All SLSA attestation validations passed');
  });

  test('Should verify keyless signing integration', async ({ page }) => {
    test.setTimeout(testTimeout);

    const latestRun = await getLatestWorkflowRun(repositoryOwner, repositoryName, githubToken!);

    // Navigate to keyless signing job details
    await page.goto(`https://github.com/${repositoryOwner}/${repositoryName}/actions/runs/${latestRun.id}`);

    // Click on keyless-signing job
    await page.locator('text=keyless-signing').click();

    // Verify key steps completed successfully
    const keylessSteps = [
      'Install Cosign CLI',
      'Configure and test GitHub OIDC token',
      'Test Cosign with OIDC integration',
      'Implement keyless signing for container images',
      'Verify container signature',
      'Configure Rekor transparency log integration',
      'Sign SBOM artifacts with keyless signing'
    ];

    for (const step of keylessSteps) {
      const stepElement = page.locator(`text=${step}`);
      await expect(stepElement).toBeVisible();

      // Verify step has success icon
      const stepContainer = stepElement.locator('..');
      await expect(stepContainer.locator('[data-testid="step-status"]')).toHaveAttribute('aria-label', /success/);
    }

    // Verify OIDC token validation output
    await page.locator('text=Configure and test GitHub OIDC token').click();
    const oidcOutput = page.locator('[data-testid="step-output"]');
    await expect(oidcOutput).toContainText('PASS: OIDC token configuration and validation completed');
    await expect(oidcOutput).toContainText('Identity verified:');

    // Verify container signing output
    await page.locator('text=Implement keyless signing for container images').click();
    const signingOutput = page.locator('[data-testid="step-output"]');
    await expect(signingOutput).toContainText('PASS: Container image signed successfully');
    await expect(signingOutput).toContainText('Signature created with OIDC identity:');

    console.log('Keyless signing integration verified successfully');
  });

  test('Should verify vulnerability scanning integration', async ({ page }) => {
    test.setTimeout(testTimeout);

    const latestRun = await getLatestWorkflowRun(repositoryOwner, repositoryName, githubToken!);

    await page.goto(`https://github.com/${repositoryOwner}/${repositoryName}/actions/runs/${latestRun.id}`);

    // Click on vulnerability-scanning job
    await page.locator('text=vulnerability-scanning').click();

    // Verify vulnerability scanning steps
    const scanningSteps = [
      'Run Trivy scanner',
      'Run Grype scanner',
      'Parse vulnerability counts',
      'Generate SBOM in CycloneDX format',
      'Generate SBOM in SPDX format',
      'Validate SBOM formats'
    ];

    for (const step of scanningSteps) {
      const stepElement = page.locator(`text=${step}`);
      await expect(stepElement).toBeVisible();
    }

    // Check vulnerability scan results in summary
    await page.locator('text=Parse vulnerability counts').click();
    const summaryOutput = page.locator('[data-testid="step-output"]');
    await expect(summaryOutput).toContainText('Security Pipeline Results');
    await expect(summaryOutput).toContainText('Vulnerability Scan Results');
    await expect(summaryOutput).toContainText('SBOM Generation Results');

    // Verify SBOM validation passed
    await page.locator('text=Validate SBOM formats').click();
    const validationOutput = page.locator('[data-testid="step-output"]');
    await expect(validationOutput).toContainText('PASS: All SBOM validations passed');

    console.log('Vulnerability scanning integration verified successfully');
  });

  test('Should verify performance requirements are met', async ({ page }) => {
    test.setTimeout(testTimeout);

    const latestRun = await getLatestWorkflowRun(repositoryOwner, repositoryName, githubToken!);

    // Calculate total workflow runtime
    const startTime = new Date(latestRun.run_started_at);
    const endTime = new Date(latestRun.updated_at);
    const totalRuntimeMinutes = (endTime.getTime() - startTime.getTime()) / (1000 * 60);

    // Verify total runtime is within acceptable limits (less than 10 minutes per story requirement)
    expect(totalRuntimeMinutes).toBeLessThan(10);
    console.log(`Total workflow runtime: ${totalRuntimeMinutes.toFixed(2)} minutes`);

    await page.goto(`https://github.com/${repositoryOwner}/${repositoryName}/actions/runs/${latestRun.id}`);

    // Navigate to SLSA provenance job and check timing
    await page.locator('text=slsa-provenance').click();

    // Get job timing information
    const jobTiming = page.locator('[data-testid="job-timing"]');
    if (await jobTiming.isVisible()) {
      const timingText = await jobTiming.textContent();
      console.log(`SLSA provenance job timing: ${timingText}`);
    }

    // Verify attestation generation step timing
    await page.locator('text=Generate SLSA v1.0 provenance statement').click();
    const stepOutput = page.locator('[data-testid="step-output"]');
    await expect(stepOutput).toContainText('SLSA v1.0 provenance statement generated');

    // Check signing step performance
    await page.locator('text=Sign attestations with keyless approach').click();
    const signingOutput = page.locator('[data-testid="step-output"]');
    await expect(signingOutput).toContainText('All attestations signed successfully with keyless approach');

    console.log('Performance requirements verification completed');
  });
});

// Helper functions
async function getLatestWorkflowRun(owner: string, repo: string, token: string): Promise<WorkflowRun> {
  const response = await fetch(`https://api.github.com/repos/${owner}/${repo}/actions/runs?per_page=1&status=completed`, {
    headers: {
      'Authorization': `token ${token}`,
      'Accept': 'application/vnd.github.v3+json'
    }
  });

  if (!response.ok) {
    throw new Error(`GitHub API request failed: ${response.status}`);
  }

  const data = await response.json();
  if (!data.workflow_runs || data.workflow_runs.length === 0) {
    throw new Error('No completed workflow runs found');
  }

  return data.workflow_runs[0];
}

async function getWorkflowArtifacts(owner: string, repo: string, runId: number, token: string): Promise<WorkflowArtifact[]> {
  const response = await fetch(`https://api.github.com/repos/${owner}/${repo}/actions/runs/${runId}/artifacts`, {
    headers: {
      'Authorization': `token ${token}`,
      'Accept': 'application/vnd.github.v3+json'
    }
  });

  if (!response.ok) {
    throw new Error(`GitHub API request failed: ${response.status}`);
  }

  const data = await response.json();
  return data.artifacts || [];
}

async function downloadAndVerifyAttestation(artifactUrl: string, token: string): Promise<AttestationData> {
  // In a real implementation, this would download and extract the artifact
  // For testing purposes, we return a mock structure
  return {
    _type: 'https://in-toto.io/Statement/v1',
    subject: [
      {
        name: 'vulnerable-demo:latest',
        digest: {
          sha256: 'mock-digest'
        }
      }
    ],
    predicateType: 'https://slsa.dev/provenance/v1',
    predicate: {
      buildDefinition: {
        buildType: 'https://github.com/Attestations/GitHubActionsWorkflow@v1'
      }
    }
  };
}