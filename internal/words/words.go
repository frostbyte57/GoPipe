package words

import (
	"fmt"
	"math/rand"
	"time"
)

func GenerateCode(id int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	pin := r.Intn(900000) + 100000
	return fmt.Sprintf("%d-%d", id, pin)
}
