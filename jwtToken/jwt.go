// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package jwtToken

import (
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/pkg/errors"
)

const (
	AccessTokenExpireDuration = 7 * 24 * time.Hour
	AdminToken                = 1
	UserToken                 = 2
	hs256MinKeyBytes          = 32
)

type Claims struct {
	Uid          int64  `json:"uid"`
	Source       string `json:"source"`
	Sign         string `json:"sign"`
	BusinessCode string `json:"Business_code"`
	RoleId       int64  `json:"role_id"`
	TokenType    int    `json:"token_type"`
	jwt.StandardClaims
}

func GenerateToken(jwtSecret string, uid int64, source string, sign string, businessCode string, roleId int64, tokenType int) (accessToken string, err error) {
	key, err := jwtSigningKey(jwtSecret)
	if err != nil {
		return "", err
	}

	nowTime := time.Now()
	expireTime := nowTime.Add(AccessTokenExpireDuration)
	claims := Claims{
		Uid:          uid,
		Source:       source,
		Sign:         sign,
		RoleId:       roleId,
		TokenType:    tokenType,
		BusinessCode: businessCode,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expireTime.Unix(),
			Issuer:    "test-user",
		},
	}

	accessToken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
	if err != nil {
		return "", errors.Wrap(err, "failed to get accessToken")
	}

	return accessToken, err
}

func ParseToken(jwtSecret string, token string) (*Claims, error) {
	key, err := jwtSigningKey(jwtSecret)
	if err != nil {
		return nil, err
	}

	claims := &Claims{}
	parser := jwt.Parser{ValidMethods: []string{jwt.SigningMethodHS256.Alg()}}
	parsedToken, err := parser.ParseWithClaims(token, claims, func(_ *jwt.Token) (interface{}, error) {
		return key, nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse token")
	}
	if parsedToken == nil || !parsedToken.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// ParseRefreshToken 校验用于刷新的访问令牌，只允许有效期内的管理员或用户令牌。
func ParseRefreshToken(jwtSecret string, accessToken string) (*Claims, error) {
	claims, err := ParseToken(jwtSecret, accessToken)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to parse access token for refresh")
	}
	if claims.TokenType != AdminToken && claims.TokenType != UserToken {
		return nil, errors.New("unsupported token type for refresh")
	}
	if claims.ExpiresAt <= time.Now().Unix() {
		return nil, errors.New("token has expired or has no expiration")
	}
	return claims, nil
}

// jwtSigningKey 校验 HS256 密钥配置，保留调用方配置的原始字节。
func jwtSigningKey(secret string) ([]byte, error) {
	if len(secret) < hs256MinKeyBytes || strings.TrimSpace(secret) == "" {
		return nil, errors.New("JWT signing key must be at least 32 bytes and not blank")
	}
	return []byte(secret), nil
}
