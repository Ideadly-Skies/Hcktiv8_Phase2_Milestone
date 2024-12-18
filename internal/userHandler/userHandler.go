package handler

import (
	"fmt"
	"net/http"
	config "w4/p2/milestones/config/database"

	"context"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"github.com/jackc/pgconn"
)

// Customers struct
type Customer struct {
	ID int `json:"id"`
	Name string `json:"name"`
	Username string `json:"username"`
	Email string `json:"Email"`
	Password string `json:"Password"`
	Deposit  string `json:"deposit_amount"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Jwt 	  string `json:"jwt_token"`
}

// Admin struct
type Admin struct {
	ID int `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"`
	Role 	 string `json:"role"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updaated_at"`
	Jwt 	 string `json:"jwt_token"`
}

// RegisterRequest for user 
type RegisterRequest struct {
	Name string `json:"name" validate:"required,name"`
	Username string `json:"username" validate:"required,username"`
	Email string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,password"`
	Role 	 string `json:"role" validate:"required,role"`
}

// loginRequest for user 
type LoginRequestUser struct {
	Email string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,password"`
}

type LoginRequestAdmin struct {
	Username string `json:"username" validate:"required,username"`
	Password string `json:"password" validate:"required,password"`
}

// login response: token
type LoginResponse struct {
	Token string `json:"token"`
}

var jwtSecret = []byte("12345")

// register user
func RegisterCustomer(c echo.Context) error {
    var req RegisterRequest 
    if err := c.Bind(&req); err != nil {
        return c.JSON(http.StatusBadRequest, map[string]string{"message": "Invalid Request"})
    }

	// hash the password
    hashPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Internal Server Error"})
    }

    // query to insert into customers db
	customer_query := "INSERT INTO customer (name, username, email, password) VALUES ($1, $2, $3, $4) RETURNING id"
	
	var customerID int
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// query row 1: insert to users 
	err = config.Pool.QueryRow(ctx, customer_query, req.Name, req.Username, req.Email, string(hashPassword)).Scan(&customerID)
	if err != nil {
		fmt.Println("Error inserting into Customers table:", err)

		if pgErr, ok := err.(*pgconn.PgError); ok {
			if pgErr.Code == "23505" {
				return c.JSON(http.StatusBadRequest, map[string]string{"message": "Email already registered"})
			}
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Internal Server Error"})
	}

    return c.JSON(http.StatusOK, map[string]interface{}{
        "message": fmt.Sprintf(`User %s registered successfully`,req.Name),
        "email": req.Email,
    })
}

// register admin
func RegisterAdmin(c echo.Context) error {
    var req RegisterRequest 
    if err := c.Bind(&req); err != nil {
        return c.JSON(http.StatusBadRequest, map[string]string{"message": "Invalid Request"})
    }

	// hash the password
    hashPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Internal Server Error"})
    }

    // query to insert into admin db
	admin_query := "INSERT INTO admin (username, password, role) VALUES ($1, $2, $3) RETURNING id"
	
	var adminID int
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// query row 1: insert to users 
	err = config.Pool.QueryRow(ctx, admin_query, req.Username, string(hashPassword), req.Role).Scan(&adminID)
	if err != nil {
		fmt.Println("Error inserting into Admin table:", err)

		if pgErr, ok := err.(*pgconn.PgError); ok {
			if pgErr.Code == "23505" {
				return c.JSON(http.StatusBadRequest, map[string]string{"message": "Email already registered"})
			}
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Internal Server Error"})
	}

    return c.JSON(http.StatusOK, map[string]interface{}{
        "message": fmt.Sprintf(`Admin %s registered successfully`,req.Name),
        "email": req.Email,
    })
}

// login customer
func LoginCustomer(c echo.Context) error {
	var req LoginRequestUser
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message":"Invalid Request"})
	}
	
	var customer Customer
	query := "SELECT id, email, password FROM customer WHERE email = $1"
	err := config.Pool.QueryRow(context.Background(), query, req.Email).Scan(&customer.ID, &customer.Email, &customer.Password)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "Invalid email or password"})
	}

	// compare password to see if it matches the customer password provided
	if err := bcrypt.CompareHashAndPassword([]byte(customer.Password), []byte(req.Password)); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "Invalid email or password"})
	}

	// create new jwt claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"customer_id": customer.ID,
		"exp":     jwt.NewNumericDate(time.Now().Add(72 * time.Hour)), // Use `jwt.NewNumericDate` for expiry
	})
	
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Invalid Generate Token"})
	}

	// Update the jwt_token column in the database
	updateQuery := "UPDATE customer SET jwt_token = $1 WHERE id = $2"
	_, err = config.Pool.Exec(context.Background(), updateQuery, tokenString, customer.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to update token"})
	}

	// return ok status and login response
	return c.JSON(http.StatusOK, LoginResponse{Token: tokenString})
}

// login admin
func LoginAdmin(c echo.Context) error {
	var req LoginRequestAdmin
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message":"Invalid Request"})
	}
	
	var admin Admin 
	query := "SELECT id, username, password FROM admin WHERE username = $1"
	err := config.Pool.QueryRow(context.Background(), query, req.Username).Scan(&admin.ID, &admin.Username, &admin.Password)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "Invalid email or password"})
	}

	// compare password to see if it matches the customer password provided
	if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(req.Password)); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "Invalid email or password"})
	}

	// create new jwt claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"admin_id": admin.ID,
		"exp":     jwt.NewNumericDate(time.Now().Add(72 * time.Hour)), // Use `jwt.NewNumericDate` for expiry
	})
	
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Invalid Generate Token"})
	}

	// Update the jwt_token column in the database
	updateQuery := "UPDATE admin SET jwt_token = $1 WHERE id = $2"
	_, err = config.Pool.Exec(context.Background(), updateQuery, tokenString, admin.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Failed to update token"})
	}

	// return ok status and login response
	return c.JSON(http.StatusOK, LoginResponse{Token: tokenString})
}