package internal

import (
	"flag"
	"fmt"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v2"
)

// createTestContextForRevoke creates a test CLI context with the given flags for revoke command
func createTestContextForRevoke(flags map[string]interface{}) *cli.Context {
	app := &cli.App{}
	set := flag.NewFlagSet("test", flag.ContinueOnError)

	// Set default values
	defaults := map[string]interface{}{
		"token":    "",
		"hostname": "api.github.com",
		"silent":   false,
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
		}
	}

	return cli.NewContext(app, set, nil)
}

func TestRevoke(t *testing.T) {
	// Setup
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	tests := []struct {
		name          string
		flags         map[string]interface{}
		setupMocks    func()
		expectedError string
	}{
		{
			name: "successful_token_revocation_default_hostname",
			flags: map[string]interface{}{
				"token":    "ghs_test_token_123",
				"hostname": "api.github.com",
				"silent":   true,
			},
			setupMocks: func() {
				httpmock.RegisterResponder("DELETE", "https://api.github.com/installation/token",
					httpmock.NewStringResponder(204, ""))
			},
			expectedError: "",
		},
		{
			name: "successful_token_revocation_custom_hostname_without_api_path",
			flags: map[string]interface{}{
				"token":    "ghs_test_token_456",
				"hostname": "github.company.com",
				"silent":   true,
			},
			setupMocks: func() {
				httpmock.RegisterResponder("DELETE", "https://github.company.com/api/v3/installation/token",
					httpmock.NewStringResponder(204, ""))
			},
			expectedError: "",
		},
		{
			name: "successful_token_revocation_custom_hostname_with_api_path",
			flags: map[string]interface{}{
				"token":    "ghs_test_token_789",
				"hostname": "github.company.com/api/v3",
				"silent":   true,
			},
			setupMocks: func() {
				httpmock.RegisterResponder("DELETE", "https://github.company.com/api/v3/installation/token",
					httpmock.NewStringResponder(204, ""))
			},
			expectedError: "",
		},
		{
			name: "successful_token_revocation_verbose_output",
			flags: map[string]interface{}{
				"token":    "ghs_test_token_verbose",
				"hostname": "api.github.com",
				"silent":   false,
			},
			setupMocks: func() {
				httpmock.RegisterResponder("DELETE", "https://api.github.com/installation/token",
					httpmock.NewStringResponder(204, ""))
			},
			expectedError: "",
		},
		{
			name: "error_invalid_token_401",
			flags: map[string]interface{}{
				"token":    "invalid_token",
				"hostname": "api.github.com",
				"silent":   true,
			},
			setupMocks: func() {
				httpmock.RegisterResponder("DELETE", "https://api.github.com/installation/token",
					httpmock.NewStringResponder(401, `{"message": "Bad credentials"}`))
			},
			expectedError: "failed revoking installation token: token might be invalid or not properly formatted. Unexpected status code: 401",
		},
		{
			name: "error_forbidden_403",
			flags: map[string]interface{}{
				"token":    "forbidden_token",
				"hostname": "api.github.com",
				"silent":   true,
			},
			setupMocks: func() {
				httpmock.RegisterResponder("DELETE", "https://api.github.com/installation/token",
					httpmock.NewStringResponder(403, `{"message": "Forbidden"}`))
			},
			expectedError: "failed revoking installation token: token might be invalid or not properly formatted. Unexpected status code: 403",
		},
		{
			name: "error_not_found_404",
			flags: map[string]interface{}{
				"token":    "not_found_token",
				"hostname": "api.github.com",
				"silent":   true,
			},
			setupMocks: func() {
				httpmock.RegisterResponder("DELETE", "https://api.github.com/installation/token",
					httpmock.NewStringResponder(404, `{"message": "Not Found"}`))
			},
			expectedError: "failed revoking installation token: token might be invalid or not properly formatted. Unexpected status code: 404",
		},
		{
			name: "error_server_error_500",
			flags: map[string]interface{}{
				"token":    "server_error_token",
				"hostname": "api.github.com",
				"silent":   true,
			},
			setupMocks: func() {
				httpmock.RegisterResponder("DELETE", "https://api.github.com/installation/token",
					httpmock.NewStringResponder(500, `{"message": "Internal Server Error"}`))
			},
			expectedError: "failed revoking installation token: token might be invalid or not properly formatted. Unexpected status code: 500",
		},
		{
			name: "hostname_case_insensitive",
			flags: map[string]interface{}{
				"token":    "ghs_case_test_token",
				"hostname": "API.GITHUB.COM",
				"silent":   true,
			},
			setupMocks: func() {
				httpmock.RegisterResponder("DELETE", "https://api.github.com/installation/token",
					httpmock.NewStringResponder(204, ""))
			},
			expectedError: "",
		},
		{
			name: "custom_hostname_with_trailing_slash",
			flags: map[string]interface{}{
				"token":    "ghs_trailing_slash_token",
				"hostname": "github.company.com/api/v3/",
				"silent":   true,
			},
			setupMocks: func() {
				// The hostname processing logic preserves the trailing slash when /api/v3 is already present,
				// resulting in a double slash in the final URL
				httpmock.RegisterResponder("DELETE", "https://github.company.com/api/v3//installation/token",
					httpmock.NewStringResponder(204, ""))
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mocks for each test
			httpmock.Reset()
			tt.setupMocks()

			// Create CLI context
			ctx := createTestContextForRevoke(tt.flags)

			// Execute the function
			err := Revoke(ctx)

			// Assert results
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err.Error())
			} else {
				assert.NoError(t, err)
			}

			// Verify HTTP calls were made as expected
			info := httpmock.GetCallCountInfo()
			assert.Greater(t, len(info), 0, "Expected HTTP calls to be made")
		})
	}
}

