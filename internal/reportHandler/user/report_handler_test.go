package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	// "time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	config "w4/p2/milestones/config/database"
)

func TestGetBookingReport(t *testing.T) {
	// Initialize test data
	setupTestData()

	// Setup Echo
	e := echo.New()

	// Mock request without query parameter
	req := httptest.NewRequest(http.MethodGet, "/booking-report", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Manually set the JWT claims for a customer
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"customer_id": float64(1), // Customer ID from the database
	})
	c.Set("user", token)

	// Call the handler function
	err := GetBookingReport(c)

	// Assertions for fetching full booking history
	if assert.NoError(t, err) {
		assert.Equal(t, http.StatusOK, rec.Code)

		// Parse the response body
		var response map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &response)

		// Verify response data
		assert.Equal(t, "Booking report retrieved successfully", response["message"])
		data := response["data"].([]interface{})
		assert.GreaterOrEqual(t, len(data), 1, "Booking report should contain at least one entry")
	}

	// Mock request with query parameter recent=true
	req = httptest.NewRequest(http.MethodGet, "/booking-report?recent=true", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)

	// Reuse the token for consistency
	c.Set("user", token)

	// Call the handler function for recent bookings
	err = GetBookingReport(c)

	// Assertions for fetching the most recent booking
	if assert.NoError(t, err) {
		assert.Equal(t, http.StatusOK, rec.Code)

		// Parse the response body
		var response map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &response)

		// Verify response data
		assert.Equal(t, "Booking report retrieved successfully", response["message"])
		data := response["data"].([]interface{})
		assert.Equal(t, 1, len(data), "Recent booking report should contain exactly one entry")
	}
}

// setupTestData sets up the database for testing
func setupTestData() {
	// Create mock data in the database
	_, _ = config.Pool.Exec(context.Background(), `
		INSERT INTO customer (name, username, email, password, wallet)
		VALUES ('Test Customer', 'testcustomer', 'test@example.com', 'hashed_password', 100000)
		ON CONFLICT DO NOTHING
	`)

	_, _ = config.Pool.Exec(context.Background(), `
		INSERT INTO computer (id, name, type, hourly_rate, isAvailable)
		VALUES (1, 'PC-001', 'Gaming', 20000, true)
		ON CONFLICT DO NOTHING
	`)

	_, _ = config.Pool.Exec(context.Background(), `
		INSERT INTO admin (id, username, password, role)
		VALUES (1, 'admin1', 'hashed_password', 'Manager')
		ON CONFLICT DO NOTHING
	`)

	_, _ = config.Pool.Exec(context.Background(), `
		INSERT INTO rental_history (id, customer_id, computer_id, admin_id, rental_start_time, rental_end_time, total_cost)
		VALUES (1, 1, 1, 1, '2024-12-18 10:00:00', '2024-12-18 12:00:00', 40000),
		       (2, 1, 1, 1, '2024-12-17 09:00:00', '2024-12-17 11:00:00', 30000)
		ON CONFLICT DO NOTHING
	`)
}
