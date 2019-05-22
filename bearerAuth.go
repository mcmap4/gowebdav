package gowebdav

import (
	"fmt"
)

// BearerAuth structure holds our JWT Bearer Token
type BearerAuth struct {
	pw string // JWT Token
}

// Type identifies the DigestAuthenticator
func (b *BearerAuth) Type() string {
	return "BearerAuth"
}

// User holds the DigestAuth username
func (b *BearerAuth) User() string {
	return "jwt-token"
}

// Pass holds the DigestAuth password
func (b *BearerAuth) Pass() string {
	return b.pw
}

// Authorize the current request
func (b *BearerAuth) Authorize(c *Client, method string, path string) {
	c.headers.Set("Authorization", fmt.Sprintf("Bearer %s", b.pw))
}