func TestRevokeToken(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	tests := []struct {
		name          string
		hostname      string
		token         string
		responseCode  int
		responseBody  string
		expectedError string
	}{
		{
			name:          "successful_revocation_github_com",
			hostname:      "api.github.com",
			token:         "ghs_test_token_123",
			responseCode:  204,
			responseBody:  "",
			expectedError: "",
		},
		{
			name:          "successful_revocation_custom_hostname",
			hostname:      "github.company.com/api/v3",
			token:         "ghs_test_token_456",
			responseCode:  204,
			responseBody:  "",
			expectedError: "",
		},
		{
			name:          "error_bad_credentials_401",
			hostname:      "api.github.com",
			token:         "invalid_token",
			responseCode:  401,
			responseBody:  `{"message": "Bad credentials"}`,
			expectedError: "token might be invalid or not properly formatted. Unexpected status code: 401",
		},
		{
			name:          "error_forbidden_403",
			hostname:      "api.github.com",
			token:         "forbidden_token",
			responseCode:  403,
			responseBody:  `{"message": "Forbidden"}`,
			expectedError: "token might be invalid or not properly formatted. Unexpected status code: 403",
		},
		{
			name:          "error_not_found_404",
			hostname:      "api.github.com",
			token:         "not_found_token",
			responseCode:  404,
			responseBody:  `{"message": "Not Found"}`,
			expectedError: "token might be invalid or not properly formatted. Unexpected status code: 404",
		},
		{
			name:          "error_unprocessable_entity_422",
			hostname:      "api.github.com",
			token:         "malformed_token",
			responseCode:  422,
			responseBody:  `{"message": "Unprocessable Entity"}`,
			expectedError: "token might be invalid or not properly formatted. Unexpected status code: 422",
		},
		{
			name:          "error_internal_server_error_500",
			hostname:      "api.github.com",
			token:         "server_error_token",
			responseCode:  500,
			responseBody:  `{"message": "Internal Server Error"}`,
			expectedError: "token might be invalid or not properly formatted. Unexpected status code: 500",
		},
		{
			name:          "error_service_unavailable_503",
			hostname:      "api.github.com",
			token:         "service_unavailable_token",
			responseCode:  503,
			responseBody:  `{"message": "Service Unavailable"}`,
			expectedError: "token might be invalid or not properly formatted. Unexpected status code: 503",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpmock.Reset()

			endpoint := fmt.Sprintf("https://%s/installation/token", tt.hostname)
			httpmock.RegisterResponder("DELETE", endpoint,
				httpmock.NewStringResponder(tt.responseCode, tt.responseBody))

			err := revokeToken(tt.hostname, tt.token)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err.Error())
			} else {
				assert.NoError(t, err)
			}

			// Verify the request was made with correct method and endpoint
			info := httpmock.GetCallCountInfo()
			assert.Equal(t, 1, info[fmt.Sprintf("DELETE %s", endpoint)], "Expected exactly one DELETE request to be made")
		})
	}
}

