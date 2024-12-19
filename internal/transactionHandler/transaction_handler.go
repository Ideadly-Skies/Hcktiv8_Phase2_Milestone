package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	config "w4/p2/milestones/config/database"
	"encoding/json"

	"github.com/golang-jwt/jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"

	"os"
)

// Struct for wallet balance response
type WalletBalanceResponse struct {
	Balance float64 `json:"balance"`
}

// Struct for Top-Up and Rental requests
type PaymentRequest struct {
	Amount        float64 `json:"amount" validate:"required"`
	PaymentPurpose string  `json:"purpose" validate:"required"` // Either "Top-Up" or "Rental Payment"
}

// Initialize Midtrans Core API client
var coreAPI coreapi.Client

func Init() {
	// retrieve server key from .env
	ServerKey := os.Getenv("ServerKey")

	coreAPI = coreapi.Client{}
	coreAPI.New(ServerKey, midtrans.Sandbox)
}

// Get wallet balance
func GetWalletBalance(c echo.Context) error {
	// Extract customer ID from JWT claims
	user := c.Get("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	customerID := int(claims["customer_id"].(float64))

	// Query wallet balance
	var balance float64
	query := "SELECT wallet FROM customer WHERE id = $1"
	err := config.Pool.QueryRow(context.Background(), query, customerID).Scan(&balance)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to retrieve wallet balance"})
	}

	// Return wallet balance
	return c.JSON(http.StatusOK, WalletBalanceResponse{Balance: balance})
}

// Create Payment Request (Top-Up or Rental Payment)
func CreatePayment(c echo.Context) error {
	// Initialize Midtrans
	Init()

	// Extract customer ID from JWT claims
	user := c.Get("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	customerID := int(claims["customer_id"].(float64))

	// Bind and validate request body
	var req PaymentRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "Invalid request"})
	}

	if req.Amount <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "Payment amount must be greater than zero"})
	}

	// Generate order ID
	orderID := fmt.Sprintf("order-%d-%d", customerID, time.Now().Unix())

	// Create a Midtrans charge request
	request := &coreapi.ChargeReq{
		PaymentType: coreapi.PaymentTypeGopay,
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  orderID,
			GrossAmt: int64(req.Amount), // Midtrans requires amount in integer (cents)
		},
		Gopay: &coreapi.GopayDetails{
			EnableCallback: true,
			CallbackUrl:    "https://24d5-66-96-225-168.ngrok-free.app/webhook/payment",
		},
	}

	// Send the charge request to Midtrans
	resp, err := coreAPI.ChargeTransaction(request)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to create payment"})
	}

	// Insert the transaction into the Transaction table
	transactionQuery := `
		INSERT INTO transaction (customer_id, transaction_type, amount, transaction_method, status, payment_url, order_id)
		VALUES ($1, $2, $3, 'Bank Transfer', 'Pending', $4, $5)
	`
	_, txnErr := config.Pool.Exec(context.Background(), transactionQuery, customerID, req.PaymentPurpose, req.Amount, resp.Actions[0].URL, orderID)
	if txnErr != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to log transaction"})
	}

	// Respond with Midtrans payment details
	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":       "Payment request created",
		"transaction_id": resp.TransactionID,
		"order_id":      resp.OrderID,
		"payment_url":   resp.Actions[0].URL,
		"gross_amount":  resp.GrossAmount,
		"status":        resp.TransactionStatus,
	})
}

// Manual Payment Status Check
func CheckPaymentStatus(c echo.Context) error {
	// initialize midtrans api
	Init()	
	
	orderID := c.Param("orderID")

	// Fetch payment status from Midtrans
	resp, err := coreAPI.CheckTransaction(orderID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to fetch payment status"})
	}

	// Update the transaction status in the database
	updateTransactionQuery := `UPDATE transaction SET status = $1 WHERE order_id = $2`
	_, dbErr := config.Pool.Exec(context.Background(), updateTransactionQuery, resp.TransactionStatus, orderID)
	if dbErr != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to update transaction"})
	}

	// print the transaction status for debugging
	fmt.Println("resp transaction status: ", resp.TransactionStatus)

	// Handle successful payment
	if resp.TransactionStatus == "settlement" {
		// Fetch transaction type and customer ID from the database
		var transactionType string
		var customerID int
		
		transactionQuery := `SELECT transaction_type, customer_id FROM transaction WHERE order_id = $1`
		err := config.Pool.QueryRow(context.Background(), transactionQuery, orderID).Scan(&transactionType, &customerID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to fetch transaction details"})
		}

		fmt.Println("transaction type: ", transactionType)

		// update wallet for top up
		if transactionType == "Top-Up" {
			updateWalletQuery := `UPDATE customer SET wallet = wallet + $1 WHERE id = $2`
			_, err := config.Pool.Exec(context.Background(), updateWalletQuery, resp.GrossAmount, customerID)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to update wallet balance"})
			}
		} else if transactionType == "Rental Payment" {
			// Fetch metadata and deserialize
			var metadataJSON string
			metadataQuery := `SELECT metadata FROM transaction WHERE order_id = $1`
			err := config.Pool.QueryRow(context.Background(), metadataQuery, orderID).Scan(&metadataJSON)
			
			// fmt.Println("first err: ", err)
			
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to fetch rental metadata"})
			}
		
			// Deserialize JSON into a map
			var metadata map[string]interface{}
			err = json.Unmarshal([]byte(metadataJSON), &metadata)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to parse rental metadata"})
			}
			
			// fmt.Println("second err: ", err)	

			// Validate and convert metadata fields
			computerID, ok := metadata["computer_id"].(float64) // JSON numbers are float64 in Go
			
			// fmt.Println("third err: ", ok)				

			if !ok {
				return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Invalid computer_id in metadata"})
			}
		
			rentalStart, ok := metadata["rental_start"].(string)
			
			// fmt.Println("fourth err: ", ok)

			if !ok {
				return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Invalid rental_start in metadata"})
			}
		
			rentalEnd, ok := metadata["rental_end"].(string)

			// fmt.Println("fifth err: ", ok)

			if !ok {
				return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Invalid rental_end in metadata"})
			}
			
			activityDesc, ok := metadata["activity_desc"].(string)
			
			// fmt.Println("sixth err: ", ok)
			
			if !ok {
				return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Invalid activity_desc in metadata"})
			}
		
			// Log rental activity
			logQuery := `
				INSERT INTO log (customer_id, computer_id, login_time, logout_time, activity_description)
				VALUES ($1, $2, $3, $4, $5)`
			_, err = config.Pool.Exec(context.Background(), logQuery, 
				customerID, int(computerID), rentalStart, rentalEnd, activityDesc)
			
			// fmt.Println("seventh err: ", err)
			
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to log rental activity"})
			}
		} 
	}

	return c.JSON(http.StatusOK, resp)
}