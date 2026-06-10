package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nikiv/ansible-ui/internal/model"
)

// cloudInventory generates the ansible inventory-plugin config for a turnkey
// cloud inventory and returns the provider credentials to pass as process env
// (so the plugin authenticates without anything secret touching the playbook,
// the command line or the log). Extra plugin YAML in inv.Content is appended.
func (s *Server) cloudInventory(ctx context.Context, inv *model.Inventory) (cfg string, env map[string]string, err error) {
	env = map[string]string{}
	switch inv.Provider {
	case model.CloudProviderAWSEC2:
		var b strings.Builder
		b.WriteString("plugin: amazon.aws.aws_ec2\n")
		if inv.Region != "" {
			b.WriteString("regions:\n  - " + inv.Region + "\n")
		}
		if extra := strings.TrimSpace(inv.Content); extra != "" {
			b.WriteString(extra)
			b.WriteString("\n")
		}
		cfg = b.String()
		if inv.CredentialID != nil && *inv.CredentialID != "" {
			cred, cerr := s.store.GetCredential(ctx, *inv.CredentialID)
			sec, serr := s.credentialSecret(ctx, *inv.CredentialID)
			if cerr == nil && serr == nil && cred != nil && sec != nil {
				env["AWS_ACCESS_KEY_ID"] = cred.Login
				env["AWS_SECRET_ACCESS_KEY"] = sec.Password
			}
		}
		if inv.Region != "" {
			env["AWS_DEFAULT_REGION"] = inv.Region
		}
		return cfg, env, nil

	case model.CloudProviderGCPCompute:
		var sa string
		if inv.CredentialID != nil && *inv.CredentialID != "" {
			if sec, serr := s.credentialSecret(ctx, *inv.CredentialID); serr == nil && sec != nil {
				sa = sec.ServiceAccount
			}
		}
		if sa == "" {
			return "", nil, fmt.Errorf("a GCP service-account credential is required")
		}
		// Project: explicit (Region field) or read from the service-account JSON.
		project := inv.Region
		if project == "" {
			var saj struct {
				ProjectID string `json:"project_id"`
			}
			_ = json.Unmarshal([]byte(sa), &saj)
			project = saj.ProjectID
		}
		var b strings.Builder
		b.WriteString("plugin: google.cloud.gcp_compute\n")
		b.WriteString("auth_kind: serviceaccount\n")
		if project != "" {
			b.WriteString("projects:\n  - " + project + "\n")
		}
		if extra := strings.TrimSpace(inv.Content); extra != "" {
			b.WriteString(extra)
			b.WriteString("\n")
		}
		// Creds via env (the google.cloud auth reads these) — never on disk / in the log.
		env["GCP_AUTH_KIND"] = "serviceaccount"
		env["GCP_SERVICE_ACCOUNT_CONTENTS"] = sa
		if project != "" {
			env["GCP_PROJECT"] = project
		}
		return b.String(), env, nil

	case model.CloudProviderAzureRM:
		var sp struct {
			SubscriptionID string `json:"subscriptionId"`
			TenantID       string `json:"tenantId"`
			ClientID       string `json:"clientId"`
			Secret         string `json:"secret"`
		}
		if inv.CredentialID != nil && *inv.CredentialID != "" {
			if sec, serr := s.credentialSecret(ctx, *inv.CredentialID); serr == nil && sec != nil && sec.ServiceAccount != "" {
				_ = json.Unmarshal([]byte(sec.ServiceAccount), &sp)
			}
		}
		if sp.SubscriptionID == "" || sp.ClientID == "" {
			return "", nil, fmt.Errorf("an Azure service-principal credential (subscription/tenant/client/secret) is required")
		}
		var b strings.Builder
		b.WriteString("plugin: azure.azcollection.azure_rm\n")
		b.WriteString("auth_source: env\n")
		b.WriteString("plain_host_names: true\n")
		if extra := strings.TrimSpace(inv.Content); extra != "" {
			b.WriteString(extra)
			b.WriteString("\n")
		}
		// Creds via env (azure.azcollection reads AZURE_* automatically) — never on disk.
		env["AZURE_SUBSCRIPTION_ID"] = sp.SubscriptionID
		env["AZURE_TENANT"] = sp.TenantID
		env["AZURE_CLIENT_ID"] = sp.ClientID
		env["AZURE_SECRET"] = sp.Secret
		return b.String(), env, nil

	default:
		return "", nil, fmt.Errorf("unsupported cloud inventory provider %q", inv.Provider)
	}
}
