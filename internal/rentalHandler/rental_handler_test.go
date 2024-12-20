package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestRentComputer(t *testing.T) {
    // Setup Echo
    e := echo.New()

    // Mock request payload
    requestPayload := RentalRequest{
        CustomerID:   1,
        ComputerID:   1,
        AdminID:      1,
        RentalStart:  time.Now(),
        RentalEnd:    time.Now().Add(2 * time.Hour),
        ActivityDesc: "Test rental",
        Services: []ServiceEntry{
            {ServiceID: 1, Quantity: 2}, // Ensure this matches a service with sufficient quantity
        },
    }

    requestBody, _ := json.Marshal(requestPayload)

    // Mock request with a valid payment method
    req := httptest.NewRequest(http.MethodPost, "/rental?payment_method=wallet", bytes.NewReader(requestBody))
    req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

    // Create a new Echo context
    rec := httptest.NewRecorder()
    c := e.NewContext(req, rec)

    // Manually set the JWT claims
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "admin_id": float64(1), // Admin ID from the ddl.sql setup
        "role":     "super-admin",
    })
    c.Set("user", token)

    // Mock function call
    handler := RentComputer
    err := handler(c)

    // Assertions
    if assert.NoError(t, err) {
        assert.Equal(t, http.StatusOK, rec.Code)
        response := map[string]interface{}{}
        json.Unmarshal(rec.Body.Bytes(), &response)
        assert.Contains(t, response["message"], "Rental recorded successfully")
    }
}