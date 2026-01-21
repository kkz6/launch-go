package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Request is a builder for constructing HTTP requests.
type Request struct {
	client      *Client
	method      string
	path        string
	query       url.Values
	headers     map[string]string
	body        interface{}
	rawBody     io.Reader
	contentType string
}

// NewRequest creates a new request builder.
func (c *Client) Request(method, path string) *Request {
	return &Request{
		client:  c,
		method:  method,
		path:    path,
		query:   make(url.Values),
		headers: make(map[string]string),
	}
}

// GET creates a new GET request builder.
func (c *Client) GET(path string) *Request {
	return c.Request(http.MethodGet, path)
}

// POST creates a new POST request builder.
func (c *Client) POST(path string) *Request {
	return c.Request(http.MethodPost, path)
}

// PUT creates a new PUT request builder.
func (c *Client) PUT(path string) *Request {
	return c.Request(http.MethodPut, path)
}

// PATCH creates a new PATCH request builder.
func (c *Client) PATCH(path string) *Request {
	return c.Request(http.MethodPatch, path)
}

// DELETE creates a new DELETE request builder.
func (c *Client) DELETE(path string) *Request {
	return c.Request(http.MethodDelete, path)
}

// WithQuery adds a query parameter.
func (r *Request) WithQuery(key, value string) *Request {
	r.query.Add(key, value)
	return r
}

// WithQueryParams adds multiple query parameters.
func (r *Request) WithQueryParams(params map[string]string) *Request {
	for k, v := range params {
		r.query.Add(k, v)
	}
	return r
}

// WithHeader adds a header to the request.
func (r *Request) WithHeader(key, value string) *Request {
	r.headers[key] = value
	return r
}

// WithHeaders adds multiple headers to the request.
func (r *Request) WithHeaders(headers map[string]string) *Request {
	for k, v := range headers {
		r.headers[k] = v
	}
	return r
}

// WithBody sets the request body (will be JSON encoded).
func (r *Request) WithBody(body interface{}) *Request {
	r.body = body
	return r
}

// WithRawBody sets a raw body reader.
func (r *Request) WithRawBody(body io.Reader, contentType string) *Request {
	r.rawBody = body
	r.contentType = contentType
	return r
}

// WithJSONBody sets the request body with explicit JSON content type.
func (r *Request) WithJSONBody(body interface{}) *Request {
	r.body = body
	r.contentType = "application/json"
	return r
}

// WithFormBody sets a form-encoded request body.
func (r *Request) WithFormBody(data url.Values) *Request {
	r.rawBody = strings.NewReader(data.Encode())
	r.contentType = "application/x-www-form-urlencoded"
	return r
}

// Build constructs the HTTP request.
func (r *Request) Build(ctx context.Context) (*http.Request, error) {
	// Build URL with query params
	reqURL := r.client.baseURL + r.path
	if len(r.query) > 0 {
		reqURL += "?" + r.query.Encode()
	}

	// Build body
	var bodyReader io.Reader
	if r.rawBody != nil {
		bodyReader = r.rawBody
	} else if r.body != nil {
		jsonBody, err := json.Marshal(r.body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = &bytesReader{data: jsonBody}
	}

	req, err := http.NewRequestWithContext(ctx, r.method, reqURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set client headers
	for k, v := range r.client.headers {
		req.Header.Set(k, v)
	}

	// Set authentication
	if r.client.authToken != "" {
		req.Header.Set("Authorization", r.client.authScheme+" "+r.client.authToken)
	}

	// Set request-specific headers (override client headers)
	for k, v := range r.headers {
		req.Header.Set(k, v)
	}

	// Set content type if specified
	if r.contentType != "" {
		req.Header.Set("Content-Type", r.contentType)
	}

	return req, nil
}

// Do executes the request and returns the result.
func (r *Request) Do(ctx context.Context, result interface{}) error {
	req, err := r.Build(ctx)
	if err != nil {
		return err
	}
	return r.client.DoRequest(req, result)
}

// DoRaw executes the request and returns the raw response.
// The caller is responsible for closing the response body.
func (r *Request) DoRaw(ctx context.Context) (*http.Response, error) {
	req, err := r.Build(ctx)
	if err != nil {
		return nil, err
	}

	var resp *http.Response
	if r.client.retryOpts != nil {
		resp, err = r.client.doWithRetry(req)
	} else {
		resp, err = r.client.httpClient.Do(req)
	}

	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// DoBytes executes the request and returns the response body as bytes.
func (r *Request) DoBytes(ctx context.Context) ([]byte, error) {
	resp, err := r.DoRaw(ctx)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &HTTPError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(body),
		}
	}

	return body, nil
}

// Response wraps an HTTP response with additional helpers.
type Response struct {
	StatusCode int
	Status     string
	Headers    http.Header
	Body       []byte
}

// JSON unmarshals the response body into the given target.
func (r *Response) JSON(target interface{}) error {
	if len(r.Body) == 0 {
		return nil
	}
	return json.Unmarshal(r.Body, target)
}

// String returns the response body as a string.
func (r *Response) String() string {
	return string(r.Body)
}

// IsSuccess returns true if the response status code is 2xx.
func (r *Response) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// DoResponse executes the request and returns a Response wrapper.
func (r *Request) DoResponse(ctx context.Context) (*Response, error) {
	resp, err := r.DoRaw(ctx)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Headers:    resp.Header,
		Body:       body,
	}, nil
}

// JSON helper functions for common operations

// MarshalJSON marshals the given value to JSON.
func MarshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// UnmarshalJSON unmarshals JSON data into the given target.
func UnmarshalJSON(data []byte, target interface{}) error {
	return json.Unmarshal(data, target)
}

// DecodeJSONResponse decodes a JSON response body into the given target.
func DecodeJSONResponse(resp *http.Response, target interface{}) error {
	if resp.Body == nil {
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if len(body) == 0 {
		return nil
	}

	return json.Unmarshal(body, target)
}

// ToMap converts a struct to a map[string]interface{}.
func ToMap(v interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}
