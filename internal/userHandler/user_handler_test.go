package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"fmt"
	"time"
)

func TestRegisterAndLoginCustomer(t *testing.T) {
    e := echo.New()

    // Register Customer
    registerPayload := `{
        "name": "Helena Tantowijaya",
        "username": "Helena123",
        "email": "Helena123@example.com",
        "password": "password123",
        "role": "customer"
    }`

    req := httptest.NewRequest(http.MethodPost, "/customer/register", bytes.NewReader([]byte(registerPayload)))
    req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
    rec := httptest.NewRecorder()
    c := e.NewContext(req, rec)

    // Call RegisterCustomer
    err := RegisterCustomer(c)
    if assert.NoError(t, err) {
        assert.Equal(t, http.StatusOK, rec.Code)

        var registerResponse map[string]interface{}
        json.Unmarshal(rec.Body.Bytes(), &registerResponse)
        assert.Equal(t, "User Helena Tantowijaya registered successfully", registerResponse["message"])
    }

    // Login Customer
    loginPayload := `{
        "email": "johndoe_test_123@example.com",
        "password": "password123"
    }`

    loginReq := httptest.NewRequest(http.MethodPost, "/customer/login", bytes.NewReader([]byte(loginPayload)))
    loginReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
    loginRec := httptest.NewRecorder()
    loginC := e.NewContext(loginReq, loginRec)

    // Call LoginCustomer
    err = LoginCustomer(loginC)
    if assert.NoError(t, err) {
        assert.Equal(t, http.StatusOK, loginRec.Code)

        var loginResponse LoginResponse
        json.Unmarshal(loginRec.Body.Bytes(), &loginResponse)
        assert.NotEmpty(t, loginResponse.Token)
    }
}

func TestRegisterAndLoginAdmin(t *testing.T) {
	e := echo.New()

	// Generate a unique username for testing to avoid duplicate constraint issues
	uniqueUsername := fmt.Sprintf("adminuser_%d", time.Now().UnixNano())

	// Register Admin
	registerPayload := fmt.Sprintf(`{
		"name": "Admin User",
		"username": "%s",
		"email": "admin_%d@example.com",
		"password": "adminpass",
		"role": "admin"
	}`, uniqueUsername, time.Now().UnixNano())

	registerReq := httptest.NewRequest(http.MethodPost, "/admin/register", bytes.NewReader([]byte(registerPayload)))
	registerReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	registerRec := httptest.NewRecorder()
	registerCtx := e.NewContext(registerReq, registerRec)

	// Call RegisterAdmin
	err := RegisterAdmin(registerCtx)
	if assert.NoError(t, err) {
		assert.Equal(t, http.StatusOK, registerRec.Code)

		var registerResponse map[string]interface{}
		json.Unmarshal(registerRec.Body.Bytes(), &registerResponse)
		assert.Equal(t, fmt.Sprintf("Admin %s registered successfully", uniqueUsername), registerResponse["message"])
		assert.Equal(t, "admin", registerResponse["role"])
	}

	// Login Admin
	loginPayload := fmt.Sprintf(`{
		"username": "%s",
		"password": "adminpass"
	}`, uniqueUsername)

	loginReq := httptest.NewRequest(http.MethodPost, "/admin/login", bytes.NewReader([]byte(loginPayload)))
	loginReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	loginRec := httptest.NewRecorder()
	loginCtx := e.NewContext(loginReq, loginRec)

	// Call LoginAdmin
	err = LoginAdmin(loginCtx)
	if assert.NoError(t, err) {
		assert.Equal(t, http.StatusOK, loginRec.Code)

		var loginResponse LoginResponse
		json.Unmarshal(loginRec.Body.Bytes(), &loginResponse)
		assert.NotEmpty(t, loginResponse.Token)
	}
}