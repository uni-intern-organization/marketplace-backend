package auth

import (
	"github.com/pquerna/otp/totp"
)

func GenerateTOTPSecret(issuer, account string) (secret string, url string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: account,
	})
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

func ValidateTOTP(secret, code string) bool {
	return totp.Validate(code, secret)
}
