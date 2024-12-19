package handler

import (
	"context"
	// "fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	config "w4/p2/milestones/config/database"
)

// RentalRequest structure
type RentalRequest struct {
	CustomerID    int            `json:"customer_id" validate:"required"`
	ComputerID    int            `json:"computer_id" validate:"required"`
	AdminID       int            `json:"admin_id" validate:"required"`
	RentalStart   time.Time      `json:"rental_start" validate:"required"`
	RentalEnd     time.Time      `json:"rental_end" validate:"required"`
	Services      []ServiceEntry `json:"services"` // optional
	ActivityDesc  string         `json:"activity_description"`
}

// ServiceEntry structure for additional services
type ServiceEntry struct {
	ServiceID int `json:"service_id" validate:"required"`
	Quantity  int `json:"quantity" validate:"required"`
}

func RentComputer(c echo.Context) error {
	var req RentalRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "Invalid request"})
	}

	// Calculate rental duration and cost (assume hourly rate is fetched from DB)
	var hourlyRate int
	query := "SELECT hourly_rate FROM computer WHERE id = $1 AND isAvailable = TRUE"
	err := config.Pool.QueryRow(context.Background(), query, req.ComputerID).Scan(&hourlyRate)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "Computer not available"})
	}

	rentalDuration := req.RentalEnd.Sub(req.RentalStart).Hours()
	totalCost := int(rentalDuration) * hourlyRate

	// Insert into Rental_History
	var rentalHistoryID int
	rentalHistoryQuery := `
		INSERT INTO rental_history (customer_id, computer_id, admin_id, rental_start_time, rental_end_time, total_cost)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
	err = config.Pool.QueryRow(context.Background(), rentalHistoryQuery, req.CustomerID, req.ComputerID, req.AdminID, req.RentalStart, req.RentalEnd, totalCost).Scan(&rentalHistoryID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to record rental history"})
	}

	// Insert into Log
	logQuery := `
		INSERT INTO log (customer_id, computer_id, login_time, logout_time, activity_description)
		VALUES ($1, $2, $3, $4, $5)`
	_, err = config.Pool.Exec(context.Background(), logQuery, req.CustomerID, req.ComputerID, req.RentalStart, req.RentalEnd, req.ActivityDesc)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to log rental activity"})
	}

	// Insert into Rental_Services if there are additional services
	if len(req.Services) > 0 {
		for _, service := range req.Services {
			serviceQuery := `
				INSERT INTO rental_services (rental_history_id, service_id, quantity)
				VALUES ($1, $2, $3)`
			_, err = config.Pool.Exec(context.Background(), serviceQuery, rentalHistoryID, service.ServiceID, service.Quantity)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to log additional services"})
			}
		}
	}

	// Update Computer availability
	updateComputerQuery := "UPDATE computer SET isAvailable = FALSE WHERE id = $1"
	_, err = config.Pool.Exec(context.Background(), updateComputerQuery, req.ComputerID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to update computer availability"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":         "Rental recorded successfully",
		"rental_history":  rentalHistoryID,
		"total_cost":      totalCost,
		"rental_duration": rentalDuration,
	})
}
