package handlers_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	cerrors "github.com/oldmonad/ec2Drift/pkg/errors"
	"github.com/oldmonad/ec2Drift/pkg/logger"
	"github.com/oldmonad/ec2Drift/pkg/parser"
	"github.com/oldmonad/ec2Drift/pkg/ports"
	"github.com/oldmonad/ec2Drift/pkg/ports/rest/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

func TestMain(m *testing.M) {
	// Initialize test logger
	logger.SetLogger(zap.NewNop())
	code := m.Run()
	os.Exit(code)
}

type MockAppRunner struct {
	mock.Mock
}

func (m *MockAppRunner) Run(ctx context.Context, args []string, pt parser.ParserType, rt ports.Runtype) error {
	return m.Called(ctx, args, pt, rt).Error(0)
}

type MockValidator struct {
	mock.Mock
}

func (m *MockValidator) ValidateAttributes(requested []string) ([]string, error) {
	args := m.Called(requested)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockValidator) ValidateFormat(format string) (parser.ParserType, error) {
	args := m.Called(format)
	return args.Get(0).(parser.ParserType), args.Error(1)
}

func TestDriftHandler(t *testing.T) {
	t.Run("handle non-POST method", func(t *testing.T) {
		appMock := new(MockAppRunner)
		validatorMock := new(MockValidator)
		handler := handlers.NewDriftHandler(appMock, validatorMock)

		req := httptest.NewRequest("GET", "/drift", nil)
		w := httptest.NewRecorder()

		handler.HandleDrift(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		assert.JSONEq(t, `{"error":"Method not allowed"}`, w.Body.String())
	})

	t.Run("handle invalid JSON", func(t *testing.T) {
		appMock := new(MockAppRunner)
		validatorMock := new(MockValidator)
		handler := handlers.NewDriftHandler(appMock, validatorMock)

		req := httptest.NewRequest("POST", "/drift", bytes.NewReader([]byte(`{invalid}`)))
		w := httptest.NewRecorder()

		handler.HandleDrift(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid JSON")
	})

	t.Run("attribute validation failure", func(t *testing.T) {
		appMock := new(MockAppRunner)
		validatorMock := new(MockValidator)
		handler := handlers.NewDriftHandler(appMock, validatorMock)

		validationErr := cerrors.NewAttributeValidationError(
			&cerrors.InvalidAttributesError{
				InvalidAttrs: []string{"bad-attr"},
				ValidAttrs:   []string{"good-attr"},
			},
		)

		validatorMock.On("ValidateAttributes", []string{"bad-attr"}).
			Return([]string{}, validationErr)

		body := `{"attributes": ["bad-attr"], "format": "json"}`
		req := httptest.NewRequest("POST", "/drift", bytes.NewReader([]byte(body)))
		w := httptest.NewRecorder()

		handler.HandleDrift(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid attributes: [bad-attr]")
		validatorMock.AssertExpectations(t)
	})

	// t.Run("format validation failure", func(t *testing.T) {
	// 	appMock := new(MockAppRunner)
	// 	validatorMock := new(MockValidator)
	// 	handler := handlers.NewDriftHandler(appMock, validatorMock)

	// 	validationErr := cerrors.NewFormatValidationError(errors.New("invalid format"))
	// 	validatorMock.On("ValidateFormat", "invalid").
	// 		Return(parser.Unknown, validationErr)

	// 	body := `{"attributes": ["instance-id"], "format": "invalid"}`
	// 	req := httptest.NewRequest("POST", "/drift", bytes.NewReader([]byte(body)))
	// 	w := httptest.NewRecorder()

	// 	handler.HandleDrift(w, req)

	// 	assert.Equal(t, http.StatusBadRequest, w.Code)
	// 	assert.Contains(t, w.Body.String(), "format validation failed")
	// 	validatorMock.AssertExpectations(t)
	// })

	t.Run("drift detected", func(t *testing.T) {
		appMock := new(MockAppRunner)
		validatorMock := new(MockValidator)
		handler := handlers.NewDriftHandler(appMock, validatorMock)

		validatorMock.On("ValidateAttributes", []string{"instance-id"}).
			Return([]string{"instance-id"}, nil)
		validatorMock.On("ValidateFormat", "json").
			Return(parser.JSON, nil)
		appMock.On("Run", mock.Anything, []string{"instance-id"}, parser.JSON, ports.HTTP).
			Return(cerrors.ErrDriftDetected{})

		body := `{"attributes": ["instance-id"], "format": "json"}`
		req := httptest.NewRequest("POST", "/drift", bytes.NewReader([]byte(body)))
		w := httptest.NewRecorder()

		handler.HandleDrift(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.JSONEq(t, `{"drift_detected":true,"message":"Drift detected"}`, w.Body.String())
	})

	// t.Run("no EC2 instances error", func(t *testing.T) {
	// 	appMock := new(MockAppRunner)
	// 	validatorMock := new(MockValidator)
	// 	handler := handlers.NewDriftHandler(appMock, validatorMock)

	// 	validatorMock.On("ValidateAttributes", []string{"instance-id"}).
	// 		Return([]string{"instance-id"}, nil)
	// 	validatorMock.On("ValidateFormat", "json").
	// 		Return(parser.JSON, nil)
	// 	appMock.On("Run", mock.Anything, []string{"instance-id"}, parser.JSON, ports.HTTP).
	// 		Return(cerrors.ErrNoEC2Instances{})

	// 	body := `{"attributes": ["instance-id"], "format": "json"}`
	// 	req := httptest.NewRequest("POST", "/drift", bytes.NewReader([]byte(body)))
	// 	w := httptest.NewRecorder()

	// 	handler.HandleDrift(w, req)

	// 	assert.Equal(t, http.StatusBadRequest, w.Code)
	// 	assert.Contains(t, w.Body.String(), "no EC2 instances found")
	// })

	// t.Run("generic app error", func(t *testing.T) {
	// 	appMock := new(MockAppRunner)
	// 	validatorMock := new(MockValidator)
	// 	handler := handlers.NewDriftHandler(appMock, validatorMock)

	// 	validatorMock.On("ValidateAttributes", []string{"instance-id"}).
	// 		Return([]string{"instance-id"}, nil)
	// 	validatorMock.On("ValidateFormat", "json").
	// 		Return(parser.JSON, nil)
	// 	appMock.On("Run", mock.Anything, []string{"instance-id"}, parser.JSON, ports.HTTP).
	// 		Return(errors.New("database error"))

	// 	body := `{"attributes": ["instance-id"], "format": "json"}`
	// 	req := httptest.NewRequest("POST", "/drift", bytes.NewReader([]byte(body)))
	// 	w := httptest.NewRecorder()

	// 	handler.HandleDrift(w, req)

	// 	assert.Equal(t, http.StatusInternalServerError, w.Code)
	// 	assert.Contains(t, w.Body.String(), "application error")
	// })

	t.Run("successful execution with no drift", func(t *testing.T) {
		appMock := new(MockAppRunner)
		validatorMock := new(MockValidator)
		handler := handlers.NewDriftHandler(appMock, validatorMock)

		validatorMock.On("ValidateAttributes", []string{"instance-id"}).
			Return([]string{"instance-id"}, nil)
		validatorMock.On("ValidateFormat", "json").
			Return(parser.JSON, nil)
		appMock.On("Run", mock.Anything, []string{"instance-id"}, parser.JSON, ports.HTTP).
			Return(nil)

		body := `{"attributes": ["instance-id"], "format": "json"}`
		req := httptest.NewRequest("POST", "/drift", bytes.NewReader([]byte(body)))
		w := httptest.NewRecorder()

		handler.HandleDrift(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.JSONEq(t, `{"drift_detected":false,"message":"No drift detected"}`, w.Body.String())
	})
}
