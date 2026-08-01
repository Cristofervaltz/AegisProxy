package secrets

import (
	"context"
	"fmt"
	"time"

	vault "github.com/hashicorp/vault/api"
)

// Manager defines how to fetch secure keys
type Manager interface {
	GetOpenAIKey(ctx context.Context) (string, error)
}

// EnvManager implements Manager using environment variables
type EnvManager struct {
	key string
}

func NewEnvManager(key string) *EnvManager {
	return &EnvManager{key: key}
}

func (e *EnvManager) GetOpenAIKey(ctx context.Context) (string, error) {
	if e.key == "" {
		return "", fmt.Errorf("api key not set in environment")
	}
	return e.key, nil
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

// GetOpenAIKey retrieves the OpenAI API key from Vault
func (v *VaultManager) GetOpenAIKey(ctx context.Context) (string, error) {
	secret, err := v.client.KVv2("secret").Get(ctx, v.secretPath) // KVv2 is standard for newer Vault
	if err != nil {
		// Try fallback to standard Read if KVv2 helper fails (e.g., using KVv1)
		s, fallbackErr := v.client.Logical().Read(v.secretPath)
		if fallbackErr != nil || s == nil {
			return "", fmt.Errorf("failed to read secret from vault: %v (fallback error: %v)", err, fallbackErr)
		}

		data := s.Data
		if key, ok := data["api_key"].(string); ok {
			return key, nil
		}
		return "", fmt.Errorf("api_key not found in secret data")
	}

	if secret != nil && secret.Data != nil {
		if key, ok := secret.Data["api_key"].(string); ok {
			return key, nil
		}
	}

	return "", fmt.Errorf("api_key not found in secret data")
}
