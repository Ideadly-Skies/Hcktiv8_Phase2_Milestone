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

// GetWalletBalance godoc
// @Summary Get wallet balance
// @Description Fetch the wallet balance of the authenticated customer
// @Tags Transactions
// @Produce json
// @Success 200 {object} WalletBalanceResponse "Customer's wallet balance"
// @Failure 500 {object} map[string]string "Internal server error"
// @Security BearerAuth
// @Router /wallet/balance [get]
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

// CreatePayment godoc
// @Summary Create a payment request
// @Description Allows customers to create payment requests (e.g., Top-Up)
// @Tags Transactions
// @Accept json
// @Produce json
// @Param request body PaymentRequest true "Payment request body"
// @Success 200 {object} map[string]interface{} "Payment request details"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 500 {object} map[string]string "Internal server error"
// @Security BearerAuth
// @Router /payment/create [post]
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

// CheckPaymentStatus godoc
// @Summary Check payment status
// @Description Manually check the status of a payment using the order ID
// @Tags Transactions
// @Produce json
// @Param orderID path string true "Order ID"
// @Success 200 {object} map[string]interface{} "Payment status details"
// @Failure 500 {object} map[string]string "Internal server error"
// @Security BearerAuth
// @Router /payment/status/{orderID} [get]
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
			adminID, ok := metadata["admin_id"].(float64) // JSON numbers are float64 in Go
			
			// fmt.Println("third err: ", ok)				

			if !ok {
				return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Invalid admin_id in metadata"})
			}

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
			
			totalCost, ok := metadata["total_cost"].(float64)
			if !ok {
				return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Invalid activity_desc in metadata"})
			}
		
			// Update rental history
			rentalHistoryQuery := `
			INSERT INTO rental_history (customer_id, computer_id, admin_id, rental_start_time, rental_end_time, total_cost, booking_status)
			VALUES ($1, $2, $3, $4, $5, $6, 'Completed')`
			
			_, err = config.Pool.Exec(context.Background(), rentalHistoryQuery,
				customerID, int(computerID), adminID, rentalStart, rentalEnd, totalCost)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to update rental history"})
			}
	
			// Update PC availability
			updatePCQuery := `UPDATE computer SET isAvailable = FALSE WHERE id = $1`
			_, err = config.Pool.Exec(context.Background(), updatePCQuery, int(computerID))
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to update computer availability"})
			}
		
			// Log general activity in the log table
			desc := fmt.Sprintf("Rental payment completed for Customer %d, with Computer %d", customerID, int(computerID))
			logQuery := `INSERT INTO log (description) VALUES ($1)`
			_, err = config.Pool.Exec(context.Background(), logQuery, desc)
			
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to log activity"})
			}
		} else if transactionType == "Service Payment" {

			// Fetch metadata for deferred service deduction
			var metadataJSON string
			metadataQuery := `SELECT metadata FROM transaction WHERE order_id = $1`
			err := config.Pool.QueryRow(context.Background(), metadataQuery, orderID).Scan(&metadataJSON)
			fmt.Println("error invoked from check payment status: ", err)	
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to fetch metadata"})
			}
		
			var metadata []map[string]interface{}
			err = json.Unmarshal([]byte(metadataJSON), &metadata)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to parse metadata"})
			}
		
			// Deduct service quantities
			for _, service := range metadata {
				serviceID := int(service["service_id"].(float64))
				quantity := int(service["quantity"].(float64))
				
				// update quantity of said product
				updateQuantityQuery := "UPDATE service SET quantity = quantity - $1 WHERE id = $2"
				_, err = config.Pool.Exec(context.Background(), updateQuantityQuery, quantity, serviceID)
				if err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]string{"message": fmt.Sprintf("Failed to update quantity for Service ID %d", serviceID)})
				}

				// Log the service purchase in the log table
				logDesc := fmt.Sprintf("Customer %d purchased Service ID %d (Quantity: %d)", customerID, serviceID, quantity)
				logQuery := `INSERT INTO log (description) VALUES ($1)`
				_, err = config.Pool.Exec(context.Background(), logQuery, logDesc)
				if err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to log service purchase"})
				}
				
				// Insert into the rental_services table (optional if related to a rental)
				rentalServiceQuery := `
					INSERT INTO rental_services (service_id, quantity, created_at)
					VALUES ($1, $2, NOW())`
				_, err = config.Pool.Exec(context.Background(), rentalServiceQuery, serviceID, quantity)
				if err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]string{
						"message": fmt.Sprintf("Failed to log service into rental_services for Service ID %d", serviceID),
					})
				}
			}
		}
	}

	return c.JSON(http.StatusOK, resp)
}