func TestRevokeTokenNetworkErrors(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	tests := []struct {
		name          string
		hostname      string
		token         string
		setupMock     func()
		expectedError string
		errorContains string
	}{
		{
			name:     "network_connection_error",
			hostname: "api.github.com",
			token:    "ghs_network_error_token",
			setupMock: func() {
				httpmock.RegisterResponder("DELETE", "https://api.github.com/installation/token",
					httpmock.NewErrorResponder(fmt.Errorf("connection refused")))
			},
			errorContains: "unable to DELETE to https://api.github.com/installation/token",
		},
		{
			name:     "timeout_error",
			hostname: "api.github.com",
			token:    "ghs_timeout_token",
			setupMock: func() {
				httpmock.RegisterResponder("DELETE", "https://api.github.com/installation/token",
					httpmock.NewErrorResponder(fmt.Errorf("request timeout")))
			},
			errorContains: "unable to DELETE to https://api.github.com/installation/token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpmock.Reset()
			tt.setupMock()

			err := revokeToken(tt.hostname, tt.token)

			assert.Error(t, err)
			if tt.expectedError != "" {
				assert.Equal(t, tt.expectedError, err.Error())
			} else if tt.errorContains != "" {
				assert.Contains(t, err.Error(), tt.errorContains)
			}

			// Verify the request was attempted
			info := httpmock.GetCallCountInfo()
			endpoint := fmt.Sprintf("https://%s/installation/token", tt.hostname)
			assert.Equal(t, 1, info[fmt.Sprintf("DELETE %s", endpoint)], "Expected exactly one DELETE request to be attempted")
		})
	}
}

func TestRevokeAdvancedCases(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	tests := []struct {
		name       string
		setupTest  func() *cli.Context
		setupMocks func()
		verifyFunc func(t *testing.T)
	}{
		{
			name: "verify_request_headers",
			setupTest: func() *cli.Context {
				return createTestContextForRevoke(map[string]interface{}{
					"token":    "ghs_header_test_token",
					"hostname": "api.github.com",
					"silent":   true,
				})
			},
			setupMocks: func() {
				httpmock.RegisterResponder("DELETE", "https://api.github.com/installation/token",
					func(req *http.Request) (*http.Response, error) {
						// Verify required headers are present
						assert.Equal(t, "Bearer ghs_header_test_token", req.Header.Get("Authorization"))
						assert.Equal(t, "application/vnd.github+json", req.Header.Get("Accept"))
						assert.Equal(t, "2022-11-28", req.Header.Get("X-GitHub-Api-Version"))
						assert.Equal(t, "Link-/gh-token", req.Header.Get("User-Agent"))
						return httpmock.NewStringResponse(204, ""), nil
					})
			},
			verifyFunc: func(t *testing.T) {
				// Headers are verified in the mock responder
			},
		},
		{
			name: "hostname_normalization_mixed_case",
			setupTest: func() *cli.Context {
				return createTestContextForRevoke(map[string]interface{}{
					"token":    "ghs_mixed_case_token",
					"hostname": "GitHub.Company.Com",
					"silent":   true,
				})
			},
			setupMocks: func() {
				// The hostname should be normalized to lowercase and have /api/v3 appended
				httpmock.RegisterResponder("DELETE", "https://github.company.com/api/v3/installation/token",
					httpmock.NewStringResponder(204, ""))
			},
			verifyFunc: func(t *testing.T) {
				info := httpmock.GetCallCountInfo()
				assert.Equal(t, 1, info["DELETE https://github.company.com/api/v3/installation/token"])
			},
		},
		{
			name: "empty_token_string",
			setupTest: func() *cli.Context {
				return createTestContextForRevoke(map[string]interface{}{
					"token":    "",
					"hostname": "api.github.com",
					"silent":   true,
				})
			},
			setupMocks: func() {
				httpmock.RegisterResponder("DELETE", "https://api.github.com/installation/token",
					httpmock.NewStringResponder(401, `{"message": "Bad credentials"}`))
			},
			verifyFunc: func(t *testing.T) {
				// Should still attempt the request even with empty token
				info := httpmock.GetCallCountInfo()
				assert.Equal(t, 1, info["DELETE https://api.github.com/installation/token"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpmock.Reset()
			tt.setupMocks()

			ctx := tt.setupTest()
			_ = Revoke(ctx) // We don't check error here as we're testing specific behaviors

			tt.verifyFunc(t)
		})
	}
}
