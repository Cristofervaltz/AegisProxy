package secrets

import (
	"context"
	"fmt"
	"time"

	vault "github.com/hashicorp/vault/api"
)

// Manager defines how to fetch secure keys
type Manager interface {
	GetKeys(ctx context.Context) (map[string]string, error)
}

// EnvManager implements Manager using environment variables
type EnvManager struct {
	keys map[string]string
}

func NewEnvManager(openai, anthropic, gemini string) *EnvManager {
	return &EnvManager{
		keys: map[string]string{
			"OPENAI_API_KEY":    openai,
			"ANTHROPIC_API_KEY": anthropic,
			"GEMINI_API_KEY":    gemini,
		},
	}
}

func (e *EnvManager) GetKeys(ctx context.Context) (map[string]string, error) {
	return e.keys, nil
}

// VaultManager implements Manager using HashiCorp Vault
type VaultManager struct {
	client     *vault.Client
	secretPath string
}

// NewVaultManager creates a new Vault connection
func NewVaultManager(addr, token, secretPath string) (*VaultManager, error) {
	config := vault.DefaultConfig()
	config.Address = addr
	config.Timeout = 5 * time.Second

	client, err := vault.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize vault client: %w", err)
	}

	client.SetToken(token)

	return &VaultManager{
		client:     client,
		secretPath: secretPath,
	}, nil
}

// GetKeys retrieves all API keys from Vault
func (v *VaultManager) GetKeys(ctx context.Context) (map[string]string, error) {
	keys := make(map[string]string)
	
	secret, err := v.client.KVv2("secret").Get(ctx, v.secretPath)
	if err != nil {
		s, fallbackErr := v.client.Logical().Read(v.secretPath)
		if fallbackErr != nil || s == nil {
			return nil, fmt.Errorf("failed to read secret from vault: %v (fallback error: %v)", err, fallbackErr)
		}
		
		for k, val := range s.Data {
			if strVal, ok := val.(string); ok {
				keys[k] = strVal
			}
		}
		return keys, nil
	}

	if secret != nil && secret.Data != nil {
		for k, val := range secret.Data {
			if strVal, ok := val.(string); ok {
				keys[k] = strVal
			}
		}
		return keys, nil
	}

	return keys, nil
}
