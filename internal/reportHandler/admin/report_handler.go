package handler

import (
	"context"
	// "fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/golang-jwt/jwt/v4"

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
		WHERE transaction_date BETWEEN $1 AND $2 AND status = 'Settlement'`
	err = config.Pool.QueryRow(context.Background(), query, startDate, endDate).Scan(&totalRevenue, &totalTransactions)
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

	// Construct the response
	response := RevenueReportResponse{
		TotalRevenue:      totalRevenue,
		TotalTransactions: totalTransactions,
		TopServices:       topServices,
	}

	return c.JSON(http.StatusOK, response)
}
