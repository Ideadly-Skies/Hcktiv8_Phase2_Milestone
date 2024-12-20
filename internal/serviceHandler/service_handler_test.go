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

func TestPurchaseService(t *testing.T) {
	// Setup Echo
	e := echo.New()

	// Mock request payload
	requestPayload := ServiceRequest{
		CustomerID: 1,
		Services: []struct {
			ServiceID int `json:"service_id"`
			Quantity  int `json:"quantity"`
		}{
			{ServiceID: 1, Quantity: 2}, // Valid service
			{ServiceID: 2, Quantity: 1}, // Valid service
		},
		PaymentMethod: "wallet",
	}

	requestBody, _ := json.Marshal(requestPayload)

	// Mock request
	req := httptest.NewRequest(http.MethodPost, "/services/purchase", bytes.NewReader(requestBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	// Create a new Echo context
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Mock JWT claims for an admin user
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"role": "admin", // Role allowed to make purchases
	})
	c.Set("user", token)

	// Call the handler function
	err := PurchaseService(c)

	// Assertions
	if assert.NoError(t, err) {
		assert.Equal(t, http.StatusOK, rec.Code)

		// Parse the response body
		var response map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &response)

		// Validate response data
		assert.Equal(t, "Services purchased successfully", response["message"])
		assert.Greater(t, response["total_cost"].(float64), float64(0), "Total cost should be greater than 0")
	}
}