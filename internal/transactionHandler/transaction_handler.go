package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	config "w4/p2/milestones/config/database"

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

// Struct for Top-Up request
type TopUpRequest struct {
	Amount float64 `json:"amount" validate:"required"`
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

// Top-Up wallet
func TopUpWallet(c echo.Context) error {
	// init midtrans api
	Init()	

	// Extract customer ID from JWT claims
	user := c.Get("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	customerID := int(claims["customer_id"].(float64))

	// Bind and validate request body
	var req TopUpRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "Invalid request"})
	}

	if req.Amount <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "Top-up amount must be greater than zero"})
	}

	// Create a Midtrans transaction
	orderID := fmt.Sprintf("order-%d-%d", customerID, time.Now().Unix())
	request := &coreapi.ChargeReq{
		PaymentType: coreapi.PaymentTypeBankTransfer,
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  orderID,
			GrossAmt: int64(req.Amount), // Midtrans requires amount in integer (cents)
		},
		BankTransfer: &coreapi.BankTransferDetails{
			Bank: midtrans.BankBca, // Change to your preferred bank
		},
	}

	// Send the charge request to Midtrans
	resp, err := coreAPI.ChargeTransaction(request)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to process payment"})
	}

	// Simulate top-up success for demo (In production, use webhook for payment status updates)
	dbQuery := "UPDATE customer SET wallet = wallet + $1 WHERE id = $2"
	_, dbErr := config.Pool.Exec(context.Background(), dbQuery, req.Amount, customerID)
    if dbErr != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to update wallet balance"})
    }

	// Insert the transaction into the Transaction table
	transactionQuery := `
		INSERT INTO transaction (customer_id, transaction_type, amount, transaction_method, status)
		VALUES ($1, 'Top-Up', $2, 'Bank Transfer', 'Completed')
	`
	_, txnErr := config.Pool.Exec(context.Background(), transactionQuery, customerID, req.Amount)
	if txnErr != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to log transaction"})
	}

	// Respond with Midtrans transaction details
	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":        "Top-up request successful",
		"transaction_id": resp.TransactionID,
		"order_id":       resp.OrderID,
		"payment_type":   resp.PaymentType,
		"bank":           resp.VaNumbers[0].Bank,
		"va_number":      resp.VaNumbers[0].VANumber,
		"gross_amount":   resp.GrossAmount,
		"status":         resp.TransactionStatus,
	})
}