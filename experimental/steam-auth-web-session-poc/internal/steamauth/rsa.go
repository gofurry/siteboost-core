package steamauth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"math/big"
)

func encryptPassword(password string, key rsaKeyResponse) (string, error) {
	modulus := new(big.Int)
	modulus.SetString(key.Mod, 16)
	exponent := new(big.Int)
	exponent.SetString(key.Exp, 16)

	encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, &rsa.PublicKey{
		N: modulus,
		E: int(exponent.Int64()),
	}, []byte(password))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}
