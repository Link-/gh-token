package internal

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/google/go-github/v55/github"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v2"
)

// createTestContextForInstallations creates a test CLI context with the given flags for installations command
func createTestContextForInstallations(flags map[string]interface{}) *cli.Context {
	app := &cli.App{}
	set := flag.NewFlagSet("test", flag.ContinueOnError)

	// Set default values
	defaults := map[string]interface{}{
		"app-id":     "",
		"key":        "",
		"base64-key": "",
		"hostname":   "api.github.com",
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

func TestInstallations(t *testing.T) {
	// Setup
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// Read test key for base64 encoding
	keyBytes, err := os.ReadFile("fixtures/test-private-key.pem")
	assert.NoError(t, err)
	keyBase64 := base64.StdEncoding.EncodeToString(keyBytes)

	// Sample installation responses
	singleInstallationResponse := []github.Installation{
		{
			ID:      github.Int64(12345),
			Account: &github.User{Login: github.String("testuser")},
		},
	}
	singleInstallationJSON, _ := json.Marshal(singleInstallationResponse)

	multipleInstallationsResponse := []github.Installation{
		{
			ID:      github.Int64(12345),
			Account: &github.User{Login: github.String("testuser1")},
		},
		{
			ID:      github.Int64(67890),
			Account: &github.User{Login: github.String("testuser2")},
		},
	}
	multipleInstallationsJSON, _ := json.Marshal(multipleInstallationsResponse)

	emptyInstallationsResponse := []github.Installation{}
	emptyInstallationsJSON, _ := json.Marshal(emptyInstallationsResponse)

	tests := []struct {
		name          string
		flags         map[string]interface{}
		setupMocks    func()
		expectedError string
	}{
		{
			name: "successful_list_installations_with_key_file",
			flags: map[string]interface{}{
				"app-id":   "123456",
				"key":      "fixtures/test-private-key.pem",
				"hostname": "api.github.com",
			},
			setupMocks: func() {
				httpmock.RegisterResponder("GET", "https://api.github.com/app/installations?per_page=100&page=0",
					httpmock.NewStringResponder(200, string(singleInstallationJSON)))
			},
			expectedError: "",
		},
		{
			name: "successful_list_installations_with_base64_key",
			flags: map[string]interface{}{
				"app-id":     "123456",
				"base64-key": keyBase64,
				"hostname":   "api.github.com",
			},
			setupMocks: func() {
				httpmock.RegisterResponder("GET", "https://api.github.com/app/installations?per_page=100&page=0",
					httpmock.NewStringResponder(200, string(singleInstallationJSON)))
			},
			expectedError: "",
		},
		{
			name: "successful_list_multiple_installations",
			flags: map[string]interface{}{
				"app-id":   "123456",
				"key":      "fixtures/test-private-key.pem",
				"hostname": "api.github.com",
			},
			setupMocks: func() {
				httpmock.RegisterResponder("GET", "https://api.github.com/app/installations?per_page=100&page=0",
					httpmock.NewStringResponder(200, string(multipleInstallationsJSON)))
			},
			expectedError: "",
		},
		{
			name: "successful_empty_installations_list",
			flags: map[string]interface{}{
				"app-id":   "123456",
				"key":      "fixtures/test-private-key.pem",
				"hostname": "api.github.com",
			},
			setupMocks: func() {
				httpmock.RegisterResponder("GET", "https://api.github.com/app/installations?per_page=100&page=0",
					httpmock.NewStringResponder(200, string(emptyInstallationsJSON)))
			},
			expectedError: "",
		},
		{
			name: "successful_with_custom_hostname_without_api_path",
			flags: map[string]interface{}{
				"app-id":   "123456",
				"key":      "fixtures/test-private-key.pem",
				"hostname": "github.company.com",
			},
			setupMocks: func() {
				httpmock.RegisterResponder("GET", "https://github.company.com/api/v3/app/installations?per_page=100&page=0",
					httpmock.NewStringResponder(200, string(singleInstallationJSON)))
			},
			expectedError: "",
		},
		{
			name: "successful_with_custom_hostname_with_api_path",
			flags: map[string]interface{}{
				"app-id":   "123456",
				"key":      "fixtures/test-private-key.pem",
				"hostname": "github.company.com/api/v3",
			},
			setupMocks: func() {
				httpmock.RegisterResponder("GET", "https://github.company.com/api/v3/app/installations?per_page=100&page=0",
					httpmock.NewStringResponder(200, string(singleInstallationJSON)))
			},
			expectedError: "",
		},
		{
			name: "successful_with_mixed_case_hostname",
			flags: map[string]interface{}{
				"app-id":   "123456",
				"key":      "fixtures/test-private-key.pem",
				"hostname": "GitHub.Company.COM",
			},
			setupMocks: func() {
				httpmock.RegisterResponder("GET", "https://github.company.com/api/v3/app/installations?per_page=100&page=0",
					httpmock.NewStringResponder(200, string(singleInstallationJSON)))
			},
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
				"key":        "fixtures/test-private-key.pem",
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
			name: "error_http_request_fails",
			flags: map[string]interface{}{
				"app-id":   "123456",
				"key":      "fixtures/test-private-key.pem",
				"hostname": "api.github.com",
			},
			setupMocks: func() {
				httpmock.RegisterResponder("GET", "https://api.github.com/app/installations?per_page=100&page=0",
					httpmock.NewErrorResponder(fmt.Errorf("network error")))
			},
			expectedError: "failed listing installations",
		},
		{
			name: "error_http_status_not_200",
			flags: map[string]interface{}{
				"app-id":   "123456",
				"key":      "fixtures/test-private-key.pem",
				"hostname": "api.github.com",
			},
			setupMocks: func() {
				httpmock.RegisterResponder("GET", "https://api.github.com/app/installations?per_page=100&page=0",
					httpmock.NewStringResponder(404, `{"message": "Not Found"}`))
			},
			expectedError: "failed listing installations: unexpected status code: 404",
		},
		{
			name: "error_invalid_json_response",
			flags: map[string]interface{}{
				"app-id":   "123456",
				"key":      "fixtures/test-private-key.pem",
				"hostname": "api.github.com",
			},
			setupMocks: func() {
				httpmock.RegisterResponder("GET", "https://api.github.com/app/installations?per_page=100&page=0",
					httpmock.NewStringResponder(200, "invalid json"))
			},
			expectedError: "failed listing installations: unable to unmarshal response body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mocks for each test
			httpmock.Reset()
			tt.setupMocks()

			// Create CLI context
			ctx := createTestContextForInstallations(tt.flags)

			// Execute the function
			err := Installations(ctx)

			// Assert results
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)

				// Verify HTTP calls were made as expected
				info := httpmock.GetCallCountInfo()
				assert.Greater(t, len(info), 0, "Expected HTTP calls to be made")
			}
		})
	}
}

