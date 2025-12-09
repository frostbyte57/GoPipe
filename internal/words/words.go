package words

import (
	"fmt"
	"math/rand"
	"time"
)

// GenerateCode returns a numeric code used for the wormhole connection.
// Format: "id-pin" e.g. "7-123456"
func GenerateCode(id int) string {
	// Create a new source instead of using global rand
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	// Generate 6 digit pin
	pin := r.Intn(900000) + 100000
	return fmt.Sprintf("%d-%d", id, pin)
}
