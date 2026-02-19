package vies

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func NewTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: RoundTripFunc(fn),
	}
}

func TestParseError(t *testing.T) {

	cases := []struct {
		name string
		body []byte
		want error
	}{
		{
			"invalid json",
			[]byte(`!`),
			errors.New("invalid character '!' looking for beginning of value"),
		},
		{
			"invalid response structure",
			[]byte(`{}`),
			errors.New("invalid response structure"),
		},
		{
			"valid error",
			[]byte(`{"errorWrappers": [{"error": "err", "message": "msg"}]}`),
			errors.New("err: msg"),
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			v := &Validator{}
			err := v.doError(&tt.body)
			assert.Error(t, err)
			assert.Equal(t, tt.want.Error(), err.Error())

		})
	}

}

func TestAvailabilityMarshalJSON(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		data, err := json.Marshal(AvailabilityAvailable)
		assert.NoError(t, err)
		assert.Equal(t, `"available"`, string(data))
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := json.Marshal(Availability("bogus"))
		assert.Error(t, err)
		assert.Equal(t, `json: error calling MarshalJSON for type vies.Availability: invalid availability: "bogus"`, err.Error())
	})
}

func TestAvailabilityUnmarshalJSON(t *testing.T) {
	t.Run("valid values", func(t *testing.T) {
		cases := []struct {
			name  string
			input string
			want  Availability
		}{
			{
				name:  "available mixed case",
				input: `"Available"`,
				want:  AvailabilityAvailable,
			},
			{
				name:  "unavailable lower",
				input: `"unavailable"`,
				want:  AvailabilityUnavailable,
			},
			{
				name:  "monitoring disabled mixed case",
				input: `"Monitoring Disabled"`,
				want:  AvailabilityMonitoringDisabled,
			},
		}

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				var v Availability
				err := json.Unmarshal([]byte(tt.input), &v)
				assert.NoError(t, err)
				assert.Equal(t, tt.want, v)
			})
		}
	})

	t.Run("invalid value", func(t *testing.T) {
		var v Availability
		err := json.Unmarshal([]byte(`"bogus"`), &v)
		assert.Error(t, err)
		assert.Equal(t, `invalid availability: "bogus"`, err.Error())
	})

	t.Run("invalid type", func(t *testing.T) {
		var v Availability
		err := json.Unmarshal([]byte(`123`), &v)
		assert.Error(t, err)
		assert.Equal(t, "json: cannot unmarshal number into Go value of type string", err.Error())
	})
}

func TestNewValidator(t *testing.T) {
	t.Run("default endpoint and client", func(t *testing.T) {
		v, err := NewValidator(nil, "")
		assert.NoError(t, err)
		assert.NotNil(t, v)
		assert.Equal(t, http.DefaultClient, v.client)
		assert.Equal(t, ViesEndpointUrl, v.endpoint.String())
	})

	t.Run("custom endpoint and client", func(t *testing.T) {
		client := &http.Client{}
		endpoint := "https://example.com/api/"
		v, err := NewValidator(client, endpoint)
		assert.NoError(t, err)
		assert.NotNil(t, v)
		assert.Equal(t, client, v.client)
		assert.Equal(t, endpoint, v.endpoint.String())
	})

	t.Run("invalid endpoint", func(t *testing.T) {
		v, err := NewValidator(&http.Client{}, "http://[::1")
		assert.Nil(t, v)
		assert.Error(t, err)
	})
}

