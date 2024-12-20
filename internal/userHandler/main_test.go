package handler

import (
    "testing"
    "w4/p2/milestones/config/database"
)

func TestMain(m *testing.M) {
    // Initialize the database connection
    config.InitDB()
    defer config.CloseDB()

    // Run the tests
    m.Run()
}