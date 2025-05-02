package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/oldmonad/ec2Drift/internal/app"
	cerrors "github.com/oldmonad/ec2Drift/pkg/errors"
	"github.com/oldmonad/ec2Drift/pkg/logger"
	"github.com/oldmonad/ec2Drift/pkg/ports"
	"github.com/oldmonad/ec2Drift/pkg/utils/validator"
	"go.uber.org/zap"
)

// DriftHandler handles HTTP requests for drift detection
type DriftHandler struct {
	app       app.AppRunner       // Application logic handler
	validator validator.Validator // Validator for inputs
}

// NewDriftHandler creates a new instance of DriftHandler
func NewDriftHandler(app app.AppRunner, validator validator.Validator) *DriftHandler {
	return &DriftHandler{app: app, validator: validator}
}

// HandleDrift processes the POST /drift endpoint
func (h *DriftHandler) HandleDrift(w http.ResponseWriter, r *http.Request) {
	logger.Log.Debug("Handling drift detection request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
	)

	// Only accept POST requests
	if r.Method != http.MethodPost {
		logger.Log.Warn("Invalid method attempted",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
		)
		sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Request payload structure
	var req struct {
		Attrs  []string `json:"attributes"` // Attributes to check for drift
		Format string   `json:"format"`     // Input format: terraform or json
	}

	// Parse and validate the request body
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Log.Error("Failed to decode request body",
			zap.Error(err),
			zap.String("path", r.URL.Path),
		)
		sendError(w, http.StatusBadRequest, cerrors.NewErrInvalidJSON(err).Error())
		return
	}

	logger.Log.Debug("Request parameters received",
		zap.Strings("attributes", req.Attrs),
		zap.String("format", req.Format),
	)

	// Validate the attributes
	validAttrs, err := h.validator.ValidateAttributes(req.Attrs)
	if err != nil {
		logger.Log.Warn("Attribute validation failed",
			zap.Error(err),
			zap.Strings("requested_attributes", req.Attrs),
		)
		sendError(w, http.StatusBadRequest, cerrors.NewAttributeValidationError(err).Error())
		return
	}

	// Validate the format type
	parserType, err := h.validator.ValidateFormat(req.Format)
	if err != nil {
		logger.Log.Warn("Format validation failed",
			zap.Error(err),
			zap.String("requested_format", req.Format),
		)
		sendError(w, http.StatusBadRequest, cerrors.NewFormatValidationError(err).Error())
		return
	}

	logger.Log.Info("Starting drift detection",
		zap.Strings("valid_attributes", validAttrs),
		zap.String("format", req.Format),
		zap.String("parser_type", string(parserType)),
	)

	// Run the main application logic for drift detection
	err = h.app.Run(r.Context(), validAttrs, parserType, ports.HTTP)
	if err != nil {
		switch {
		// Case when drift is detected
		case errors.As(err, &cerrors.ErrDriftDetected{}):
			logger.Log.Info("Drift detected in EC2 instances",
				zap.Strings("attributes", validAttrs),
				zap.String("format", req.Format),
			)
			sendResponse(w, http.StatusOK, map[string]interface{}{
				"drift_detected": true,
				"message":        "Drift detected",
			})

		// Case when no EC2 instances were found
		case errors.As(err, &cerrors.ErrNoEC2Instances{}):
			logger.Log.Warn("No EC2 instances found",
				zap.Error(err),
			)
			sendError(w, http.StatusBadRequest, err.Error())

		// Generic application error
		default:
			logger.Log.Error("Application error during drift detection",
				zap.Error(err),
				zap.Strings("attributes", validAttrs),
				zap.String("format", req.Format),
			)
			sendError(w, http.StatusInternalServerError, cerrors.NewErrAppRun(err).Error())
		}
		return
	}

	// If no drift is detected, return successful response
	logger.Log.Info("No drift detected in EC2 instances",
		zap.Strings("attributes", validAttrs),
		zap.String("format", req.Format),
	)
	sendResponse(w, http.StatusOK, map[string]interface{}{
		"drift_detected": false,
		"message":        "No drift detected",
	})
}

// sendError sends an error response with JSON payload
func sendError(w http.ResponseWriter, statusCode int, message string) {
	logger.Log.Debug("Sending error response",
		zap.Int("status_code", statusCode),
		zap.String("message", message),
	)
	sendResponse(w, statusCode, map[string]interface{}{
		"error": message,
	})
}

// sendResponse writes a JSON response with given status and data
func sendResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Log.Error("Failed to encode response",
			zap.Error(err),
			zap.Int("status_code", statusCode),
		)
	}
}