func TestValidatorDoJSON(t *testing.T) {
	type requestPayload struct {
		A string `json:"a"`
		B int    `json:"b"`
	}

	type responsePayload struct {
		OK bool `json:"ok"`
	}

	t.Run("success with request body", func(t *testing.T) {
		client := NewTestClient(func(req *http.Request) *http.Response {
			assert.Equal(t, http.MethodPost, req.Method)
			assert.Equal(t, "https://example.com/api/do", req.URL.String())
			assert.Equal(t, "application/json", req.Header.Get("Content-Type"))

			body, err := io.ReadAll(req.Body)
			assert.NoError(t, err)
			assert.Equal(t, `{"a":"value","b":42}`, string(body))

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true}`)),
				Header:     make(http.Header),
			}
		})

		v, err := NewValidator(client, "https://example.com/api/")
		assert.NoError(t, err)

		var out responsePayload
		err = v.doJSON(context.Background(), http.MethodPost, "do", requestPayload{A: "value", B: 42}, &out)
		assert.NoError(t, err)
		assert.Equal(t, responsePayload{OK: true}, out)
	})

	t.Run("success without request body", func(t *testing.T) {
		client := NewTestClient(func(req *http.Request) *http.Response {
			assert.Equal(t, http.MethodGet, req.Method)
			assert.Equal(t, "https://example.com/api/ping", req.URL.String())
			assert.Equal(t, "", req.Header.Get("Content-Type"))

			if req.Body != nil {
				body, err := io.ReadAll(req.Body)
				assert.NoError(t, err)
				assert.Empty(t, string(body))
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true}`)),
				Header:     make(http.Header),
			}
		})

		v, err := NewValidator(client, "https://example.com/api/")
		assert.NoError(t, err)

		var out responsePayload
		err = v.doJSON(context.Background(), http.MethodGet, "ping", nil, &out)
		assert.NoError(t, err)
		assert.Equal(t, responsePayload{OK: true}, out)
	})

	t.Run("non-200 returns validation error", func(t *testing.T) {
		client := NewTestClient(func(req *http.Request) *http.Response {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Body: io.NopCloser(bytes.NewBufferString(
					`{"errorWrappers":[{"error":"err","message":"msg"}]}`,
				)),
				Header: make(http.Header),
			}
		})

		v, err := NewValidator(client, "https://example.com/api/")
		assert.NoError(t, err)

		var out responsePayload
		err = v.doJSON(context.Background(), http.MethodGet, "ping", nil, &out)
		assert.Error(t, err)
		assert.Equal(t, "err: msg", err.Error())

		var vErr *ValidationError
		assert.ErrorAs(t, err, &vErr)
	})

	t.Run("non-200 with invalid error payload", func(t *testing.T) {
		client := NewTestClient(func(req *http.Request) *http.Response {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewBufferString(`!`)),
				Header:     make(http.Header),
			}
		})

		v, err := NewValidator(client, "https://example.com/api/")
		assert.NoError(t, err)

		var out responsePayload
		err = v.doJSON(context.Background(), http.MethodGet, "ping", nil, &out)
		assert.Error(t, err)
		assert.Equal(t, "invalid character '!' looking for beginning of value", err.Error())
	})

	t.Run("request body marshal error", func(t *testing.T) {
		client := NewTestClient(func(req *http.Request) *http.Response {
			t.Fatalf("unexpected request: %v", req)
			return nil
		})

		v, err := NewValidator(client, "https://example.com/api/")
		assert.NoError(t, err)

		var out responsePayload
		err = v.doJSON(context.Background(), http.MethodPost, "do", func() {}, &out)
		assert.Error(t, err)
		assert.Equal(t, "json: unsupported type: func()", err.Error())
	})
}