func TestListInstallations(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// Sample installation responses
	firstPageResponse := []github.Installation{
		{ID: github.Int64(12345), Account: &github.User{Login: github.String("user1")}},
		{ID: github.Int64(67890), Account: &github.User{Login: github.String("user2")}},
	}
	firstPageJSON, _ := json.Marshal(firstPageResponse)

	secondPageResponse := []github.Installation{
		{ID: github.Int64(11111), Account: &github.User{Login: github.String("user3")}},
	}
	secondPageJSON, _ := json.Marshal(secondPageResponse)

	emptyResponse := []github.Installation{}
	emptyResponseJSON, _ := json.Marshal(emptyResponse)

	// Create a response with 100 items to test pagination
	fullPageResponse := make([]github.Installation, 100)
	for i := 0; i < 100; i++ {
		fullPageResponse[i] = github.Installation{
			ID:      github.Int64(int64(i + 1000)),
			Account: &github.User{Login: github.String(fmt.Sprintf("user%d", i))},
		}
	}
	fullPageJSON, _ := json.Marshal(fullPageResponse)

	tests := []struct {
		name          string
		hostname      string
		jwt           string
		setupMocks    func()
		expectedCount int
		expectedError string
	}{
		{
			name:     "successful_single_page",
			hostname: "api.github.com",
			jwt:      "test.jwt.token",
			setupMocks: func() {
				httpmock.RegisterResponder("GET", "https://api.github.com/app/installations?per_page=100&page=0",
					httpmock.NewStringResponder(200, string(firstPageJSON)))
			},
			expectedCount: 2,
			expectedError: "",
		},
		{
			name:     "successful_multiple_pages",
			hostname: "api.github.com",
			jwt:      "test.jwt.token",
			setupMocks: func() {
				httpmock.RegisterResponder("GET", "https://api.github.com/app/installations?per_page=100&page=0",
					httpmock.NewStringResponder(200, string(fullPageJSON)))
				httpmock.RegisterResponder("GET", "https://api.github.com/app/installations?per_page=100&page=1",
					httpmock.NewStringResponder(200, string(secondPageJSON)))
			},
			expectedCount: 101,
			expectedError: "",
		},
		{
			name:     "successful_empty_response",
			hostname: "api.github.com",
			jwt:      "test.jwt.token",
			setupMocks: func() {
				httpmock.RegisterResponder("GET", "https://api.github.com/app/installations?per_page=100&page=0",
					httpmock.NewStringResponder(200, string(emptyResponseJSON)))
			},
			expectedCount: 0,
			expectedError: "",
		},
		{
			name:     "successful_with_custom_hostname",
			hostname: "github.company.com/api/v3",
			jwt:      "test.jwt.token",
			setupMocks: func() {
				httpmock.RegisterResponder("GET", "https://github.company.com/api/v3/app/installations?per_page=100&page=0",
					httpmock.NewStringResponder(200, string(firstPageJSON)))
			},
			expectedCount: 2,
			expectedError: "",
		},
		{
			name:     "error_http_request_fails",
			hostname: "api.github.com",
			jwt:      "test.jwt.token",
			setupMocks: func() {
				httpmock.RegisterResponder("GET", "https://api.github.com/app/installations?per_page=100&page=0",
					httpmock.NewErrorResponder(fmt.Errorf("network error")))
			},
			expectedCount: 0,
			expectedError: "unable to POST to",
		},
		{
			name:     "error_status_not_200",
			hostname: "api.github.com",
			jwt:      "test.jwt.token",
			setupMocks: func() {
				httpmock.RegisterResponder("GET", "https://api.github.com/app/installations?per_page=100&page=0",
					httpmock.NewStringResponder(401, `{"message": "Unauthorized"}`))
			},
			expectedCount: 0,
			expectedError: "unexpected status code: 401",
		},
		{
			name:     "error_invalid_json_response",
			hostname: "api.github.com",
			jwt:      "test.jwt.token",
			setupMocks: func() {
				httpmock.RegisterResponder("GET", "https://api.github.com/app/installations?per_page=100&page=0",
					httpmock.NewStringResponder(200, "invalid json"))
			},
			expectedCount: 0,
			expectedError: "unable to unmarshal response body",
		},
		{
			name:     "error_on_second_page",
			hostname: "api.github.com",
			jwt:      "test.jwt.token",
			setupMocks: func() {
				httpmock.RegisterResponder("GET", "https://api.github.com/app/installations?per_page=100&page=0",
					httpmock.NewStringResponder(200, string(fullPageJSON)))
				httpmock.RegisterResponder("GET", "https://api.github.com/app/installations?per_page=100&page=1",
					httpmock.NewStringResponder(500, `{"message": "Internal Server Error"}`))
			},
			expectedCount: 0,
			expectedError: "unexpected status code: 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mocks for each test
			httpmock.Reset()
			tt.setupMocks()

			// Execute the function
			result, err := listInstallations(tt.hostname, tt.jwt)

			// Assert results
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectedCount, len(*result))

				// Verify HTTP calls were made as expected
				info := httpmock.GetCallCountInfo()
				assert.Greater(t, len(info), 0, "Expected HTTP calls to be made")

				// Verify request headers for the first request
				if tt.expectedCount > 0 {
					// Check that the Authorization header was set correctly
					endpoint := fmt.Sprintf("https://%s/app/installations?per_page=100&page=0", tt.hostname)
					assert.Equal(t, 1, info[fmt.Sprintf("GET %s", endpoint)], "Expected exactly one call to first page")
				}
			}
		})
	}
}

