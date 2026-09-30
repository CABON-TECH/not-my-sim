package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load(".env")
	username := os.Getenv("AT_USERNAME")
	apiKey := os.Getenv("AT_API_KEY")
	from := os.Getenv("AT_SMS_SHORTCODE")
	to := "+254729080194"

	fmt.Printf("Testing AT SMS API...\n")
	fmt.Printf("Username: %s\n", username)
	fmt.Printf("From: %s\n", from)
	fmt.Printf("To: %s\n", to)

	endpoint := "https://api.africastalking.com/version1/messaging"
	if username == "sandbox" {
		endpoint = "https://api.sandbox.africastalking.com/version1/messaging"
	}

	data := url.Values{}
	data.Set("username", username)
	data.Set("to", to)
	data.Set("message", "Test SMS from Not-My-SIM")
	data.Set("from", "NotMySim")

	req, _ := http.NewRequest("POST", endpoint, strings.NewReader(data.Encode()))
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("apiKey", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("HTTP Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	fmt.Printf("HTTP Status: %d\n", resp.StatusCode)
	fmt.Printf("AT Response: %s\n", string(bodyBytes))
}
