package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	config "w4/p2/milestones/config/database"

	"github.com/golang-jwt/jwt/v4"
	"github.com/labstack/echo/v4"
)

// BookingReport structure
type BookingReport struct {
	RentalID       int     `json:"rental_id"`
	ComputerID     int     `json:"computer_id"`
	ComputerName   string  `json:"computer_name"`
	AdminID        int     `json:"admin_id"`
	AdminUsername  string  `json:"admin_username"`
	RentalStart    time.Time  `json:"rental_start"`
	RentalEnd      time.Time  `json:"rental_end"`
	TotalCost      float64 `json:"total_cost"`
}

// GetBookingReport godoc
// @Summary Get booking report
// @Description Retrieve a customer's booking history, including details of rentals and costs. Optionally fetch only the most recent booking.
// @Tags Reports
// @Accept json
// @Produce json
// @Param recent query string false "If set to 'true', fetches only the most recent booking"
// @Success 200 {object} map[string]interface{} "Booking report retrieved successfully"
// @Failure 500 {object} map[string]string "Failed to fetch or process booking report data"
// @Security BearerAuth
// @Router /booking-report [get]
func GetBookingReport(c echo.Context) error {
	// Extract customer ID from the JWT claims
	user := c.Get("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	customerID := int(claims["customer_id"].(float64))

	// check for the optional "recent" query parameter
	recent := c.QueryParam("recent")

	// Query rental history for the customer
	query := `
		SELECT rh.id AS rental_id, rh.computer_id, c.name AS computer_name, rh.admin_id,
		       a.username AS admin_username, rh.rental_start_time, rh.rental_end_time, rh.total_cost
		FROM rental_history rh
		LEFT JOIN computer c ON rh.computer_id = c.id
		LEFT JOIN admin a ON rh.admin_id = a.id
		WHERE rh.customer_id = $1
	`

	// Modify the query if the "recent" parameter is provided
	if recent == "true" {
		query += `
			ORDER BY rh.rental_start_time DESC
			LIMIT 1
		`
	} else {
		query += `
			ORDER BY rh.rental_start_time DESC
		`
	}

	rows, err := config.Pool.Query(context.Background(), query, customerID)
	if err != nil {
		fmt.Printf("Query error: %v\n", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to fetch booking report"})
	}
	defer rows.Close()

	// Populate the booking report
	var reports []BookingReport
	for rows.Next() {
		var report BookingReport
		if err := rows.Scan(
			&report.RentalID, &report.ComputerID, &report.ComputerName, &report.AdminID,
			&report.AdminUsername, &report.RentalStart, &report.RentalEnd, &report.TotalCost,
		); err != nil {
			fmt.Printf("Scan error: %v\n", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to process booking report data"})
		}
		reports = append(reports, report)
	}

	// Check for errors after iteration
	if err := rows.Err(); err != nil {
		fmt.Printf("Row iteration error: %v\n", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to process booking report data"})
	}

	// Return the booking report
	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Booking report retrieved successfully",
		"data":    reports,
	})
}