func TestInstallationsPaginationBehavior(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// Test that pagination stops when we get less than 100 results
	// Create responses for multiple pages
	page0Response := make([]github.Installation, 100)
	for i := 0; i < 100; i++ {
		page0Response[i] = github.Installation{
			ID:      github.Int64(int64(i)),
			Account: &github.User{Login: github.String(fmt.Sprintf("user%d", i))},
		}
	}
	page0JSON, _ := json.Marshal(page0Response)

	page1Response := make([]github.Installation, 100)
	for i := 0; i < 100; i++ {
		page1Response[i] = github.Installation{
			ID:      github.Int64(int64(i + 100)),
			Account: &github.User{Login: github.String(fmt.Sprintf("user%d", i+100))},
		}
	}
	page1JSON, _ := json.Marshal(page1Response)

	// Page 2 has less than 100 results, should stop pagination
	page2Response := make([]github.Installation, 50)
	for i := 0; i < 50; i++ {
		page2Response[i] = github.Installation{
			ID:      github.Int64(int64(i + 200)),
			Account: &github.User{Login: github.String(fmt.Sprintf("user%d", i+200))},
		}
	}
	page2JSON, _ := json.Marshal(page2Response)

	t.Run("pagination_stops_when_less_than_100_results", func(t *testing.T) {
		httpmock.Reset()

		httpmock.RegisterResponder("GET", "https://api.github.com/app/installations?per_page=100&page=0",
			httpmock.NewStringResponder(200, string(page0JSON)))
		httpmock.RegisterResponder("GET", "https://api.github.com/app/installations?per_page=100&page=1",
			httpmock.NewStringResponder(200, string(page1JSON)))
		httpmock.RegisterResponder("GET", "https://api.github.com/app/installations?per_page=100&page=2",
			httpmock.NewStringResponder(200, string(page2JSON)))

		result, err := listInstallations("api.github.com", "test.jwt.token")

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 250, len(*result)) // 100 + 100 + 50

		// Verify all three pages were called
		info := httpmock.GetCallCountInfo()
		assert.Equal(t, 1, info["GET https://api.github.com/app/installations?per_page=100&page=0"])
		assert.Equal(t, 1, info["GET https://api.github.com/app/installations?per_page=100&page=1"])
		assert.Equal(t, 1, info["GET https://api.github.com/app/installations?per_page=100&page=2"])
		// Page 3 should not be called
		assert.Equal(t, 0, info["GET https://api.github.com/app/installations?per_page=100&page=3"])
	})
}

func TestInstallationsRequestHeaders(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	installationResponse := []github.Installation{
		{ID: github.Int64(12345)},
	}
	installationJSON, _ := json.Marshal(installationResponse)

	t.Run("correct_headers_are_set", func(t *testing.T) {
		httpmock.Reset()

		// Use a custom responder to check headers
		httpmock.RegisterResponder("GET", "https://api.github.com/app/installations?per_page=100&page=0",
			func(req *http.Request) (*http.Response, error) {
				// Verify headers
				assert.Equal(t, "Bearer test.jwt.token", req.Header.Get("Authorization"))
				assert.Equal(t, "application/vnd.github+json", req.Header.Get("Accept"))
				assert.Equal(t, "2022-11-28", req.Header.Get("X-GitHub-Api-Version"))
				assert.Equal(t, "Link-/gh-token", req.Header.Get("User-Agent"))

				return httpmock.NewStringResponse(200, string(installationJSON)), nil
			})

		result, err := listInstallations("api.github.com", "test.jwt.token")

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 1, len(*result))
	})
}
