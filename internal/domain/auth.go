package domain

import (
	"context"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func getJwtSecret() ([]byte, error) {
	secret := os.Getenv("JWT_SECRET")
	if len(secret) == 0 {
		return nil, &JwtSecretError{}
	}
	return []byte(secret), nil
}

func (au *AuthService) Register(ctx context.Context, req SignUpRequest) error {
	if len(req.Login) == 0 || len(req.Password) == 0 {
		return &ValidationError{Message: "User must specify login and password"}
	}

	existingUser, err := au.UserSvc.GetUserByLogin(ctx, req.Login)
	if err != nil {
		return err
	}
	if existingUser != nil {
		return &UserAlreadyExistsError{Login: req.Login}
	}

	id := uuid.New()

	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	err = au.UserSvc.SaveUser(ctx, User{
		UUID:         id.String(),
		Login:        req.Login,
		PasswordHash: string(hashedPwd),
	})
	return err
}

func (au *AuthService) Authorize(ctx context.Context, jwtReq *JwtRequest) (jwtRep *JwtResponse, err error) {

	user, err := au.UserSvc.GetUserByLogin(ctx, jwtReq.Login)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, &IncorrectCredsError{
			login: jwtReq.Login,
		}
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(jwtReq.Password))
	if err != nil {
		return nil, &IncorrectCredsError{
			login: jwtReq.Login,
		}
	}

	accTkn, err := au.JwtProvider.GenerateAccessToken(user)
	if err != nil {
		return nil, err
	}

	refTkn, err := au.JwtProvider.GenerateRefreshToken(user)
	if err != nil {
		return nil, err
	}

	ans := JwtResponse{
		Type:         "Bearer",
		AccessToken:  accTkn,
		RefreshToken: refTkn,
	}

	return &ans, nil
}

func (au *AuthService) UpdateAccessToken(ctx context.Context, refreshToken string) (*JwtResponse, error) {
	err := au.JwtProvider.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, err
	}

	uuid, err := au.JwtProvider.GetUUIDFromToken(refreshToken)
	if err != nil {
		return nil, err
	}
	if len(uuid) == 0 {
		return nil, &InvalidTokenError{}
	}

	user, err := au.GetUser(ctx, uuid)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, &IncorrectCredsError{}
	}

	accTkn, err := au.JwtProvider.GenerateAccessToken(user)
	if err != nil {
		return nil, err
	}

	ans := JwtResponse{
		Type:         "Bearer",
		RefreshToken: refreshToken,
		AccessToken:  accTkn,
	}

	return &ans, nil
}

func (au *AuthService) UpdateRefreshToken(ctx context.Context, refreshToken string) (*JwtResponse, error) {
	err := au.JwtProvider.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, err
	}

	uuid, err := au.JwtProvider.GetUUIDFromToken(refreshToken)
	if err != nil {
		return nil, err
	}
	if len(uuid) == 0 {
		return nil, &InvalidTokenError{}
	}

	user, err := au.GetUser(ctx, uuid)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, &IncorrectCredsError{}
	}

	accTkn, err := au.JwtProvider.GenerateAccessToken(user)
	if err != nil {
		return nil, err
	}
	refTkn, err := au.JwtProvider.GenerateRefreshToken(user)
	if err != nil {
		return nil, err
	}

	ans := JwtResponse{
		Type:         "Bearer",
		RefreshToken: refTkn,
		AccessToken:  accTkn,
	}

	return &ans, nil
}

func (au *AuthService) GetUser(ctx context.Context, uuid string) (*User, error) {
	return au.UserSvc.GetUser(ctx, uuid)
}

func NewJwtProvider() (JwtProviderInterface, error) {
	sec, err := getJwtSecret()
	if err != nil {
		return nil, err
	}
	return &JwtProvider{secret: sec}, nil
}

func (jwtp *JwtProvider) GenerateAccessToken(user *User) (string, error) {
	claims := jwt.MapClaims{
		"uuid":       user.UUID,
		"exp":        time.Now().Add(time.Hour * 24).Unix(),
		"token_type": "access",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(jwtp.secret)
}

func (jwtp *JwtProvider) GenerateRefreshToken(user *User) (string, error) {
	claims := jwt.MapClaims{
		"uuid":       user.UUID,
		"exp":        time.Now().Add(24 * 7 * time.Hour).Unix(),
		"token_type": "refresh",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(jwtp.secret)
}

func (jwtp *JwtProvider) ValidateAccessToken(token string) error {
	tkn, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {

		if t.Method.Alg() != "HS256" {
			return nil, &InvalidTokenError{}
		}

		return jwtp.secret, nil
	})

	if err != nil || !tkn.Valid {
		return &InvalidTokenError{}
	}

	tknType, ok := tkn.Claims.(jwt.MapClaims)["token_type"].(string)
	if !ok || tknType != "access" {
		return &InvalidTokenError{}
	}

	return err
}

func (jwtp *JwtProvider) ValidateRefreshToken(token string) error {
	tkn, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {

		if t.Method.Alg() != "HS256" {
			return nil, &InvalidTokenError{}
		}

		return jwtp.secret, nil
	})

	if err != nil || !tkn.Valid {
		return &InvalidTokenError{}
	}

	tknType, ok := tkn.Claims.(jwt.MapClaims)["token_type"].(string)
	if !ok || tknType != "refresh" {
		return &InvalidTokenError{}
	}

	return err
}

func (jwtp *JwtProvider) GetUUIDFromToken(token string) (string, error) {
	var uuid string = ""

	tkn, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != "HS256" {
			return nil, &InvalidTokenError{}
		}
		return jwtp.secret, nil
	})

	if err != nil {
		return "", err
	}

	if claims, ok := tkn.Claims.(jwt.MapClaims); ok && tkn.Valid {
		uuid, ok = claims["uuid"].(string)
		if !ok {
			err = &InvalidTokenError{}
		}
	} else {
		err = &InvalidTokenError{}
	}

	return uuid, err
}
