package dto

import "time"

// ErrorResponse represents a standardized error response returned by the API.
//
// Fields:
//   - message: A human-readable description of the error.
//   - error: A more technical detail of the error (optional, omitted if empty).
//   - timestamp: Time when the error occurred, useful for debugging and correlation.
//
// swagger:model ErrorResponse
type ErrorResponse struct {
	Message      string    `json:"message" example:"Something went wrong"`
	ErrorDetails string    `json:"error,omitempty" example:"internal server error"`
	Timestamp    time.Time `json:"timestamp" example:"2025-08-02T15:04:05Z07:00"`
}

// Error implements the error interface for ErrorResponse.
//
// Returns:
//   - string: A string representing the full error message.
//     If ErrorDetails is present, it concatenates it with Message.
func (e ErrorResponse) Error() string {
	if e.ErrorDetails != "" {
		return e.Message + ": " + e.ErrorDetails
	}
	return e.Message
}

// NewErrorResponse constructs an ErrorResponse with a message and optional technical error.
//
// Parameters:
//   - message: a human-readable message.
//   - err: the technical error (optional, may be nil).
//
// Returns:
//   - ErrorResponse: enriched with timestamp and technical details if provided.
func NewErrorResponse(message string, err error) ErrorResponse {
	errorDetails := ""
	if err != nil {
		errorDetails = err.Error()
	}

	return ErrorResponse{
		Message:      message,
		ErrorDetails: errorDetails,
		Timestamp:    time.Now(),
	}
}
