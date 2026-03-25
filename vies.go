package vies

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
)

const (
	apiEndpointUrl       = "https://ec.europa.eu/taxation_customs/vies/rest-api/"
	apiConfigurationPath = "configurations"
	apiCheckStatusPath   = "check-status"
	apiCheckVatPath      = "check-vat-number"
	apiBatchStatusPath   = "vat-validation"
	apiBatchReportPath   = "vat-validation-report"
)

var (
	batchFileColumns = []string{
		"MS Code",
		"VAT Number",
		"Requester MS Code",
		"Requester VAT Number",
	}
	batchRequiredColumns = map[string]string{
		"ms code":              "countryCode",
		"name":                 "name",
		"corrected vat number": "vatNumber",
		"valid":                "valid",
		"address":              "address",
		//"user error":           "error",
	}
)

type Client struct {
	endpoint             *url.URL
	httpClient           HttpClientInterface
	batchResponseHandler BatchResponseHandlerInterface
}

type ClientConfig struct {
	HttpClient           HttpClientInterface
	EndpointUrl          string
	BatchResponseHandler BatchResponseHandlerInterface
}

func NewClient(config *ClientConfig) (*Client, error) {

	var client HttpClientInterface
	var batchHandler BatchResponseHandlerInterface

	endpoint := apiEndpointUrl
	client = http.DefaultClient
	batchHandler = &SpreadsheetMlReader{}

	if config != nil {
		if config.EndpointUrl != "" {
			endpoint = config.EndpointUrl
		}
		if config.HttpClient != nil {
			client = config.HttpClient
		}
		if config.BatchResponseHandler != nil {
			batchHandler = config.BatchResponseHandler
		}
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	return &Client{
		endpoint:             u,
		httpClient:           client,
		batchResponseHandler: batchHandler,
	}, nil
}

func (client *Client) Configuration(ctx context.Context) (*Configuration, error) {
	var result Configuration
	if err := client.doJSON(ctx, http.MethodGet, apiConfigurationPath, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (client *Client) Batch(ctx context.Context, vats []string) (string, error) {

	if len(vats) == 0 {
		return "", fmt.Errorf("empty VAT list provided")
	}

	for _, vat := range vats {
		if err := client.isValidVat(vat); err != nil {
			return "", err
		}
	}

	var requestBody bytes.Buffer
	multipartWriter := multipart.NewWriter(&requestBody)
	part, err := multipartWriter.CreateFormFile("fileToUpload", "vat-numbers.csv")
	if err != nil {
		return "", err
	}

	csvWriter := csv.NewWriter(part)
	csvWriter.Comma = ','
	_ = csvWriter.Write(batchFileColumns)
	for _, vat := range vats {
		_ = csvWriter.Write([]string{vat[0:2], vat[2:]})
	}
	csvWriter.Flush()
	_ = multipartWriter.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint.JoinPath(apiBatchStatusPath).String(), &requestBody)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	rsp, err := client.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer rsp.Body.Close()

	rspBody, err := io.ReadAll(rsp.Body)
	if err != nil {
		return "", err
	}

	if rsp.StatusCode == http.StatusOK {
		var token batchToken
		if err := json.Unmarshal(rspBody, &token); err != nil {
			return "", err
		}
		return token.Token, nil
	}

	return "", client.doError(&rspBody)
}

func (client *Client) BatchStatus(ctx context.Context, token string) (*BatchStatus, error) {
	var result BatchStatus

	if token == "" {
		return nil, fmt.Errorf("empty token provided")
	}

	path := fmt.Sprintf("%s/%s", apiBatchStatusPath, token)
	if err := client.doJSON(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (client *Client) BatchReport(ctx context.Context, token string) ([]CheckResult, error) {

	if token == "" {
		return nil, fmt.Errorf("empty token provided")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, client.endpoint.JoinPath(fmt.Sprintf("%s/%s", apiBatchReportPath, token)).String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	rsp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	if rsp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(rsp.Body)
		if err != nil {
			return nil, err
		}
		return nil, client.doError(&body)
	}

	contentType := rsp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "spreadsheetml.sheet") {
		return nil, fmt.Errorf("unexpected response type:  %s", contentType)
	}
	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}

	data, err := client.batchResponseHandler.Handle(&body)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("no rows found in response")
	}

	headerMap := make(map[string]int)

	for i, col := range data[0] {
		t := strings.ToLower(col)
		if _, ok := batchRequiredColumns[t]; ok {
			headerMap[batchRequiredColumns[t]] = i
		}
	}

	if len(headerMap) != len(batchRequiredColumns) {
		return nil, fmt.Errorf("invalid response structure")
	}

	var results []CheckResult
	for _, row := range data[1:] {

		valid := strings.ToUpper(row[headerMap["valid"]]) == "YES"
		rec := CheckResult{
			CountryCode: row[headerMap["countryCode"]],
			VatNumber:   row[headerMap["vatNumber"]],
			Valid:       valid,
			//Error:       row[headerMap["error"]],
		}

		if valid {
			//rec.Error = ""
			rec.Name = row[headerMap["name"]]
			rec.Address = row[headerMap["address"]]
		}

		rec.Vat = fmt.Sprintf("%s%s", rec.CountryCode, rec.VatNumber)
		results = append(results, rec)
	}
	return results, nil

}

func (client *Client) Valid(ctx context.Context, vat string) (bool, error) {
	result, err := client.Check(ctx, vat)
	if err != nil {
		return false, err
	}
	return result.Valid, nil
}

func (client *Client) isValidVat(vat string) error {
	if len(vat) < 3 {
		return fmt.Errorf("invalid VAT provided %s", vat)
	}
	return nil
}

func (client *Client) Check(ctx context.Context, vat string) (*CheckResult, error) {

	if err := client.isValidVat(vat); err != nil {
		return nil, err
	}

	var status CheckResult
	reqBody := &checkRequest{
		CountryCode: strings.ToUpper(vat[0:2]),
		VatNumber:   vat[2:],
	}

	if err := client.doJSON(ctx, http.MethodPost, apiCheckVatPath, reqBody, &status); err != nil {
		return nil, err
	}

	status.Vat = fmt.Sprintf("%s%s", status.CountryCode, status.VatNumber)

	return &status, nil
}

func (client *Client) doError(body *[]byte) error {

	var e statusErrorResponse
	if err := json.Unmarshal(*body, &e); err != nil {
		return err
	}

	if len(e.ErrorWrappers) > 0 {
		err := e.ErrorWrappers[0]
		return &ApiError{
			Err:     err.Error,
			Message: err.Message,
		}
	}

	return fmt.Errorf("unexpected response structure")
}

func (client *Client) Status(ctx context.Context) (*Status, error) {
	var status Status
	if err := client.doJSON(ctx, http.MethodGet, apiCheckStatusPath, nil, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

func (client *Client) doJSON(ctx context.Context, method, path string, reqBody any, out any) error {
	var body io.Reader
	if reqBody != nil {
		reqBytes, err := json.Marshal(reqBody)
		if err != nil {
			return err
		}
		body = bytes.NewReader(reqBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, client.endpoint.JoinPath(path).String(), body)
	if err != nil {
		return err
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	rsp, err := client.httpClient.Do(req)
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

	return client.doError(&rspBody)
}
