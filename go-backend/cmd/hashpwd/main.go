package main

import (
	"fmt"
	"github.com/accountant-crm/go-backend/internal/auth"
)

func main() {
	hash, _ := auth.HashPassword("TestPassword123")
	fmt.Println(hash)
}
