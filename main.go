package main

import (
	"w4/p2/milestones/config/database"
	user_handler "w4/p2/milestones/internal/userHandler"
	cust_middleware "w4/p2/milestones/internal/middleware"
	transaction_handler "w4/p2/milestones/internal/transactionHandler"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main(){
	// migrate data to supabase
	// config.MigrateData()

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

	// protected routes for customer using JWT middlware
	customerGroup := e.Group("/customer")
	customerGroup.Use(cust_middleware.JWTMiddleware)

	customerGroup.GET("/wallet/balance", transaction_handler.GetWalletBalance)
	customerGroup.POST("/wallet/topup", transaction_handler.TopUpWallet)

	// start the server at 8080
	e.Logger.Fatal(e.Start(":8080"))
}