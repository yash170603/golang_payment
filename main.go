package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/razorpay/razorpay-go"
)

// Config holds all configuration values
type Config struct {
	APIKey         string
	SecretKey      string
	Port           string
	AllowedOrigins []string
}

// PaymentService handles all payment related operations
type PaymentService struct {
	client *razorpay.Client
	config Config
}

// PaymentRequest represents the incoming payment creation request
type PaymentRequest struct {
	Amount int `json:"amount" binding:"required,min=1"`
}

// PaymentVerificationRequest represents the payment verification payload
type PaymentVerificationRequest struct {
	ServerOrderID     string `json:"order_id" binding:"required"`
	RazorpayPaymentID string `json:"razorpay_payment_id" binding:"required"`
	RazorpaySignature string `json:"razorpay_signature" binding:"required"`
}

// NewPaymentService creates a new instance of PaymentService
func NewPaymentService(config Config) (*PaymentService, error) {
	if config.APIKey == "" || config.SecretKey == "" {
		return nil, fmt.Errorf("missing required configuration")
	}

	client := razorpay.NewClient(config.APIKey, config.SecretKey)
	return &PaymentService{
		client: client,
		config: config,
	}, nil
}

func main() {

	err := godotenv.Load()
	if err != nil {

		log.Fatal("Error loading .env file")
	}
	fmt.Printf("API Key: %s\n", os.Getenv("RAZORPAY_API_KEY"))
	fmt.Printf("Secret Key: %s\n", os.Getenv("RAZORPAY_SECRET_KEY"))
	fmt.Printf("Port: %s\n", os.Getenv("PORT"))
	fmt.Printf("Allowed Origins: %s\n", os.Getenv("ALLOWED_ORIGINS"))
	// Set Gin to release mode in production
	gin.SetMode(gin.TestMode)

	config := Config{
		APIKey:         os.Getenv("RAZORPAY_API_KEY"),
		SecretKey:      os.Getenv("RAZORPAY_SECRET_KEY"),
		Port:           os.Getenv("PORT"),
		AllowedOrigins: strings.Split(os.Getenv("ALLOWED_ORIGINS"), ","),
	}

	if config.Port == "" {
		config.Port = "8080"
	}

	service, err := NewPaymentService(config)
	if err != nil {
		log.Fatalf("Failed to initialize payment service: %v", err)
	}

	r := gin.Default()

	// Middleware setup
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     config.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Routes
	r.POST("/api/v1/orders", service.CreateOrder)
	r.POST("/api/v1/verify", service.VerifyOrder)

	// Start server
	if err := r.Run(":" + config.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func (s *PaymentService) CreateOrder(c *gin.Context) {
	var req PaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	data := map[string]interface{}{
		"amount":   req.Amount,
		"currency": "INR",
		"receipt":  fmt.Sprintf("rcpt_%d", time.Now().Unix()),
		"notes": map[string]interface{}{
			"created_at": time.Now().Format(time.RFC3339),
		},
	}

	order, err := s.client.Order.Create(data, nil)
	if err != nil {
		log.Printf("Error creating order: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create order",
		})
		return
	}

	c.JSON(http.StatusOK, order)
}

func (s *PaymentService) VerifyOrder(c *gin.Context) {
	var req PaymentVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Generate verification data
	data := fmt.Sprintf("%s|%s", req.ServerOrderID, req.RazorpayPaymentID)

	// Verify signature
	if !s.verifySignature(data, req.RazorpaySignature) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid payment signature",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Payment verified successfully",
	})
}

func (s *PaymentService) verifySignature(data, signature string) bool {
	h := hmac.New(sha256.New, []byte(s.config.SecretKey))
	h.Write([]byte(data))
	generated := hex.EncodeToString(h.Sum(nil))
	return hmac.Equal([]byte(generated), []byte(signature))
}
