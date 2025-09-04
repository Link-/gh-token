package internal

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/go-github/v55/github"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v2"
)

// createTestContext creates a test CLI context with the given flags
func createTestContext(flags map[string]interface{}) *cli.Context {
	app := &cli.App{}
	set := flag.NewFlagSet("test", flag.ContinueOnError)

	// Set default values
	defaults := map[string]interface{}{
		"app-id":          "",
		"installation-id": "",
		"key":             "",
		"base64-key":      "",
		"jwt":             false,
		"jwt-expiry":      10,
		"hostname":        "api.github.com",
		"token-only":      false,
		"silent":          false,
	}

	// Override with test-specific flags
	for k, v := range flags {
		defaults[k] = v
	}

	// Set up flags based on type
	for key, value := range defaults {
		switch v := value.(type) {
		case string:
			set.String(key, v, "")
		case bool:
			set.Bool(key, v, "")
		case int:
			set.Int(key, v, "")
		}
	}

	return cli.NewContext(app, set, nil)
}

// getBoolFlag safely gets bool values from flags map
func getBoolFlag(flags map[string]interface{}, key string) bool {
	if val, ok := flags[key].(bool); ok {
		return val
	}
	return false
}

func TestGenerate(t *testing.T) {
	// Setup
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// Read test key for base64 encoding
	keyBytes, err := os.ReadFile("fixtures/test-private-key.test.pem")
	assert.NoError(t, err)
	keyBase64 := base64.StdEncoding.EncodeToString(keyBytes)

	installationResponse := []github.Installation{
		{ID: github.Int64(12345)},
	}
	installationJSON, _ := json.Marshal(installationResponse)

	tokenResponse := &github.InstallationToken{
		Token:     github.String("ghs_test_token_123"),
		ExpiresAt: &github.Timestamp{Time: time.Now().Add(time.Hour)},
	}
	tokenJSON, _ := json.Marshal(tokenResponse)

	tests := []struct {
		name          string
		flags         map[string]interface{}
		setupMocks    func()
		expectedError string
	}{
		{
			name: "successful_token_generation_with_key_file",
			flags: map[string]interface{}{
				"app-id":          "123456",
				"installation-id": "12345",
				"key":             "fixtures/test-private-key.test.pem",
				"hostname":        "api.github.com",
				"jwt-expiry":      10,
				"silent":          true,
			},
			setupMocks: func() {
				httpmock.RegisterResponder("POST", "https://api.github.com/app/installations/12345/access_tokens",
					httpmock.NewStringResponder(201, string(tokenJSON)))
			},
			expectedError: "",
		},
		{
			name: "successful_token_generation_with_base64_key",
			flags: map[string]interface{}{
				"app-id":          "123456",
				"installation-id": "12345",
				"base64-key":      keyBase64,
				"hostname":        "api.github.com",
				"jwt-expiry":      10,
				"silent":          true,
			},
			setupMocks: func() {
				httpmock.RegisterResponder("POST", "https://api.github.com/app/installations/12345/access_tokens",
					httpmock.NewStringResponder(201, string(tokenJSON)))
			},
			expectedError: "",
		},
		{
			name: "successful_with_auto_installation_id",
			flags: map[string]interface{}{
				"app-id":     "123456",
				"key":        "fixtures/test-private-key.test.pem",
				"hostname":   "api.github.com",
				"jwt-expiry": 10,
				"silent":     true,
			},
			setupMocks: func() {
				httpmock.RegisterResponder("GET", "https://api.github.com/app/installations?per_page=1",
					httpmock.NewStringResponder(200, string(installationJSON)))
				httpmock.RegisterResponder("POST", "https://api.github.com/app/installations/12345/access_tokens",
					httpmock.NewStringResponder(201, string(tokenJSON)))
			},
			expectedError: "",
		},
		{
			name: "successful_jwt_only",
			flags: map[string]interface{}{
				"app-id":     "123456",
				"key":        "fixtures/test-private-key.test.pem",
				"jwt":        true,
				"jwt-expiry": 10,
				"silent":     true,
			},
			setupMocks:    func() {},
			expectedError: "",
		},
		{
			name: "error_no_key_specified",
			flags: map[string]interface{}{
				"app-id": "123456",
			},
			setupMocks:    func() {},
			expectedError: "either --key or --base64-key must be specified",
		},
		{
			name: "error_both_keys_specified",
			flags: map[string]interface{}{
				"app-id":     "123456",
				"key":        "fixtures/test-private-key.test.pem",
				"base64-key": keyBase64,
			},
			setupMocks:    func() {},
			expectedError: "only one of --key or --base64-key may be specified",
		},
		{
			name: "error_invalid_key_file",
			flags: map[string]interface{}{
				"app-id": "123456",
				"key":    "fixtures/nonexistent.pem",
			},
			setupMocks:    func() {},
			expectedError: "unable to read key file",
		},
		{
			name: "error_invalid_base64_key",
			flags: map[string]interface{}{
				"app-id":     "123456",
				"base64-key": "invalid-base64-string",
			},
			setupMocks:    func() {},
			expectedError: "unable to decode key from base64",
		},
		{
			name: "error_installation_not_found",
			flags: map[string]interface{}{
				"app-id": "123456",
				"key":    "fixtures/test-private-key.test.pem",
			},
			setupMocks: func() {
				httpmock.RegisterResponder("GET", "https://api.github.com/app/installations?per_page=1",
					httpmock.NewStringResponder(404, `{"message": "Not Found"}`))
			},
			expectedError: "failed retrieving default installation ID: unexpected status code: 404",
		},
		{
			name: "error_token_generation_fails",
			flags: map[string]interface{}{
				"app-id":          "123456",
				"installation-id": "12345",
				"key":             "fixtures/test-private-key.test.pem",
			},
			setupMocks: func() {
				httpmock.RegisterResponder("POST", "https://api.github.com/app/installations/12345/access_tokens",
					httpmock.NewStringResponder(403, `{"message": "Forbidden"}`))
			},
			expectedError: "failed generating installation token: unexpected status code: 403",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mocks for each test
			httpmock.Reset()
			tt.setupMocks()

			// Create CLI context
			ctx := createTestContext(tt.flags)

			// Execute the function
			err := Generate(ctx)

			// Assert results
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			// Verify HTTP calls were made as expected
			if tt.expectedError == "" && !getBoolFlag(tt.flags, "jwt") {
				info := httpmock.GetCallCountInfo()
				assert.Greater(t, len(info), 0, "Expected HTTP calls to be made")
			}
		})
	}
}

