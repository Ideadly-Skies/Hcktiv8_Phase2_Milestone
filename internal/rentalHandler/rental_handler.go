package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/golang-jwt/jwt/v4"
	config "w4/p2/milestones/config/database"

	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"

	"os"
)

// RentalRequest structure
type RentalRequest struct {
	CustomerID    int            `json:"customer_id" validate:"required"`
	ComputerID    int            `json:"computer_id" validate:"required"`
	AdminID       int            `json:"admin_id" validate:"required"`
	RentalStart   time.Time      `json:"rental_start" validate:"required"`
	RentalEnd     time.Time      `json:"rental_end" validate:"required"`
	Services      []ServiceEntry `json:"services"`
	ActivityDesc  string         `json:"activity_description"`
}

// ServiceEntry structure for additional services
type ServiceEntry struct {
	ServiceID int `json:"service_id" validate:"required"`
	Quantity  int `json:"quantity" validate:"required"`
}

// Initialize Midtrans Core API client
var coreAPI coreapi.Client

func Init() {
	// retrieve server key from .env
	ServerKey := os.Getenv("ServerKey")

	coreAPI = coreapi.Client{}
	coreAPI.New(ServerKey, midtrans.Sandbox)
}

func RentComputer(c echo.Context) error {
    // Initialize Midtrans CoreAPI client
    Init()

    var req RentalRequest
    if err := c.Bind(&req); err != nil {
        return c.JSON(http.StatusBadRequest, map[string]string{"message": "Invalid request"})
    }

    // Extract admin ID and role from JWT claims
    user := c.Get("user").(*jwt.Token)
    claims := user.Claims.(jwt.MapClaims)
	fmt.Println("claims:", claims)

    adminID := int(claims["admin_id"].(float64))
    adminRole := claims["role"].(string)

    // Validate admin role
    if adminRole != "admin" && adminRole != "super-admin" {
        return c.JSON(http.StatusForbidden, map[string]string{"message": "Unauthorized. Only selected admins can perform this action."})
    }

    // Calculate rental duration and cost
    var hourlyRate int
    query := "SELECT hourly_rate FROM computer WHERE id = $1 AND isAvailable = TRUE"
    err := config.Pool.QueryRow(context.Background(), query, req.ComputerID).Scan(&hourlyRate)
    if err != nil {
        return c.JSON(http.StatusBadRequest, map[string]string{"message": "Computer not available"})
    }

    rentalDuration := req.RentalEnd.Sub(req.RentalStart).Hours()
    totalCost := int(rentalDuration) * hourlyRate

    // Check if the user chooses to pay with wallet or GoPay
    paymentMethod := c.QueryParam("payment_method") // "wallet" or "gopay"

    if paymentMethod == "wallet" {
        // Deduct wallet balance
        var walletBalance float64
        walletQuery := "SELECT wallet FROM customer WHERE id = $1"
        err = config.Pool.QueryRow(context.Background(), walletQuery, req.CustomerID).Scan(&walletBalance)
        if err != nil {
            return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to retrieve wallet balance"})
        }

        if walletBalance < float64(totalCost) {
            return c.JSON(http.StatusBadRequest, map[string]string{"message": "Insufficient wallet balance"})
        }

        deductWalletQuery := "UPDATE customer SET wallet = wallet - $1 WHERE id = $2"
        _, err = config.Pool.Exec(context.Background(), deductWalletQuery, totalCost, req.CustomerID)
        if err != nil {
            return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to deduct wallet balance"})
        }
    } else if paymentMethod == "gopay" {
        // Create payment request via GoPay
        orderID := fmt.Sprintf("rental-%d-%d", req.CustomerID, time.Now().Unix())
        paymentRequest := &coreapi.ChargeReq{
            PaymentType: coreapi.PaymentTypeGopay,
            TransactionDetails: midtrans.TransactionDetails{
                OrderID:  orderID,
                GrossAmt: int64(totalCost),
            },
            Gopay: &coreapi.GopayDetails{
                EnableCallback: true,
                CallbackUrl:    "https://24d5-66-96-225-168.ngrok-free.app/webhook/payment",
            },
        }

        resp, err := coreAPI.ChargeTransaction(paymentRequest)
        if err != nil {
            return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to create GoPay payment"})
        }

        // Save transaction in the database
		transactionQuery := `
			INSERT INTO transaction (customer_id, transaction_type, amount, transaction_method, status, payment_url, order_id, metadata)
			VALUES ($1, 'Rental Payment', $2, 'GoPay', 'Pending', $3, $4, $5)`
		metadata := map[string]interface{}{
			"computer_id":   req.ComputerID,
			"rental_start":  req.RentalStart,
			"rental_end":    req.RentalEnd,
			"activity_desc": req.ActivityDesc,
		}
		_, txnErr := config.Pool.Exec(context.Background(), transactionQuery, req.CustomerID, totalCost, resp.Actions[0].URL, orderID, metadata)
		if txnErr != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to log transaction"})
		}

        // Return payment URL to the user
        return c.JSON(http.StatusOK, map[string]interface{}{
            "message":    "Payment initiated",
            "payment_url": resp.Actions[0].URL,
        })
    } else {
        return c.JSON(http.StatusBadRequest, map[string]string{"message": "Invalid payment method"})
    }

    // Record rental history
    var rentalHistoryID int
    rentalHistoryQuery := `
        INSERT INTO rental_history (customer_id, computer_id, admin_id, rental_start_time, rental_end_time, total_cost, booking_status)
        VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`        
    err = config.Pool.QueryRow(context.Background(), rentalHistoryQuery, req.CustomerID, req.ComputerID, adminID, req.RentalStart, req.RentalEnd, totalCost, "Settlement").Scan(&rentalHistoryID)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to record rental history"})
    }

    // Log rental activity
    logQuery := `
        INSERT INTO log (customer_id, computer_id, login_time, logout_time, activity_description)
        VALUES ($1, $2, $3, $4, $5)`
    _, err = config.Pool.Exec(context.Background(), logQuery, req.CustomerID, req.ComputerID, req.RentalStart, req.RentalEnd, req.ActivityDesc)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to log rental activity"})
    }

    // Log additional services if any
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

    // Update computer availability
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
