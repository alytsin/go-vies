package vies

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	ViesEndpointUrl     = "https://ec.europa.eu/taxation_customs/vies/rest-api/"
	ViesCheckStatusPath = "check-status"
	ViesCheckVatPath    = "check-vat-number"
)

type ValidationError struct {
	Err     string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%v: %v", e.Err, e.Message)
}

type Validator struct {
	endpoint *url.URL
	client   *http.Client
}

type ValidatorInterface interface {
	Status(ctx context.Context) (*Status, error)
	Check(ctx context.Context, vat string) (*CheckResult, error)
	Valid(ctx context.Context, vat string) (bool, error)
}

func NewValidator(client *http.Client, endpoint string) (*Validator, error) {
	if endpoint == "" {
		endpoint = ViesEndpointUrl
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	if client == nil {
		client = http.DefaultClient
	}

	return &Validator{
		client:   client,
		endpoint: u,
	}, nil
}

func (v *Validator) Valid(ctx context.Context, vat string) (bool, error) {
	result, err := v.Check(ctx, vat)
	if err != nil {
		return false, err
	}
	return result.Valid, nil
}

func (v *Validator) Check(ctx context.Context, vat string) (*CheckResult, error) {

	if len(vat) < 2 {
		return nil, fmt.Errorf("invalid VAT provided %s", vat)
	}

	var status CheckResult
	reqBody := &checkRequest{
		CountryCode: strings.ToUpper(vat[0:2]),
		VatNumber:   vat[2:],
	}

	if err := v.doJSON(ctx, http.MethodPost, ViesCheckVatPath, reqBody, &status); err != nil {
		return nil, err
	}

	status.Vat = fmt.Sprintf("%s%s", status.CountryCode, status.VatNumber)

	return &status, nil
}

func (v *Validator) doError(body *[]byte) error {

	var e statusErrorResponse
	if err := json.Unmarshal(*body, &e); err != nil {
		return err
	}

	if len(e.ErrorWrappers) > 0 {
		err := e.ErrorWrappers[0]
		return &ValidationError{
			Err:     err.Error,
			Message: err.Message,
		}
	}

	return fmt.Errorf("invalid response structure")
}

func (v *Validator) Status(ctx context.Context) (*Status, error) {

	var status Status
	if err := v.doJSON(ctx, http.MethodGet, ViesCheckStatusPath, nil, &status); err != nil {
		return nil, err
	}

	return &status, nil
}

func (v *Validator) doJSON(ctx context.Context, method, path string, reqBody any, out any) error {
	var body io.Reader
	if reqBody != nil {
		reqBytes, err := json.Marshal(reqBody)
		if err != nil {
			return err
		}
		body = bytes.NewReader(reqBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, v.endpoint.JoinPath(path).String(), body)
	if err != nil {
		return err
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	rsp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()

	rspBody, err := io.ReadAll(rsp.Body)
	if err != nil {
		return err
	}

	if rsp.StatusCode == http.StatusOK {
		return json.Unmarshal(rspBody, out)
	}

	return v.doError(&rspBody)
}
