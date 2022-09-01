package utils

import (
	"log"
	"time"

	"github.com/MicahParks/keyfunc"
)

func JwksCreatePublicKey(jwksURL string, refreshInterval time.Duration) (*keyfunc.JWKS, error) {
	// Create the keyfunc options. Refresh the JWKS every hour and log errors.
	options := keyfunc.Options{
		RefreshInterval: refreshInterval,
		RefreshErrorHandler: func(err error) {
			log.Fatalf("There was an error with the jwt.KeyFunc\nError: %s", err.Error())
		},
	}

	// Create the JWKS from the resource at the given URL.
	jwks, err := keyfunc.Get(jwksURL, options)
	if err != nil {
		return nil, err
	}
	return jwks, nil
}
