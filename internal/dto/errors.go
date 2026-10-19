package dto

import "encoding/json"

type AppError struct {
	Code            string `json:"code"`
	Message         string `json:"message"`
	Field           string `json:"field,omitempty"`
	ExistingAssetID string `json:"existingAssetId,omitempty"`
	Details         string `json:"details,omitempty"`
}

func (e *AppError) Error() string {
	b, _ := json.Marshal(e)
	return string(b)
}

func Error(code, message string) error {
	return &AppError{Code: code, Message: message}
}
