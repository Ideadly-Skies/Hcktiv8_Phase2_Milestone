package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

var createdOrderID string // Global variable to store the order ID for testing

func TestCreatePayment(t *testing.T) {
	// Mock JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"customer_id": float64(1),
	})

	// Mock request payload
	payload := `{
		"amount": 50000,
		"purpose": "Top-Up"
	}`

	// Mock Echo context
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/payment/create", bytes.NewReader([]byte(payload)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user", token)

	// Initialize Midtrans client
	Init()

	// Call the handler
	err := CreatePayment(c)

	// Assertions
	if assert.NoError(t, err) {
		assert.Equal(t, http.StatusOK, rec.Code)

		// Parse and validate response
		var response map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &response)
		assert.Equal(t, "Payment request created", response["message"])
		assert.NotEmpty(t, response["transaction_id"])
		assert.NotEmpty(t, response["payment_url"])

		// Store the created order ID for use in TestCheckPaymentStatus
		createdOrderID = response["order_id"].(string)
	}
}

func TestCheckPaymentStatus(t *testing.T) {
    // Ensure `createdOrderID` is set from TestCreatePayment
    if createdOrderID == "" {
        t.Fatal("createdOrderID is empty. Ensure TestCreatePayment runs before TestCheckPaymentStatus.")
    }

    // Mock Echo context
    e := echo.New()
    req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/payment/status/%s", createdOrderID), nil)
    rec := httptest.NewRecorder()
    c := e.NewContext(req, rec)
    c.SetParamNames("orderID")
    c.SetParamValues(createdOrderID)

    // Call the handler
    err := CheckPaymentStatus(c)

    // Assertions
    if assert.NoError(t, err) {
        assert.Equal(t, http.StatusOK, rec.Code)

        // Parse and validate response
        var response map[string]interface{}
        json.Unmarshal(rec.Body.Bytes(), &response)
        assert.Equal(t, "pending", response["transaction_status"]) // Update expected value to "pending"
    }
}