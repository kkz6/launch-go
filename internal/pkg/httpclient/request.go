package httpclient

import (
	"bytes"
	"context"
	"encoding/base64"
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

// BearerAuth sets the Authorization header with a Bearer token.
func (r *Request) BearerAuth(token string) *Request {
	r.headers["Authorization"] = "Bearer " + token
	return r
}

// BasicAuth sets the Authorization header with Basic authentication.
func (r *Request) BasicAuth(username, password string) *Request {
	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	r.headers["Authorization"] = "Basic " + auth
	return r
}

// TokenAuth sets the Authorization header with a token scheme (e.g., "token abc123").
func (r *Request) TokenAuth(scheme, token string) *Request {
	r.headers["Authorization"] = scheme + " " + token
	return r
}

// JSON sets both Content-Type and Accept headers to application/json.
func (r *Request) JSON() *Request {
	r.headers["Content-Type"] = "application/json"
	r.headers["Accept"] = "application/json"
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

// HandleResponse reads and wraps an http.Response for easier handling.
// It closes the response body after reading.
func HandleResponse(resp *http.Response) (*Response, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Headers:    resp.Header,
		Body:       body,
	}, nil
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

// CheckSuccess returns an error if the response status code is not 2xx.
func (r *Response) CheckSuccess() error {
	if !r.IsSuccess() {
		return fmt.Errorf("request failed with status %d: %s", r.StatusCode, string(r.Body))
	}
	return nil
}

// Decode validates the response is successful and unmarshals the body into v.
func (r *Response) Decode(v interface{}) error {
	if err := r.CheckSuccess(); err != nil {
		return err
	}
	if len(r.Body) == 0 {
		return nil
	}
	return json.Unmarshal(r.Body, v)
}

// DecodeMap validates the response and returns the body as a map.
func (r *Response) DecodeMap() (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := r.Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// StatusHandler allows mapping specific status codes to custom errors.
type StatusHandler struct {
	handlers map[int]error
}

// NewStatusHandler creates a new StatusHandler for custom error mapping.
func NewStatusHandler() *StatusHandler {
	return &StatusHandler{handlers: make(map[int]error)}
}

// On maps a status code to a specific error.
func (h *StatusHandler) On(code int, err error) *StatusHandler {
	h.handlers[code] = err
	return h
}

// Check returns the mapped error for the response status code,
// or calls CheckSuccess if no mapping exists.
func (h *StatusHandler) Check(r *Response) error {
	if err, ok := h.handlers[r.StatusCode]; ok {
		return err
	}
	return r.CheckSuccess()
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

// RequestBuilder is a standalone builder for constructing http.Request objects
// without requiring an httpclient.Client. Use this when you need to build
// requests for use with a raw http.Client.
type RequestBuilder struct {
	ctx     context.Context
	method  string
	url     string
	headers map[string]string
	body    io.Reader
}

// NewRequest creates a new standalone request builder.
// This is useful when you need to build an http.Request without using httpclient.Client.
func NewRequest(ctx context.Context, method, requestURL string) *RequestBuilder {
	return &RequestBuilder{
		ctx:     ctx,
		method:  method,
		url:     requestURL,
		headers: make(map[string]string),
	}
}

// WithHeader adds a header to the request.
func (r *RequestBuilder) WithHeader(key, value string) *RequestBuilder {
	r.headers[key] = value
	return r
}

// WithHeaders adds multiple headers to the request.
func (r *RequestBuilder) WithHeaders(headers map[string]string) *RequestBuilder {
	for k, v := range headers {
		r.headers[k] = v
	}
	return r
}

// BearerAuth sets the Authorization header with a Bearer token.
func (r *RequestBuilder) BearerAuth(token string) *RequestBuilder {
	r.headers["Authorization"] = "Bearer " + token
	return r
}

// BasicAuth sets the Authorization header with Basic authentication.
func (r *RequestBuilder) BasicAuth(username, password string) *RequestBuilder {
	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	r.headers["Authorization"] = "Basic " + auth
	return r
}

// TokenAuth sets the Authorization header with a custom token scheme.
func (r *RequestBuilder) TokenAuth(scheme, token string) *RequestBuilder {
	r.headers["Authorization"] = scheme + " " + token
	return r
}

// JSON sets both Content-Type and Accept headers to application/json.
func (r *RequestBuilder) JSON() *RequestBuilder {
	r.headers["Content-Type"] = "application/json"
	r.headers["Accept"] = "application/json"
	return r
}

// WithBody sets the request body from an io.Reader.
func (r *RequestBuilder) WithBody(body io.Reader) *RequestBuilder {
	r.body = body
	return r
}

// JSONBody marshals the given value to JSON and sets it as the request body.
// Also sets Content-Type and Accept headers to application/json.
func (r *RequestBuilder) JSONBody(v interface{}) *RequestBuilder {
	data, err := json.Marshal(v)
	if err != nil {
		return r
	}
	r.body = bytes.NewReader(data)
	return r.JSON()
}

// Build constructs the http.Request with all configured options.
func (r *RequestBuilder) Build() (*http.Request, error) {
	req, err := http.NewRequestWithContext(r.ctx, r.method, r.url, r.body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range r.headers {
		req.Header.Set(k, v)
	}

	return req, nil
}

// MustBuild constructs the http.Request or panics if an error occurs.
func (r *RequestBuilder) MustBuild() *http.Request {
	req, err := r.Build()
	if err != nil {
		panic(err)
	}
	return req
}
