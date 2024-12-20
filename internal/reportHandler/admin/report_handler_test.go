package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestGenerateRevenueReport(t *testing.T) {
	// Setup Echo
	e := echo.New()

	// Mock request payload
	requestPayload := RevenueReportRequest{
		StartDate: "2024-12-01",
		EndDate:   "2024-12-18",
	}

	requestBody, _ := json.Marshal(requestPayload)

	// Mock request
	req := httptest.NewRequest(http.MethodPost, "/revenue-report", bytes.NewReader(requestBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	// Create a new Echo context
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Manually set the JWT claims for a super-admin
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"admin_id": float64(1), // Admin ID from the database
		"role":     "super-admin",
	})
	c.Set("user", token)

	// Call the handler function
	err := GenerateRevenueReport(c)

	// Assertions
	if assert.NoError(t, err) {
		assert.Equal(t, http.StatusOK, rec.Code)

		// Parse the response body
		var response RevenueReportResponse
		json.Unmarshal(rec.Body.Bytes(), &response)

		// Check if response values are within expectations
		assert.GreaterOrEqual(t, response.TotalRevenue, float64(0), "Total revenue should be >= 0")
		assert.GreaterOrEqual(t, response.TotalTransactions, 0, "Total transactions should be >= 0")
		assert.NotNil(t, response.TopServices, "Top services should not be nil")
	}
}