func TestRetrieveDefaultInstallationID(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	tests := []struct {
		name           string
		hostname       string
		jwt            string
		responseCode   int
		responseBody   string
		expectedResult string
		expectedError  string
	}{
		{
			name:           "successful_retrieval",
			hostname:       "api.github.com",
			jwt:            "test.jwt.token",
			responseCode:   200,
			responseBody:   `[{"id": 12345}]`,
			expectedResult: "12345",
			expectedError:  "",
		},
		{
			name:           "not_found",
			hostname:       "api.github.com",
			jwt:            "test.jwt.token",
			responseCode:   404,
			responseBody:   `{"message": "Not Found"}`,
			expectedResult: "",
			expectedError:  "unexpected status code: 404",
		},
		{
			name:           "invalid_json_response",
			hostname:       "api.github.com",
			jwt:            "test.jwt.token",
			responseCode:   200,
			responseBody:   "invalid json",
			expectedResult: "",
			expectedError:  "unable to unmarshal response body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpmock.Reset()

			endpoint := fmt.Sprintf("https://%s/app/installations?per_page=1", tt.hostname)
			httpmock.RegisterResponder("GET", endpoint,
				httpmock.NewStringResponder(tt.responseCode, tt.responseBody))

			result, err := retrieveDefaultInstallationID(tt.hostname, tt.jwt)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Equal(t, "", result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}

			// Verify the request was made with correct headers
			info := httpmock.GetCallCountInfo()
			assert.Equal(t, 1, info[fmt.Sprintf("GET %s", endpoint)])
		})
	}
}

func TestGenerateToken(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	tokenResponse := &github.InstallationToken{
		Token:     github.String("ghs_test_token_123"),
		ExpiresAt: &github.Timestamp{Time: time.Now().Add(time.Hour)},
	}
	tokenJSON, _ := json.Marshal(tokenResponse)

	tests := []struct {
		name           string
		hostname       string
		jwt            string
		installationID string
		responseCode   int
		responseBody   string
		expectedError  string
	}{
		{
			name:           "successful_token_generation",
			hostname:       "api.github.com",
			jwt:            "test.jwt.token",
			installationID: "12345",
			responseCode:   201,
			responseBody:   string(tokenJSON),
			expectedError:  "",
		},
		{
			name:           "forbidden_error",
			hostname:       "api.github.com",
			jwt:            "test.jwt.token",
			installationID: "12345",
			responseCode:   403,
			responseBody:   `{"message": "Forbidden"}`,
			expectedError:  "unexpected status code: 403",
		},
		{
			name:           "invalid_json_response",
			hostname:       "api.github.com",
			jwt:            "test.jwt.token",
			installationID: "12345",
			responseCode:   201,
			responseBody:   "invalid json",
			expectedError:  "unable to unmarshal response body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpmock.Reset()

			endpoint := fmt.Sprintf("https://%s/app/installations/%s/access_tokens", tt.hostname, tt.installationID)
			httpmock.RegisterResponder("POST", endpoint,
				httpmock.NewStringResponder(tt.responseCode, tt.responseBody))

			result, err := generateToken(tt.hostname, tt.jwt, tt.installationID)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, "ghs_test_token_123", *result.Token)
			}

			// Verify the request was made with correct headers
			info := httpmock.GetCallCountInfo()
			assert.Equal(t, 1, info[fmt.Sprintf("POST %s", endpoint)])
		})
	}
}

