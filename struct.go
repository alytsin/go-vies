package vies

import (
	"encoding/json"
	"fmt"
	"strings"
)

type CheckResult struct {
	CountryCode string `json:"countryCode"`
	VatNumber   string `json:"vatNumber"`
	Vat         string `json:"vat"`
	Valid       bool   `json:"valid"`
	Name        string `json:"name"`
}

type Status struct {
	Vow       StatusVow       `json:"vow"` // vow stands for VIES on the Web
	Countries []CountryStatus `json:"countries"`
}

type StatusVow struct {
	Available bool `json:"available"`
}

type Availability string

const (
	AvailabilityAvailable          Availability = "available"
	AvailabilityUnavailable        Availability = "unavailable"
	AvailabilityMonitoringDisabled Availability = "monitoring disabled"
)

func (a *Availability) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	value = strings.ToLower(value)

	switch Availability(value) {
	case AvailabilityAvailable,
		AvailabilityUnavailable,
		AvailabilityMonitoringDisabled:
		*a = Availability(value)
		return nil
	default:
		return fmt.Errorf("invalid availability: %q", value)
	}
}

func (a Availability) MarshalJSON() ([]byte, error) {
	switch a {
	case AvailabilityAvailable,
		AvailabilityUnavailable,
		AvailabilityMonitoringDisabled:
		return json.Marshal(string(a))
	default:
		return nil, fmt.Errorf("invalid availability: %q", a)
	}
}

type CountryStatus struct {
	CountryCode  string       `json:"countryCode"`
	Availability Availability `json:"availability"`
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
