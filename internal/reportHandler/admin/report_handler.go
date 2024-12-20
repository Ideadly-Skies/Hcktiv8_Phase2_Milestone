package handler

import (
	"context"
	"fmt"
	// "fmt"
	"net/http"
	"time"
	"encoding/json"

	"github.com/golang-jwt/jwt/v4"
	"github.com/labstack/echo/v4"

	config "w4/p2/milestones/config/database"
)

// RevenueReportRequest defines the request payload for the report
type RevenueReportRequest struct {
	StartDate string `json:"start_date" validate:"required"`
	EndDate   string `json:"end_date" validate:"required"`
}

// RevenueReportResponse defines the structure of the response
type RevenueReportResponse struct {
	TotalRevenue      float64 `json:"total_revenue"`
	TotalTransactions int     `json:"total_transactions"`
	TopServices       []struct {
		ServiceName  string `json:"service_name"`
		TotalRevenue float64 `json:"total_revenue"`
		TotalSold    int     `json:"total_sold"`
	} `json:"top_services"`
}

// GenerateRevenueReport godoc
// @Summary Generate revenue report
// @Description Allows super-admins to generate a revenue report for a specified date range, including total revenue, transactions, and top services.
// @Tags Reports
// @Accept json
// @Produce json
// @Param body body RevenueReportRequest true "Request body with start_date and end_date"
// @Success 200 {object} RevenueReportResponse
// @Failure 400 {object} map[string]string "Invalid request payload or date format"
// @Failure 403 {object} map[string]string "Unauthorized action. Only super-admins can generate reports."
// @Failure 500 {object} map[string]string "Failed to generate report or perform internal operation"
// @Security BearerAuth
// @Router /revenue-report [post]
func GenerateRevenueReport(c echo.Context) error {
	// Bind and validate request payload
	var req RevenueReportRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "Invalid request"})
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "Invalid start date format"})
	}

	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "Invalid end date format"})
	}

	// Extract user details from JWT
	user := c.Get("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	adminRole := claims["role"].(string)
	adminID := int(claims["admin_id"].(float64))

	// Check if the user is a super-admin
	if adminRole != "super-admin" {
		return c.JSON(http.StatusForbidden, map[string]string{
			"message": "Unauthorized. Only super-admins can generate reports.",
		})
	}

	// Query total revenue and total transactions
	var totalRevenue float64
	var totalTransactions int
	query := `
		SELECT SUM(amount) AS total_revenue, COUNT(*) AS total_transactions
		FROM transaction
		WHERE transaction_date BETWEEN $1 AND $2 AND status ILIKE 'Settlement'`
	err = config.Pool.QueryRow(context.Background(), query, startDate, endDate).Scan(&totalRevenue, &totalTransactions)
	fmt.Println("error: ", err)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to generate report"})
	}

	// Query top services
	topServicesQuery := `
		SELECT s.name, SUM(rs.quantity * s.price) AS total_revenue, SUM(rs.quantity) AS total_sold
		FROM rental_services rs
		JOIN service s ON rs.service_id = s.id
		WHERE rs.created_at BETWEEN $1 AND $2
		GROUP BY s.id
		ORDER BY total_revenue DESC
		LIMIT 5`
	rows, err := config.Pool.Query(context.Background(), topServicesQuery, startDate, endDate)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to fetch top services"})
	}
	defer rows.Close()

	var topServices []struct {
		ServiceName  string  `json:"service_name"`
		TotalRevenue float64 `json:"total_revenue"`
		TotalSold    int     `json:"total_sold"`
	}

	for rows.Next() {
		var service struct {
			ServiceName  string  `json:"service_name"`
			TotalRevenue float64 `json:"total_revenue"`
			TotalSold    int     `json:"total_sold"`
		}
		err = rows.Scan(&service.ServiceName, &service.TotalRevenue, &service.TotalSold)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to parse top services"})
		}
		topServices = append(topServices, service)
	}

	// Insert report into the report table
	reportQuery := `
		INSERT INTO report (admin_id, report_type, start_date, end_date, total_transactions, total_revenue, top_services, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())`
	topServicesJSON, err := json.Marshal(topServices)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to serialize top services"})
	}

	_, err = config.Pool.Exec(context.Background(), reportQuery, adminID, "Revenue Report", startDate, endDate, totalTransactions, totalRevenue, string(topServicesJSON))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to save report"})
	}

	// Log the admin action in the log table
	logQuery := `
		INSERT INTO log (description)
		VALUES ($1)`
	logDescription := fmt.Sprintf("Super-admin (ID: %d) generated a revenue report for %s to %s", adminID, req.StartDate, req.EndDate)
	_, err = config.Pool.Exec(context.Background(), logQuery, logDescription)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to log admin action"})
	}

	// Construct the response
	response := RevenueReportResponse{
		TotalRevenue:      totalRevenue,
		TotalTransactions: totalTransactions,
		TopServices:       topServices,
	}

	return c.JSON(http.StatusOK, response)
}