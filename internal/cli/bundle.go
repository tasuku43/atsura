package cli

import (
	"context"
	"encoding/json"

	"github.com/tasuku43/atsura/internal/domain/bundletrust"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/infra/policyyaml"
)

const maxBundleOutputBytes = 2 * 1024 * 1024

type policyValidationDocument struct {
	SchemaVersion int                     `json:"schema_version"`
	Validation    policyValidationPayload `json:"validation"`
}

type policyValidationPayload struct {
	Valid         bool                   `json:"valid"`
	CatalogDigest string                 `json:"catalog_digest"`
	PolicyDigest  string                 `json:"policy_digest"`
	RuleCount     int                    `json:"rule_count"`
	VisibleCount  int                    `json:"visible_count"`
	Policy        tailoringbundle.Policy `json:"policy"`
}

type bundleBuildDocument struct {
	SchemaVersion int                `json:"schema_version"`
	Build         bundleBuildPayload `json:"build"`
}

type bundleBuildPayload struct {
	BundleDigest string                 `json:"bundle_digest"`
	Bundle       tailoringbundle.Bundle `json:"bundle"`
}

type bundleStatusDocument struct {
	SchemaVersion int                 `json:"schema_version"`
	Status        bundleStatusPayload `json:"status"`
}

type bundleStatusPayload struct {
	BundleDigest          string                  `json:"bundle_digest"`
	CatalogDigest         string                  `json:"catalog_digest"`
	PolicyDigest          string                  `json:"policy_digest"`
	Trust                 bundletrust.State       `json:"trust"`
	Source                bundletrust.SourceState `json:"source"`
	Executable            bool                    `json:"executable"`
	SourcePath            string                  `json:"source_path"`
	SourceSHA256          string                  `json:"source_sha256"`
	SourceVersion         string                  `json:"source_version"`
	SourceProcessAttempts int                     `json:"source_process_attempts"`
}

type bundleTrustDocument struct {
	SchemaVersion int                `json:"schema_version"`
	Trust         bundleTrustPayload `json:"trust"`
}

type bundleTrustPayload struct {
	BundleDigest          string                  `json:"bundle_digest"`
	Trusted               bool                    `json:"trusted"`
	AlreadyTrusted        bool                    `json:"already_trusted"`
	Source                bundletrust.SourceState `json:"source"`
	SourceProcessAttempts int                     `json:"source_process_attempts"`
}

func runPolicyValidate(ctx context.Context, c *CLI, _ CommandSpec, intent operation.Intent, inputs ParsedInputs) int {
	result, err := c.bundles.ValidatePolicy(ctx, intent, inputs.One("--catalog"), inputs.One("--policy"))
	if err != nil {
		return c.fail(ctx, err)
	}
	document := policyValidationDocument{SchemaVersion: 1, Validation: policyValidationPayload{
		Valid: true, CatalogDigest: result.Policy.CatalogDigest, PolicyDigest: result.PolicyDigest,
		RuleCount: result.RuleCount, VisibleCount: result.VisibleCount, Policy: result.Policy,
	}}
	return c.emitJSONDocument(ctx, document, "policy validate")
}

func runPolicyInit(ctx context.Context, c *CLI, _ CommandSpec, intent operation.Intent, inputs ParsedInputs) int {
	var effect operation.Effect
	if err := effect.UnmarshalText([]byte(inputs.One("--effect"))); err != nil {
		return c.fail(ctx, fault.Wrap(fault.KindInvalidInput, "invalid_policy_effect", "The draft effect must be read, create, or write.", false, err))
	}
	policy, err := c.drafts.Init(ctx, intent, inputs.One("--catalog"), effect, inputs.Values("command"))
	if err != nil {
		return c.fail(ctx, err)
	}
	encoded, err := policyyaml.Encode(policy)
	if err != nil {
		return c.fail(ctx, fault.Wrap(fault.KindContract, "output_encoding_failed", "The schema-2 YAML draft could not be encoded.", false, err))
	}
	if len(encoded) > 256*1024 {
		return c.fail(ctx, outputContractExceeded("The schema-2 YAML draft exceeds 256 KiB.", "policy init"))
	}
	return c.emitResult(ctx, encoded)
}

func runBundleBuild(ctx context.Context, c *CLI, _ CommandSpec, intent operation.Intent, inputs ParsedInputs) int {
	result, err := c.bundles.Build(ctx, intent, inputs.One("--catalog"), inputs.One("--policy"))
	if err != nil {
		return c.fail(ctx, err)
	}
	document := bundleBuildDocument{SchemaVersion: 1, Build: bundleBuildPayload{BundleDigest: result.BundleDigest, Bundle: result.Bundle}}
	return c.emitJSONDocument(ctx, document, "bundle build")
}

func runBundleStatus(ctx context.Context, c *CLI, _ CommandSpec, intent operation.Intent, inputs ParsedInputs) int {
	result, err := c.authority.Status(ctx, intent, inputs.One("--bundle"))
	if err != nil {
		return c.fail(ctx, err)
	}
	document := bundleStatusDocument{SchemaVersion: 1, Status: bundleStatusPayload{
		BundleDigest: result.BundleDigest, CatalogDigest: result.CatalogDigest, PolicyDigest: result.PolicyDigest,
		Trust: result.Trust, Source: result.Source, Executable: result.Executable, SourcePath: result.SourcePath,
		SourceSHA256: result.SourceSHA256, SourceVersion: result.SourceVersion, SourceProcessAttempts: result.SourceProcessAttempts,
	}}
	return c.emitJSONDocument(ctx, document, "bundle status")
}

func runBundleTrust(ctx context.Context, c *CLI, spec CommandSpec, intent operation.Intent, inputs ParsedInputs) int {
	if spec.Agent.FixedTarget == nil || spec.Agent.Mutation == nil {
		return c.fail(ctx, fault.New(fault.KindContract, "invalid_mutation_contract", "The bundle trust mutation contract is incomplete.", false))
	}
	intent.Target = operation.TargetRef{Kind: spec.Agent.FixedTarget.Kind, ID: spec.Agent.FixedTarget.ID}
	intent.Impact = spec.Agent.Mutation.Impact
	result, err := c.authority.Trust(ctx, intent, inputs.One("--bundle"))
	if err != nil {
		return c.fail(ctx, err)
	}
	document := bundleTrustDocument{SchemaVersion: 1, Trust: bundleTrustPayload{
		BundleDigest: result.BundleDigest, Trusted: result.Trusted, AlreadyTrusted: result.AlreadyTrusted,
		Source: result.Source, SourceProcessAttempts: result.SourceProcessAttempts,
	}}
	return c.emitJSONDocument(ctx, document, "bundle trust")
}

func (c *CLI) emitJSONDocument(ctx context.Context, document any, command string) int {
	encoded, err := json.Marshal(document)
	if err != nil {
		return c.fail(ctx, fault.Wrap(fault.KindContract, "output_encoding_failed", "The canonical JSON output could not be encoded.", false, err))
	}
	if len(encoded)+1 > maxBundleOutputBytes {
		return c.fail(ctx, outputContractExceeded("The canonical JSON output exceeds its 2 MiB limit.", command))
	}
	return c.emitResult(ctx, append(encoded, '\n'))
}
