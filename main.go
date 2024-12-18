package main

import (
	"w4/p2/milestones/config/database"
)

func main(){
	// migrate data to supabase
	config.MigrateData()

	// connect to db
	config.InitDB()
	defer config.CloseDB()
}