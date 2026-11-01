package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	BcryptCost = 12
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
)

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(bytes), nil
}

func CheckPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func GenerateAccessToken(userID uuid.UUID, secret string, expiry time.Duration) (string, error) {
	now := time.Now().UTC()
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
		IssuedAt:  jwt.NewNumericDate(now),
		ID:        uuid.NewString(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign access token: %w", err)
	}
	return signed, nil
}

func ValidateAccessToken(tokenStr, secret string) (uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &jwt.RegisteredClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil || !token.Valid {
		return uuid.Nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || claims.Subject == "" {
		return uuid.Nil, ErrInvalidToken
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}

	return userID, nil
}

func GenerateRefreshToken(sessionID uuid.UUID) (rawToken string, tokenHash string, err error) {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", fmt.Errorf("failed to read secure random bytes: %w", err)
	}

	secretHex := hex.EncodeToString(randomBytes)
	rawToken = fmt.Sprintf("%s.%s", sessionID.String(), secretHex)
	tokenHash = HashTokenSecret(secretHex)
	return rawToken, tokenHash, nil
}

func ParseRefreshToken(rawToken string) (uuid.UUID, string, error) {
	parts := strings.Split(rawToken, ".")
	if len(parts) != 2 {
		return uuid.Nil, "", ErrInvalidToken
	}

	sessionID, err := uuid.Parse(parts[0])
	if err != nil || parts[1] == "" {
		return uuid.Nil, "", ErrInvalidToken
	}

	return sessionID, parts[1], nil
}

func HashTokenSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func VerifyTokenHash(storedHash, candidateSecret string) bool {
	candidateHash := HashTokenSecret(candidateSecret)
	return subtle.ConstantTimeCompare([]byte(storedHash), []byte(candidateHash)) == 1
}
