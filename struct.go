package vies

import (
	"fmt"
	"net/http"
)

//const (
//	AvailabilityAvailable          Availability = "available"
//	AvailabilityUnavailable        Availability = "unavailable"
//	AvailabilityMonitoringDisabled Availability = "monitoring disabled"
//)

type HttpClientInterface interface {
	Do(req *http.Request) (*http.Response, error)
}

type BatchResponseHandlerInterface interface {
	Handle(content *[]byte) ([][]string, error)
}

type ApiError struct {
	Err     string
	Message string
}

func (e *ApiError) Error() string {
	return fmt.Sprintf("%v: %v", e.Err, e.Message)
}

type CheckResult struct {
	CountryCode string `json:"countryCode"`
	Address     string `json:"address"`
	VatNumber   string `json:"vatNumber"`
	Vat         string `json:"vat"`
	Valid       bool   `json:"valid"`
	Name        string `json:"name"`
	//Error       string `json:"error,omitempty"`
}

type batchToken struct {
	Token string `json:"token"`
}
type BatchStatus struct {
	Token      string  `json:"token"`
	Status     string  `json:"status"`
	Percentage float32 `json:"percentage"`
}

type Configuration struct {
	MaximumRowsForBatch     int `json:"maximumRowsForBatch"`
	MinimumRowsForBatch     int `json:"minimumRowsForBatch"`
	MaximumFileSizeForBatch int `json:"maximumFileSizeForBatch"`
}
type Status struct {
	Vow       StatusVow       `json:"vow"`
	Countries []CountryStatus `json:"countries"`
}

type StatusVow struct {
	Available bool `json:"available"`
}

type CountryStatus struct {
	CountryCode  string `json:"countryCode"`
	Availability string `json:"availability"`
}

type statusErrorResponse struct {
	ActionSucceed bool                 `json:"actionSucceed"`
	ErrorWrappers []statusErrorWrapper `json:"errorWrappers"`
}

type statusErrorWrapper struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type checkRequest struct {
	CountryCode string `json:"countryCode"`
	VatNumber   string `json:"vatNumber"`
}