func TestValidatorCheck(t *testing.T) {
	t.Run("valid vat", func(t *testing.T) {
		client := NewTestClient(func(req *http.Request) *http.Response {
			assert.Equal(t, http.MethodPost, req.Method)
			assert.Equal(t, "https://example.com/api/check-vat-number", req.URL.String())
			assert.Equal(t, "application/json", req.Header.Get("Content-Type"))

			body, err := io.ReadAll(req.Body)
			assert.NoError(t, err)
			assert.Equal(t, `{"countryCode":"EE","vatNumber":"123"}`, string(body))

			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewBufferString(
					`{"countryCode":"EE","vatNumber":"123","valid":true,"name":"Acme"}`,
				)),
				Header: make(http.Header),
			}
		})

		v, err := NewValidator(client, "https://example.com/api/")
		assert.NoError(t, err)

		result, err := v.Check(context.Background(), "ee123")
		assert.NoError(t, err)
		assert.Equal(t, &CheckResult{
			CountryCode: "EE",
			VatNumber:   "123",
			Vat:         "EE123",
			Valid:       true,
			Name:        "Acme",
		}, result)
	})

	t.Run("invalid vat length", func(t *testing.T) {
		v, err := NewValidator(NewTestClient(func(req *http.Request) *http.Response {
			t.Fatalf("unexpected request: %v", req)
			return nil
		}), "https://example.com/api/")
		assert.NoError(t, err)

		result, err := v.Check(context.Background(), "E")
		assert.Nil(t, result)
		assert.Error(t, err)
		assert.Equal(t, "invalid VAT provided E", err.Error())
	})

	t.Run("server error", func(t *testing.T) {
		client := NewTestClient(func(req *http.Request) *http.Response {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Body: io.NopCloser(bytes.NewBufferString(
					`{"errorWrappers":[{"error":"err","message":"msg"}]}`,
				)),
				Header: make(http.Header),
			}
		})

		v, err := NewValidator(client, "https://example.com/api/")
		assert.NoError(t, err)

		result, err := v.Check(context.Background(), "EE123")
		assert.Nil(t, result)
		assert.Error(t, err)
		assert.Equal(t, "err: msg", err.Error())
	})
}

func TestValidatorValid(t *testing.T) {
	t.Run("true when check valid", func(t *testing.T) {
		client := NewTestClient(func(req *http.Request) *http.Response {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewBufferString(
					`{"countryCode":"EE","vatNumber":"123","valid":true,"name":"Acme"}`,
				)),
				Header: make(http.Header),
			}
		})

		v, err := NewValidator(client, "https://example.com/api/")
		assert.NoError(t, err)

		ok, err := v.Valid(context.Background(), "EE123")
		assert.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("returns error from check", func(t *testing.T) {
		v, err := NewValidator(NewTestClient(func(req *http.Request) *http.Response {
			t.Fatalf("unexpected request: %v", req)
			return nil
		}), "https://example.com/api/")
		assert.NoError(t, err)

		ok, err := v.Valid(context.Background(), "E")
		assert.False(t, ok)
		assert.Error(t, err)
		assert.Equal(t, "invalid VAT provided E", err.Error())
	})
}

func TestViesCheck(t *testing.T) {

	cases := []struct {
		name     string
		httpCode int
		response string
		err      error
		status   *Status
	}{
		{
			name:     "valid response",
			httpCode: 200,
			response: `{"vow": {"available": true},"countries": [{"countryCode": "EE","availability": "Available"}]}`,
			status: &Status{
				Vow:       StatusVow{Available: true},
				Countries: []CountryStatus{{CountryCode: "EE", Availability: AvailabilityAvailable}},
			},
		},
		{
			name:     "invalid response",
			httpCode: 200,
			response: `!`,
			err:      errors.New("invalid character '!' looking for beginning of value"),
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {

			client := NewTestClient(func(req *http.Request) *http.Response {
				assert.Contains(t, req.URL.String(), ViesCheckStatusPath)
				return &http.Response{
					StatusCode: tt.httpCode,
					Body:       io.NopCloser(bytes.NewBufferString(tt.response)),
					Header:     make(http.Header),
				}
			})

			v, err := NewValidator(client, "")
			assert.NoError(t, err)
			assert.NotNil(t, v)

			result, err := v.Status(context.Background())
			if tt.err != nil {
				assert.Nil(t, result)
				assert.Error(t, err)
				assert.Equal(t, tt.err.Error(), err.Error())
			}
			if tt.status != nil {
				assert.NotNil(t, result)
				assert.Equal(t, tt.status, result)
				assert.NoError(t, err)
			}

		})
	}

}
