package main

import (
	"w4/p2/milestones/config/database"
	user_handler "w4/p2/milestones/internal/userHandler"
	cust_middleware "w4/p2/milestones/internal/middleware"
	transaction_handler "w4/p2/milestones/internal/transactionHandler"
	rental_handler "w4/p2/milestones/internal/rentalHandler"
	report_handler "w4/p2/milestones/internal/reportHandler"	

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main(){
	// migrate data to supabase
	config.MigrateData()

	// connect to db
	config.InitDB()
	defer config.CloseDB()

	e := echo.New()

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// public routes
	e.POST("/customer/register", user_handler.RegisterCustomer)
	e.POST("/customer/login", user_handler.LoginCustomer)

	e.POST("/admin/register", user_handler.RegisterAdmin)
	e.POST("/admin/login", user_handler.LoginAdmin)

	// protected routes for customer using JWT middleware
	customerGroup := e.Group("/customer")
	customerGroup.Use(cust_middleware.JWTMiddleware)

	customerGroup.GET("/wallet/balance", transaction_handler.GetWalletBalance)
	customerGroup.POST("/wallet/payment", transaction_handler.CreatePayment)
	customerGroup.GET("/wallet/payment-status/:orderID", transaction_handler.CheckPaymentStatus)
	customerGroup.GET("/booking/report", report_handler.GetBookingReport)

	// protected routes for admin using JWT middleware
	adminGroup := e.Group("/admin")
	adminGroup.Use(cust_middleware.JWTMiddleware)

	adminGroup.POST("/rental", rental_handler.RentComputer)

	// start the server at 8080
	e.Logger.Fatal(e.Start(":8080"))
}