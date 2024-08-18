package handler

import (
	"cafeteller-api/firebase"
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type InstagramResponse struct {
	AccessToken string   `json:"access_token"`
	UserID      int      `json:"user_id"`
	Permission  []string `json:"permissions"`
}

func Auth(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Code is required"})
		return
	}

	appSecret := os.Getenv("INSTAGRAM_APP_SECRET")
	appID := os.Getenv("INSTAGRAM_APP_ID")
	redirectURI := os.Getenv("INSTAGRAM_REDIRECT_URI")
	if appSecret == "" || appID == "" || redirectURI == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Instagram App is not configured"})
		return
	}

	data := url.Values{}
	data.Set("app_id", appID)
	data.Set("app_secret", appSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", redirectURI+"/auth") // replace with your actual redirect URI

	resp, err := http.Post(
		"https://api.instagram.com/oauth/access_token",
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)

	if err != nil {
		log.Println("Error making request to Instagram:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to make request to Instagram"})
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Println("Error closing response body:", err)
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error reading response body:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response from Instagram"})
		return
	}

	// if body.code == 400 throw 500 error
	if resp.StatusCode == 400 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get access token from Instagram"})
		return
	}

	var igResponse InstagramResponse
	if err := json.Unmarshal(body, &igResponse); err != nil {
		log.Println("Error unmarshalling response:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response from Instagram"})
		return
	}

	additionalClaims := map[string]interface{}{
		"isAdmin": true,
	}

	auth := firebase.GetAuthClient(c)

	customToken, err := auth.CustomTokenWithClaims(context.Background(), strconv.Itoa(igResponse.UserID), additionalClaims)
	if err != nil {
		log.Println("Error creating custom token:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create custom token"})
		return
	}

	// Exchange to the long access token
	longTokenURL := "https://graph.instagram.com/access_token?grant_type=ig_exchange_token&client_secret=" + appSecret + "&access_token=" + igResponse.AccessToken
	longResp, err := http.Get(longTokenURL)
	if err != nil {
		log.Println("Error getting long access token:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get long access token"})
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Println("Error closing long token response body:", err)
		}
	}(longResp.Body)

	longTokenBody, err := io.ReadAll(longResp.Body)
	if err != nil {
		log.Println("Error reading long token response body:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read long token response"})
		return
	}

	var longTokenCredential map[string]interface{}
	if err := json.Unmarshal(longTokenBody, &longTokenCredential); err != nil {
		log.Println("Error unmarshalling long token response:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse long token response"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"credential":  longTokenCredential,
		"customToken": customToken,
	})
}
