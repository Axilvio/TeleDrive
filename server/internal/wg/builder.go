package wg

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

type Builder struct {
	Endpoint        string
	ServerPublicKey string
	DNS             string
}

func NewBuilder(endpoint, serverPublicKey, dns string) *Builder {
	if dns == "" {
		dns = "1.1.1.1"
	}
	return &Builder{Endpoint: endpoint, ServerPublicKey: serverPublicKey, DNS: dns}
}

func GeneratePrivateKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate private key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}

func (b *Builder) BuildClientConfig(userID int64, privateKey string) string {
	secondOctet := (userID / 254) % 255
	thirdOctet := (userID % 254) + 1
	address := fmt.Sprintf("10.66.%d.%d/32", secondOctet, thirdOctet)

	return fmt.Sprintf(`[Interface]
# user_id=%d
PrivateKey = %s
Address = %s
DNS = %s

[Peer]
PublicKey = %s
AllowedIPs = 0.0.0.0/0, ::/0
Endpoint = %s
PersistentKeepalive = 25
`, userID, privateKey, address, b.DNS, b.ServerPublicKey, b.Endpoint)
}
