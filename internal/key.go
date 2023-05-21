package internal

import (
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func readKey(path string) (*rsa.PrivateKey, error) {
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read key file: %w", err)
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("unable to parse key from PEM to RSA format: %w", err)
	}

	return key, nil
}

func readKeyBase64(keyBase64 string) (*rsa.PrivateKey, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return nil, fmt.Errorf("unable to decode key from base64: %w", err)
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("unable to parse key from PEM to RSA format: %w", err)
	}

	return key, nil
}

func generateJWT(appID string, expiry int, key *rsa.PrivateKey) (string, error) {
	iat := jwt.NewNumericDate(time.Now().Add(-60 * time.Second))
	exp := jwt.NewNumericDate(time.Now().Add(time.Duration(expiry) * 60 * time.Second))
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iat": iat,
		"exp": exp,
		"iss": appID,
	})
	signedToken, err := token.SignedString(key)
	if err != nil {
		return "", fmt.Errorf("unable to sign JWT: %w", err)
	}

	return signedToken, nil
}
