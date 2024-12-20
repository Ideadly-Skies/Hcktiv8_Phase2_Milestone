package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"os"

	config "w4/p2/milestones/config/database"

	"github.com/golang-jwt/jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"
)

// Initialize Midtrans Core API client
var coreAPI coreapi.Client

func Init() {
	// retrieve server key from .env
	ServerKey := os.Getenv("ServerKey")

	coreAPI = coreapi.Client{}
	coreAPI.New(ServerKey, midtrans.Sandbox)
}

func PurchaseService(c echo.Context) error {
    // init coreAPI
    Init()

    type ServiceRequest struct {
        CustomerID int               `json:"customer_id"`
        Services   []struct {
            ServiceID int `json:"service_id"`
            Quantity  int `json:"quantity"`
        } `json:"services"`
        PaymentMethod string `json:"payment_method"`
    }

    var req ServiceRequest
    if err := c.Bind(&req); err != nil {
        return c.JSON(http.StatusBadRequest, map[string]string{"message": "Invalid request"})
    }

    // Extract admin role from JWT claims
    user := c.Get("user").(*jwt.Token)
    claims := user.Claims.(jwt.MapClaims)
    adminRole := claims["role"].(string)

    // Validate admin role
    if adminRole != "admin" && adminRole != "super-admin" {
        return c.JSON(http.StatusForbidden, map[string]string{"message": "Unauthorized. Only selected admins can perform this action."})
    }

    // Calculate total service cost
    var totalCost float64
    var metadata []map[string]interface{} // To store service details for deferred deduction

    for _, service := range req.Services {
        var servicePrice float64
        var availableQuantity int

        query := "SELECT price, quantity FROM service WHERE id = $1"
        err := config.Pool.QueryRow(context.Background(), query, service.ServiceID).Scan(&servicePrice, &availableQuantity)
        if err != nil {
            return c.JSON(http.StatusBadRequest, map[string]string{"message": "Invalid service ID"})
        }

        if service.Quantity > availableQuantity {
            return c.JSON(http.StatusBadRequest, map[string]string{
                "message": fmt.Sprintf("Insufficient stock for service ID %d. Available: %d", service.ServiceID, availableQuantity),
            })
        }

        totalCost += servicePrice * float64(service.Quantity)

        // Add service details to metadata for deferred deduction
        metadata = append(metadata, map[string]interface{}{
            "service_id": service.ServiceID,
            "quantity":   service.Quantity,
        })
    }

    if req.PaymentMethod == "wallet" {
        // Deduct wallet balance and update quantities immediately
        var walletBalance float64
        walletQuery := "SELECT wallet FROM customer WHERE id = $1"
        err := config.Pool.QueryRow(context.Background(), walletQuery, req.CustomerID).Scan(&walletBalance)
        if err != nil {
            return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to retrieve wallet balance"})
        }

        if walletBalance < totalCost {
            return c.JSON(http.StatusBadRequest, map[string]string{"message": "Insufficient wallet balance"})
        }

        deductWalletQuery := "UPDATE customer SET wallet = wallet - $1 WHERE id = $2"
        _, err = config.Pool.Exec(context.Background(), deductWalletQuery, totalCost, req.CustomerID)
        if err != nil {
            return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to deduct wallet balance"})
        }

        // update the quantity of the services and insert into log table
        for _, service := range req.Services {
            updateQuantityQuery := "UPDATE service SET quantity = quantity - $1 WHERE id = $2"
            _, err = config.Pool.Exec(context.Background(), updateQuantityQuery, service.Quantity, service.ServiceID)
            if err != nil {
                return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to update service quantity"})
            }

            // Log the service purchase
            logDesc := fmt.Sprintf("Customer %d purchased Service ID %d (Quantity: %d)",
                req.CustomerID, service.ServiceID, service.Quantity)
            logQuery := `INSERT INTO log (description) VALUES ($1)`
            _, err = config.Pool.Exec(context.Background(), logQuery, logDesc)
            if err != nil {
                return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to log service purchase"})
            }

            // Insert into the rental_services table
            rentalServiceQuery := `
                INSERT INTO rental_services (rental_history_id, service_id, quantity, created_at)
                VALUES (NULL, $1, $2, NOW())`
            _, err = config.Pool.Exec(context.Background(), rentalServiceQuery, service.ServiceID, service.Quantity)
            if err != nil {
                return c.JSON(http.StatusInternalServerError, map[string]string{
                    "message": fmt.Sprintf("Failed to log service into rental_services for Service ID %d", service.ServiceID),
                })
            }
        }

        // Log transaction
        transactionQuery := `
            INSERT INTO transaction (customer_id, transaction_type, amount, transaction_method, status, transaction_date)
            VALUES ($1, 'Service Payment', $2, 'Wallet', 'settlement', NOW())`
        _, txnErr := config.Pool.Exec(context.Background(), transactionQuery, req.CustomerID, totalCost)
        if txnErr != nil {
            return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to log transaction"})
        }
    } else if req.PaymentMethod == "gopay" {
        // Create GoPay payment
        orderID := fmt.Sprintf("service-%d-%d", req.CustomerID, time.Now().Unix())
        paymentRequest := &coreapi.ChargeReq{
            PaymentType: coreapi.PaymentTypeGopay,
            TransactionDetails: midtrans.TransactionDetails{
                OrderID:  orderID,
                GrossAmt: int64(totalCost),
            },
            Gopay: &coreapi.GopayDetails{
                EnableCallback: true,
                CallbackUrl:    "https://your-callback-url.com/webhook/payment",
            },
        }

        resp, err := coreAPI.ChargeTransaction(paymentRequest)
        if err != nil {
            return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to create GoPay payment"})
        }

        // Log transaction with metadata
        transactionQuery := `
            INSERT INTO transaction (customer_id, transaction_type, amount, transaction_method, status, payment_url, order_id, metadata)
            VALUES ($1, 'Service Payment', $2, 'GoPay', 'Pending', $3, $4, $5)`
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

    return c.JSON(http.StatusOK, map[string]interface{}{
        "message":    "Services purchased successfully",
        "total_cost": totalCost,
    })
}