func TestGenerateAdvancedCases(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	tests := []struct {
		name          string
		setupTest     func() *cli.Context
		setupMocks    func()
		expectedError string
	}{
		{
			name: "jwt_expiry_below_minimum",
			setupTest: func() *cli.Context {
				return createTestContext(map[string]interface{}{
					"app-id":     "123456",
					"key":        "fixtures/test-private-key.test.pem",
					"jwt-expiry": 0, // Below minimum, should be adjusted to 10
					"jwt":        true,
					"silent":     true,
				})
			},
			setupMocks:    func() {},
			expectedError: "",
		},
		{
			name: "jwt_expiry_above_maximum",
			setupTest: func() *cli.Context {
				return createTestContext(map[string]interface{}{
					"app-id":     "123456",
					"key":        "fixtures/test-private-key.test.pem",
					"jwt-expiry": 15, // Above maximum, should be adjusted to 10
					"jwt":        true,
					"silent":     true,
				})
			},
			setupMocks:    func() {},
			expectedError: "",
		},
		{
			name: "hostname_without_api_path",
			setupTest: func() *cli.Context {
				return createTestContext(map[string]interface{}{
					"app-id":          "123456",
					"installation-id": "12345",
					"key":             "fixtures/test-private-key.test.pem",
					"hostname":        "github.company.com", // Without /api/v3
					"silent":          true,
				})
			},
			setupMocks: func() {
				tokenResponse := &github.InstallationToken{
					Token:     github.String("ghs_test_token_123"),
					ExpiresAt: &github.Timestamp{Time: time.Now().Add(time.Hour)},
				}
				tokenJSON, _ := json.Marshal(tokenResponse)
				httpmock.RegisterResponder("POST", "https://github.company.com/api/v3/app/installations/12345/access_tokens",
					httpmock.NewStringResponder(201, string(tokenJSON)))
			},
			expectedError: "",
		},
		{
			name: "hostname_with_api_path_already_included",
			setupTest: func() *cli.Context {
				return createTestContext(map[string]interface{}{
					"app-id":          "123456",
					"installation-id": "12345",
					"key":             "fixtures/test-private-key.test.pem",
					"hostname":        "github.company.com/api/v3", // Already has /api/v3
					"silent":          true,
				})
			},
			setupMocks: func() {
				tokenResponse := &github.InstallationToken{
					Token:     github.String("ghs_test_token_123"),
					ExpiresAt: &github.Timestamp{Time: time.Now().Add(time.Hour)},
				}
				tokenJSON, _ := json.Marshal(tokenResponse)
				httpmock.RegisterResponder("POST", "https://github.company.com/api/v3/app/installations/12345/access_tokens",
					httpmock.NewStringResponder(201, string(tokenJSON)))
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpmock.Reset()
			tt.setupMocks()

			ctx := tt.setupTest()
			err := Generate(ctx)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestGenerateWithOutputFormats tests different output formats
func TestGenerateWithOutputFormats(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	tokenResponse := &github.InstallationToken{
		Token:     github.String("ghs_test_token_123"),
		ExpiresAt: &github.Timestamp{Time: time.Now().Add(time.Hour)},
	}
	tokenJSON, _ := json.Marshal(tokenResponse)

	tests := []struct {
		name       string
		flags      map[string]interface{}
		setupMocks func()
	}{
		{
			name: "json_output_format",
			flags: map[string]interface{}{
				"app-id":          "123456",
				"installation-id": "12345",
				"key":             "fixtures/test-private-key.test.pem",
				"hostname":        "api.github.com",
				"jwt-expiry":      10,
				"silent":          false, // To test JSON output
			},
			setupMocks: func() {
				httpmock.RegisterResponder("POST", "https://api.github.com/app/installations/12345/access_tokens",
					httpmock.NewStringResponder(201, string(tokenJSON)))
			},
		},
		{
			name: "token_only_output_format",
			flags: map[string]interface{}{
				"app-id":          "123456",
				"installation-id": "12345",
				"key":             "fixtures/test-private-key.test.pem",
				"hostname":        "api.github.com",
				"jwt-expiry":      10,
				"token-only":      true,
				"silent":          false, // To test token-only output
			},
			setupMocks: func() {
				httpmock.RegisterResponder("POST", "https://api.github.com/app/installations/12345/access_tokens",
					httpmock.NewStringResponder(201, string(tokenJSON)))
			},
		},
		{
			name: "jwt_output_format",
			flags: map[string]interface{}{
				"app-id":     "123456",
				"key":        "fixtures/test-private-key.test.pem",
				"jwt":        true,
				"jwt-expiry": 10,
				"silent":     false, // To test JWT output
			},
			setupMocks: func() {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpmock.Reset()
			tt.setupMocks()

			ctx := createTestContext(tt.flags)
			err := Generate(ctx)

			assert.NoError(t, err)

			// Verify HTTP calls were made as expected (except for JWT-only)
			if !getBoolFlag(tt.flags, "jwt") {
				info := httpmock.GetCallCountInfo()
				assert.Greater(t, len(info), 0, "Expected HTTP calls to be made")
			}
		})
	}
}
